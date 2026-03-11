#!/usr/bin/env python
from flask import Flask, request, jsonify
import os

import function

app = Flask(__name__)


class Event:
    def __init__(self):
        self.body = request.get_data()
        self.headers = request.headers
        self.method = request.method
        self.query = request.args
        self.path = request.path


class Context:
    def __init__(self):
        self.hostname = os.getenv("HOSTNAME", "localhost")


def format_status_code(res):
    if "statusCode" in res:
        return res["statusCode"]

    return 200


def format_body(res, content_type):
    if content_type == "application/octet-stream":
        return res["body"]

    if "body" not in res:
        return ""
    elif type(res["body"]) == dict:
        return jsonify(res["body"])
    else:
        return str(res["body"])


def format_headers(res):
    if "headers" not in res:
        return []
    elif type(res["headers"]) == dict:
        headers = []
        for key in res["headers"].keys():
            header_tuple = (key, res["headers"][key])
            headers.append(header_tuple)
        return headers

    return res["headers"]


def get_content_type(res):
    content_type = ""
    if "headers" in res:
        content_type = res["headers"].get("Content-type", "")
    return content_type


def format_response(res):
    if res == None:
        return ("", 200)

    if type(res) is dict:
        statusCode = format_status_code(res)
        content_type = get_content_type(res)
        body = format_body(res, content_type)

        headers = format_headers(res)

        return (body, statusCode, headers)

    return res


@app.route("/", defaults={"path": ""}, methods=["GET", "POST"])
@app.route("/<path:path>", methods=["GET", "POST"])
def call_handler(path):
    event = Event()
    context = Context()

    response_data = function.handle(event, context)

    return format_response(response_data)


if __name__ == "__main__":
    from gunicorn.app.base import BaseApplication

    class StandaloneApplication(BaseApplication):
        def __init__(self, app, options=None):
            self.options = options or {}
            self.application = app
            super().__init__()

        def load_config(self):
            for key, value in self.options.items():
                if key in self.cfg.settings and value is not None:
                    self.cfg.set(key.lower(), value)

        def load(self):
            return self.application

    options = {
        "bind": os.getenv("GUNICORN_BIND", "0.0.0.0:5000"),
        "workers": int(os.getenv("GUNICORN_WORKERS", "4")),
        "timeout": int(os.getenv("GUNICORN_TIMEOUT", "30")),
    }
    print(f"Starting gunicorn server with options {options}")
    StandaloneApplication(app, options).run()
