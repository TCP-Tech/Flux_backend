import random
import string
import time
from seleniumbase import BaseCase
import threading
import logging
from selenium.common.exceptions import (
    WebDriverException,
)
import pyautogui

logger = logging.getLogger("nyx_logger")

def _set_cf_cookies_using_cdp(sb: BaseCase, cookies: list[dict[str, str]]):
    logger.debug("enabling driver's network domain")

    # TODO: remove this log
    logger.debug(cookies)

    sb.execute_cdp_cmd("Network.enable", {})
    for cookie in cookies:
        if not cookie.get("name"):
            logger.error(f"cookie {cookie} has no name key")
            continue
        if not cookie.get("value"):
            logger.error(f'cookie {cookie} has no value field')
            continue

        cookie_params = {
            "name": cookie["name"],
            "value": cookie["value"],
            "domain": cookie.get("domain", ".codeforces.com"),
            "path": cookie.get("path", "/"),
            "httpOnly": bool(cookie.get("httpOnly", False)),
            "secure": bool(cookie.get("secure", False)),
        }

        # Only set expiry if present
        if "expiry" in cookie:
            try:
                cookie_params["expiry"] = int(cookie["expiry"])
            except (TypeError, ValueError):
                logger.error(f'cannot cast cookie {cookie["name"]}\'s expiry ({cookie["expiry"]}) to int. Setting the default as 5 minutes from now')
                cookie_params["expiry"] = int(time.time()) + 5 * 60

        # Only set sameSite if valid
        same_site = cookie.get("sameSite")
        if same_site in ("Strict", "Lax", "None"):
            cookie_params["sameSite"] = same_site

        # set the cookies using cdp
        sb.execute_cdp_cmd("Network.setCookie", cookie_params)

    logger.debug('disabling driver\'s network domain')
    sb.execute_cdp_cmd("Network.disable", {})

def generate_random_string(length):
    characters = string.ascii_letters + string.digits
    return ''.join(random.choices(characters, k=length))

def get_random_comment(language: str) -> str:
    random_comment = generate_random_string(16)
    match language:
        case 'java' | 'cpp':
            return '//' + random_comment
        case 'python':
            return '#' + random_comment
        case _:
            raise ValueError('cannot generate comment for unknown language')

def get_cf_code_from_language(language: str) -> str:
    match language:
        case "java":
            return "87"
        case "cpp":
            return "89"
        case "python":
            return "70"
        case _:
            raise ValueError('invalid request to get code for unknown language')
        
def _random_mouse(sb, stop_event: threading.Event):
    """
    Background jitter using pyautogui to keep the session active while
    Selenium interacts with the DOM. stop_event set by caller to stop.
    """
    try:
        rect = sb.driver.execute_script(
            "return {x: window.screenX || window.screenLeft || 0, "
            "y: window.screenY || window.screenTop || 0, "
            "w: window.innerWidth || document.documentElement.clientWidth, "
            "h: window.innerHeight || document.documentElement.clientHeight};"
        )
    except WebDriverException:
        stop_event.set()
        return
    except Exception:
        stop_event.set()
        return

    base_x = int(rect.get("x", 0) + rect.get("w", 0) // 2)
    base_y = int(rect.get("y", 0) + rect.get("h", 0) // 2)

    try:
        while not stop_event.is_set():
            dx = random.randint(-12, 12)
            dy = random.randint(-12, 12)
            tx = base_x + dx
            ty = base_y + dy
            try:
                pyautogui.moveTo(tx, ty, duration=random.uniform(0.05, 0.25))
            except Exception:
                stop_event.set()
                return

            total_sleep = random.uniform(0.1, 0.5)
            slept = 0.0
            while slept < total_sleep:
                if stop_event.is_set():
                    return
                time.sleep(0.05)
                slept += 0.05

            # occasionally re-query viewport
            if random.random() < 0.12:
                try:
                    rect = sb.driver.execute_script(
                        "return {x: window.screenX || window.screenLeft || 0, "
                        "y: window.screenY || window.screenTop || 0, "
                        "w: window.innerWidth || document.documentElement.clientWidth, "
                        "h: window.innerHeight || document.documentElement.clientHeight};"
                    )
                    base_x = int(rect.get("x", 0) + rect.get("w", 0) // 2)
                    base_y = int(rect.get("y", 0) + rect.get("h", 0) // 2)
                except WebDriverException:
                    stop_event.set()
                    return
                except Exception:
                    # continue with previous base
                    pass
    except Exception:
        stop_event.set()
        return

def get_file_extension_for_lang(lang: str) -> str:
    match lang:
        case 'java':
            return '.java'
        case 'cpp':
            return '.cpp'
        case 'python':
            return '.py'
        case _:
            raise ValueError('invalid request for file extension for unknown language')