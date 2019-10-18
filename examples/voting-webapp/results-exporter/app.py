import os
import json
import time
from datetime import datetime

import socketio
from flask import Flask, jsonify


RESULTS_HOST = os.getenv("RESULTS_HOST", "http://localhost:8080")
RESULTS = {"total": None, "updated": datetime.utcnow()}
SLEEP_LENGTH = 0.02

sio = socketio.Client()

@sio.event
def message(welcome_msg):
    print(welcome_msg)

@sio.event
def scores(data):
    parsed = json.loads(data)
    RESULTS["total"] = sum(parsed.values())
    RESULTS["updated"] = datetime.utcnow()


app = Flask(__name__)

@app.route("/")
def status():
    if sio.connected:
        sio.disconnect()
    return "ok", 200

@app.route("/metrics")
def metrics():
    previous_update = RESULTS["updated"]
    sio.connect(RESULTS_HOST)
    while RESULTS["updated"] == previous_update:
        time.sleep(SLEEP_LENGTH)
    sio.disconnect()
    return jsonify(RESULTS)


if __name__ == "__main__":
    app.run()
