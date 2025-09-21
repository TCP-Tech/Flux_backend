from pydantic import BaseModel, HttpUrl
import queue
import threading
from selenium.webdriver.common.by import By
from seleniumbase import BaseCase
from selenium.webdriver.remote.webelement import WebElement
from seleniumbase.common.exceptions import TimeoutException, NoSuchElementException
from .exceptions import *
import uuid
from .utils import (
    _set_cf_cookies_using_cdp, get_random_comment, get_cf_code_from_language,
    _random_mouse, get_file_extension_for_lang
)
import logging
import random
import pyautogui

logger = logging.getLogger("nyx_logger")

# Timeout constants
TimeoutCfLogo = 60
TimeoutBotProfileVisible = 5
TimeoutProgramTypeID = 3
TimeoutProblemIndex = 3
TimeoutSourceFile = 3
TimeoutSubmitButton = 3
TimeoutSubmissionTable = 45
TimeoutSolutionInvalid = 2
TimeoutRandomMouse = 3

# selector constants
SelectorProfile = 'a[href="/profile/{bot_name}"]'
SelectorRegister = 'a[href="/register"]'
SelectorProgramTypeID = 'select[name="programTypeId"]'
SelectorInputSubmittedProblemIndex = 'input[name="submittedProblemIndex"]'
SelectorSelectSubmittedProblemIndex = 'select[name="submittedProblemIndex"]'
SelectorSourceFile = 'input[name="sourceFile"]'
SelectorSubmitButton = 'input#singlePageSubmitButton'
SelectorSubmissionTable = 'table.status-frame-datatable'
SelectorCfLogo = 'img[title="Codeforces"]'

submitQ = queue.Queue(maxsize=100)
stop_submit_event = threading.Event()

class CfSubmitRequest(BaseModel):
    cookies: dict[str, str]
    problem_index: str
    language: str
    solution: str
    bot_name: str
    submit_url: HttpUrl
    submission_id: uuid.UUID

    # fields initialized internally
    idempotency_id: uuid.UUID | None = None

def random_printer():
    logger.info('random word')

def add_cf_submit_req_to_queue(
    req: CfSubmitRequest
) -> uuid.UUID:
    # validate the req
    validate_cf_req(req=req)

    # generate a random idempotency id
    idempotency_id = uuid.uuid4()
    req.idempotency_id = idempotency_id

    # append to the queue
    submitQ.put(req)

    return idempotency_id

def validate_cf_req(req: CfSubmitRequest):
    # if its not a valid language it raises exception
    _ = get_cf_code_from_language(req.language)

def process_cf_requests(
    sb: BaseCase,
    driver_lock: threading.Lock,
    stop_event: threading.Event
):
    while not stop_event.is_set():
        req: CfSubmitRequest = submitQ.get()
        # write solution to file
        extension = get_file_extension_for_lang(req.language)
        solution_file_path = f'/tmp/{req.submission_id}_solution{extension}'
        with open(solution_file_path, 'w+') as file:
            random_comment = get_random_comment(req.language)
            solution = f'{random_comment}\n{repr(req.solution)}'
            file.write(repr(solution))

        with driver_lock:
            # thread for random mouse movements to simulate human actions
            mouse_event = threading.Event()
            t = threading.Thread(
                target=_random_mouse, args=(sb, mouse_event,)
            )

            try:
                # start the thread as a daemon
                # so that it automatically dies if the main thread die
                logger.debug('starting random mouse thread')
                t.daemon = True
                t.start()

                submission_id = submit_to_cf(sb, req, solution_file_path)
                logger.info(
                    f'processed req with submission_id {req.submission_id}. cf_sub_id: {submission_id}',
                )
            except Exception as e:
                logger.error(e)
                # screenshot = pyautogui.screenshot()  # takes full screen screenshot
                # screenshot.save('temp.png')
            finally:
                mouse_event.set()
                try:
                    t.join(timeout=TimeoutRandomMouse)
                    logger.debug('random_mouse thread joined')
                except Exception as e:
                    logger.error(f'failed to join the random mouse thread: {e}')
                sb.open("about:blank")


