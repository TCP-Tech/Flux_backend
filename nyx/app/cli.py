from seleniumbase import SB
import click
from .api import app
import logging
import threading
from .codeforces import process_cf_requests, random_printer

# setup logger once
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    handlers=[
        logging.StreamHandler()
    ]
)
logger = logging.getLogger("nyx_logger")

driver_lock = threading.Lock()
stop_submit_event = threading.Event()

@click.command()
@click.option('--debug', is_flag=True, help='Enable debug mode')
@click.option('--api-port', 'port', required=True, type=click.INT)
def main(port: int, debug):
    if debug:
        logger.setLevel(logging.DEBUG)

    cf_thread = None
    try:
        logger.info('initilaizing browser Driver')
        with SB(uc=True, page_load_strategy="none", sjw=True, headed=True) as sb:
            # start a cf thread
            logger.info('starting cf thread')
            cf_thread = threading.Thread(target=process_cf_requests, args=(sb, driver_lock, stop_submit_event))
            cf_thread.start()

            logger.info('starting app')
            app.run(port=int(port))
    finally:
        stop_submit_event.set()
        if cf_thread:
            cf_thread.join()
    