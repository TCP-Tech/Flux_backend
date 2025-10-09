from seleniumbase import SB
import click
import logging
import threading
from .codeforces import process_cf_requests, CfSubmitRequest
from seleniumbase import BaseCase
import socket
from .exceptions import *
import json
from pydantic import ValidationError, BaseModel, model_validator
from typing import Literal

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

class MainRequest(BaseModel):
    # Use Literal for fields with specific allowed values
    req_type: Literal['submit', 'handshake']
    
    # These fields are only relevant for the 'submit' type
    platform: Literal['codeforces'] = None
    solution: CfSubmitRequest = None

    # Pydantic can validate dependencies between fields
    @model_validator(mode='before')
    def check_submission_fields(cls, values):
        req_type = values.get('req_type')
        if req_type == 'submit':
            if not values.get('platform') or not values.get('solution'):
                raise ValueError('Fields "platform" and "solution" are required for submit requests')
        return values

@click.command()
@click.option('--debug', is_flag=True, help='Enable debug mode')
@click.option('--file', '-f', 'file', required=True, type=click.Path(), help='file where the socket port is written to be picked up by go app')
@click.option('--cf-submit-url', required=True, help='https link at which codeforces submissions are made')
def main(debug: bool, file, cf_submit_url):
    if debug:
        logger.setLevel(logging.DEBUG)
    
    logger.info('initilaizing browser Driver')
    with SB(uc=True, page_load_strategy="none", sjw=True, headed=True) as sb:
        logger.info('starting server')
        sb.maximize_window() # might be problematic when using with xvfb
        serve(sb, file, cf_submit_url)
    
def serve(sb: BaseCase, file: str, cf_submit_url: str):
    server = None
    try:
        server = socket.socket()
        server.bind(('localhost', 0)) # bind to a randomly available port
        server.listen(1) # keep atmost 1 connection in the backlog
        bounded_add = server.getsockname()
        logger.info(f'socket listening at: {bounded_add}')

        # write the listening address into the file
        with open(file, 'w') as f:
            f.write(f'{bounded_add[0]} {bounded_add[1]}')
        
        logger.debug('address written to the file successfully')

        while True:
            response = {}
            client, addr = server.accept()
            logger.debug(f'recieved connection from: {addr}')
            try:
                # 1. Process the request to get a response dictionary
                response = process_requests(client, sb, cf_submit_url)
                if not response:
                    logger.error(f'process_requests function returned invalid response: {response}')
                    response = {'status': 'failed', 'error': 'unknown server error'}
                
                # 2. Serialize and send the response (now inside the try block)
                res_bytes = json.dumps(response).encode('utf-8')
                client.sendall(res_bytes)

                sb.reconnect(2)
            except Exception as e:
                logger.error(f"An error occurred while processing request from {addr}: {e}")
                response = {
                    'status': 'failed',
                    'error': str(e)
                }
            finally:
                client.close()

    except KeyboardInterrupt:
        logger.info("Keyborad interrupt. Server shutting down gracefully.")
    except ServerStartException as se:
        logger.error(f'server failed to start: {se}')
        raise
    except Exception as e:
        logger.error(e)
        raise
    finally:
        if server:
            server.close()

# function is currently supported for cf submissions only
def process_requests(client: socket.socket, sb:BaseCase, cf_submit_url: str) -> dict:
    try:
        req_model = None
        with client.makefile('r', encoding='utf-8') as conn_file:
            # Parse the raw dictionary
            raw_req = json.loads(conn_file.readline())

            # Validate the entire structure using our Pydantic model
            req_model = MainRequest(**raw_req)

    except (json.JSONDecodeError, ValidationError, ValueError) as e:
        # Catch parsing, validation, or logic errors from the Pydantic model
        logger.error(f"Received corrupted or invalid request: {e}")
        return {
            'status': 'failed',
            'error': f'Invalid Request: {e}'
        }
    except Exception as e:
        # Catch unexpected errors during read
        logger.error(f"An unexpected error occurred while reading request: {e}")
        return {'status': 'failed', 'error': str(e)}
        
    # handle handshake
    if req_model.req_type == "handshake":
        return {'status': 'ok'}

    try:
        if req_model.req_type == "submit":
            # The request is already fully validated by Pydantic
            logger.debug('Processing the submission request')
            cookies = process_cf_requests(req_model.solution, sb, cf_submit_url)
            return {'status': 'ok', 'cookies': cookies}
    except BotNotWorkingException as e:
        logger.error(f'Submission failed due to bot {req_model.solution.bot_name} not working')        
        return {
            'status': 'failed',
            'error': 'bot',
        }
    except InvalidSolutionException as e:
        # Catch a specific user-facing error
        logger.error(f"Submission failed due to invalid solution: {e}")
        return {
            'status': 'failed',
            'error': str(e),
            'user_error': True
        }
    except Exception as e:
        # Catch any other exception during submission processing
        logger.error(f"Submission failed: {e}")
        return {'status': 'failed', 'error': f'Submission failed: {e}'}
    
    return {'status': 'failed', 'error': f'Unknown request type: {req_model.req_type}'}