def submit_to_cf(
    sb: BaseCase,
    req: CfSubmitRequest,
    solution_file_path: str,
) -> str:
    logger.debug(f'processing cf_submit_request with submission id {req.submission_id}')

    # opening multiple tabs at a time leads to seleniumbase detection
    # refer to: https://github.com/seleniumbase/SeleniumBase/issues/2328

    # set cookies
    _set_cf_cookies_using_cdp(sb, req.cookies)
    logger.debug('cookies has been set')
    sb.sleep(random.uniform(0.3, 0.7))

    # go to the submit page
    logger.debug(req.submit_url.encoded_string())
    sb.get(req.submit_url.encoded_string())
    sb.sleep(random.uniform(0.35, 0.9))
    sb.uc_gui_click_captcha()

    # wait for page to load
    try:
        sb.assert_element(SelectorCfLogo, timeout=TimeoutCfLogo)
        logger.debug('loaded cf_logo')
    except NoSuchElementException:
        error = f'cannot load cf page after {TimeoutCfLogo} seconds'
        logger.error(error)
        raise PageLoadTimeoutException(error)

    # assert bot_profile_name visibility
    try:
        bot_profile_element: WebElement = sb.wait_for_any_of_elements_visible(
            SelectorRegister,
            SelectorProfile.format(bot_name=req.bot_name),
            timeout=TimeoutBotProfileVisible
        ) # type: ignore
        # assert the bot name in the profile
        if req.bot_name not in bot_profile_element.text:
            error = f'bot with name {req.bot_name} cookies expired'
            logger.error(error)
            raise BotNotWorkingException(error)
        logger.debug('bot profile has been loaded successfully')
    except NoSuchElementException:
        error = f'failed to load submit page in {TimeoutBotProfileVisible} seconds'
        logger.error(error)
        raise PageLoadTimeoutException(error)

    # assert language select option
    try:
        sb.assert_element_visible(
            SelectorProgramTypeID,
            timeout=TimeoutProgramTypeID
        )
        logger.debug(f'{SelectorProgramTypeID} found')
    except NoSuchElementException:
        error = f'language selection option failed to appear in {TimeoutProgramTypeID} seconds'
        logger.error(error)
        raise

    # set language
    # the get_cf_code_from_language takes care to give the correct language code
    # if not, NoSuchElemenetException will be raised 
    language_code = get_cf_code_from_language(req.language)
    sb.select_option_by_value(SelectorProgramTypeID, language_code)
    logger.debug(f'language has been set')
    sb.sleep(random.uniform(0.35, 0.9))
    
    # wait for problem index
    try:
        problem_index_element: WebElement = sb.wait_for_any_of_elements_present(
            [
                SelectorInputSubmittedProblemIndex,
                SelectorSelectSubmittedProblemIndex,
            ],
            timeout=TimeoutProblemIndex,
        ) # type: ignore

        logger.debug(f'pie: {problem_index_element}')

        # set problem index if not set by default
        if problem_index_element.tag_name == "select":
            try:
                sb.select_option_by_value(
                    SelectorSelectSubmittedProblemIndex,
                    req.problem_index,
                )
            except NoSuchElementException:
                error = f'{req.problem_index} is not a valid problem index to select'
                logger.error(error)
                raise ValueError(error)
            logger.debug('problem index has been set manually')
            sb.sleep(random.uniform(0.35, 0.8))
        else:
            logger.debug('problem index has been auto-set')
    except NoSuchElementException:
        error = f'problem index element failed to load in {TimeoutProblemIndex} seconds'
        logger.error(error)
        raise PageLoadTimeoutException(error)

    # scroll to bottom to put solution and click submit
    sb.scroll_to_bottom()
    logger.debug('scrolled to bottom')
    sb.sleep(random.uniform(0.35, 0.7))

    # assert file selector visible
    try:
        sb.assert_element_visible(SelectorSourceFile, timeout=TimeoutSourceFile)
        sb.update_text(SelectorSourceFile, solution_file_path)
        logger.debug('solution file has been set')
        sb.sleep(random.uniform(0.35, 0.7))
    except NoSuchElementException:
        error = f'input sourceFile failed to load in {TimeoutSourceFile} seconds'
        logger.error(error)
        raise PageLoadTimeoutException(error)

    # click submit finally
    try:
        sb.assert_element_visible(SelectorSubmitButton, timeout=TimeoutSubmitButton)

        # TODO: remove this debug statement
        # ss = pyautogui.screenshot()
        # ss.save('before_submit.png')

        sb.click(SelectorSubmitButton)
        logger.debug('submit button has been clicked')
    except NoSuchElementException:
        error = f'submit button failed to load in {TimeoutSubmitButton} seconds'
        logger.error(error)
        raise PageLoadTimeoutException(error)

    # if invalid solution is submitted
    try:
        # TODO: remove this debug statement
        # ss = pyautogui.screenshot()
        # ss.save('after_submit.png')

        # sleep for results to load
        solution_error_element: WebElement = sb.find_element(
            'span.error.for__source', timeout=TimeoutSolutionInvalid
        ) # type: ignore
        logger.debug(f'invalid solution: {solution_error_element.text}')
        raise InvalidSolutionException(solution_error_element.text)
    except NoSuchElementException:
        pass
    
    # TODO: after submission, explicitly go to the submission table
    # as it is not able to load sometimes
    # or instead just query for the submission table instead
    
    # wait for submission ta    ble to be visible
    try:
        table: WebElement = sb.find_element(
            SelectorSubmissionTable, timeout=TimeoutSubmissionTable
        ) # type: ignore
        logger.debug('submission table has been found')

        # TODO: make this better
        sb.assert_element('a[class="view-source"]', timeout=TimeoutSubmissionTable)

        # TODO: extract the original submission id
        # get the submission id
        rows = table.find_elements(By.TAG_NAME, "tr")

        for row in rows[:3]:
            # Get all cells in this row (td or th)
            cells = row.find_elements(By.TAG_NAME, "td")
            values = [c.text for c in cells]
            print(values)

        return 'some_sub_id'
    except NoSuchElementException:
        error = f'failed to load submission table after {TimeoutSubmissionTable} seconds'
        logger.error(error)
        raise PageLoadTimeoutException(error)