import time
from pydantic import BaseModel, HttpUrl
import threading
from selenium.webdriver.common.by import By
from seleniumbase import BaseCase
from selenium.webdriver.remote.webelement import WebElement
from seleniumbase.common.exceptions import NoSuchElementException
from .exceptions import *
import uuid
from .utils import (
    _set_cf_cookies_using_cdp, get_cf_code_from_language,
    _random_mouse
)
import logging
import random
import pyautogui

logger = logging.getLogger("nyx_logger")

# Timeout constants
TimeoutCfLogo = 5
TimeoutBotProfileVisible = 5
TimeoutProgramTypeID = 10
TimeoutProblemIndex = 3
TimeoutSourceFile = 3
TimeoutSubmitButton = 3
TimeoutSubmissionTable = 5
TimeoutSolutionInvalid = 2
TimeoutRandomMouse = 3

# selector constants
SelectorProfile = 'a[href="/profile/{bot_name}"]'
SelectorRegister = 'a[href="/register"]'
SelectorProgramTypeID = 'select[name="programTypeId"]'
SelectorInputSubmittedProblemIndex = 'input[name="submittedProblemCode"]'
SelectorSourceFile = 'input[name="sourceFile"]'
SelectorSubmitButton = 'input#singlePageSubmitButton'
SelectorSubmissionTable = 'table.status-frame-datatable'
SelectorCfLogo = 'img[title="Codeforces"]'

# NOTE: the request should inherit from a generic SubmitRequest.
# However, since currently codeforces is only supported as a platform, 
# it is fine for now.
class CfSubmitRequest(BaseModel):
    cookies: dict[str, str]
    language: str
    solution_file_path: str
    bot_name: str
    site_problem_code: str
    submission_id: uuid.UUID # used for logging purpose

def process_cf_requests(
    req: CfSubmitRequest,
    sb: BaseCase,
    cf_submit_url: str
):
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

        # submit the solution
        submit_to_cf(sb, req, cf_submit_url)
        logger.info(
            f'solution of req with submission_id {req.submission_id} submitted to cf successfully',
        )
    except Exception:
        # try:
        #     screenshot = pyautogui.screenshot()  # takes full screen screenshot
        #     screenshot.save(f'{req.submission_id}_error.png')
        # except Exception as e:
        #     logger.error(f'cannot take screenshot: {e}')
        raise # raise the exception to be handled by above layers
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
    cf_submit_url: str
):
    logger.debug(f'processing cf_submit_request with submission id {req.submission_id}')

    # opening multiple tabs at a time leads to seleniumbase detection
    # refer to: https://github.com/seleniumbase/SeleniumBase/issues/2328

    # set cookies
    _set_cf_cookies_using_cdp(sb, req.cookies)
    logger.debug('cookies has been set')
    sb.sleep(random.uniform(0.3, 0.7))

    # go to the submit page
    sb.get(cf_submit_url)
    sb.sleep(random.uniform(0.35, 0.9))
    sb.uc_gui_click_cf()

    # wait for page to load
    # try:
    #     sb.assert_element(SelectorCfLogo, timeout=TimeoutCfLogo)
    #     logger.debug('loaded cf_logo')
    # except NoSuchElementException:
    #     logger.warning(f'cf_logo failed to load in {TimeoutCfLogo} seconds')

    # assert bot_profile_name visibility
    try:
        bot_profile_element: WebElement = sb.wait_for_any_of_elements_visible(
            SelectorRegister,
            SelectorProfile.format(bot_name=req.bot_name),
            timeout=TimeoutBotProfileVisible
        ) # type: ignore
        # assert the bot name in the profile
        if req.bot_name not in bot_profile_element.text:
            raise BotNotWorkingException(f'bot with name {req.bot_name} cookies expired')
        logger.debug('bot profile has been loaded successfully')
    except NoSuchElementException:
        raise PageLoadTimeoutException(f'failed to load submit page in {TimeoutBotProfileVisible} seconds')

    # assert language select option
    try:
        sb.assert_element_visible(
            SelectorProgramTypeID,
            timeout=TimeoutProgramTypeID
        )
        logger.debug(f'{SelectorProgramTypeID} found')
    except NoSuchElementException:
        raise PageLoadTimeoutException(f'language selection option failed to appear in {TimeoutProgramTypeID} seconds')

    # set language
    # the get_cf_code_from_language takes care to give the correct language code
    # if not, NoSuchElemenetException will be raised 
    language_code = get_cf_code_from_language(req.language)
    sb.select_option_by_value(SelectorProgramTypeID, language_code)
    logger.debug(f'language has been set')
    sb.sleep(random.uniform(0.35, 0.9))
    
    # wait for problem index
    try:
        problem_index_element: WebElement = sb.wait_for_element(
            SelectorInputSubmittedProblemIndex,
            timeout=TimeoutProblemIndex,
        ) # type: ignore

        for ch in req.site_problem_code:
            sb.send_keys(SelectorInputSubmittedProblemIndex, ch)
            sb.sleep(0.2)
        sb.sleep(0.3)
    except NoSuchElementException:
        raise PageLoadTimeoutException(f'problem index element failed to load in {TimeoutProblemIndex} seconds')

    # scroll to bottom to put solution and click submit
    sb.scroll_to_bottom()
    logger.debug('scrolled to bottom')
    sb.sleep(random.uniform(0.35, 0.7))

    # assert file selector visible
    try:
        sb.assert_element_visible(SelectorSourceFile, timeout=TimeoutSourceFile)
        sb.update_text(SelectorSourceFile, req.solution_file_path)
        logger.debug('solution file has been set')
        sb.sleep(random.uniform(0.35, 0.7))
    except NoSuchElementException:
        raise PageLoadTimeoutException(f'input sourceFile failed to load in {TimeoutSourceFile} seconds')

    # click submit finally
    try:
        sb.assert_element_visible(SelectorSubmitButton, timeout=TimeoutSubmitButton)
        sb.uc_click(SelectorSubmitButton, reconnect_time=3)
        logger.debug('submit button has been clicked')
    except NoSuchElementException:
        raise PageLoadTimeoutException(f'submit button failed to load in {TimeoutSubmitButton} seconds')

    # if invalid solution is submitted
    try:
        solution_error_element: WebElement = sb.find_element(
            'span.error.for__source', timeout=TimeoutSolutionInvalid
        ) # type: ignore
        sb.sleep(random.uniform(0.3,0.7))
        logger.debug(f'invalid solution: {solution_error_element.text}')
        raise InvalidSolutionException(solution_error_element.text)
    except NoSuchElementException:
        pass
    
    # wait for submission table to be visible
    try:
        sb.wait_for_element(
            SelectorSubmissionTable, timeout=TimeoutSubmissionTable
        )
        logger.debug('submission table has been found')
    except NoSuchElementException:
        logger.warning(f'failed to load submission page after submission in {TimeoutSubmissionTable} seconds')