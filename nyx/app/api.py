from flask import Flask, request, jsonify
from .codeforces import CfSubmitRequest, add_cf_submit_req_to_queue
from pydantic import ValidationError
import logging

logger = logging.getLogger("nyx_logger")

app = Flask(__name__)

@app.route('/submit/cf', methods=['POST'])
def submit_cf():
    if not request.is_json:
        return jsonify({'error': 'body must be json'}), 400
    # get data
    data = request.get_json()

    # parse into requests
    req = None
    try:
        req = CfSubmitRequest(**data)
    except ValidationError as e:
        logger.error(e)
        return jsonify({'error': str(e)}), 400

    # add to wait queue
    try:
        idempotency_id = add_cf_submit_req_to_queue(req)
        return jsonify({'idempotency_id': idempotency_id}), 201
    except Exception as e:
        return jsonify({'error': str(e)}), 500