"""Local fragment of the upstream stackql flask mock pattern.

Mirrors the layout used by stackql/test/python/stackql_test_tooling/flask/<svc>/app.py
but lives in this repo so the retry CLI tests are fully self-contained — no
checkout of the stackql core repo or its test tooling is required.

Run with:
    flask --app=test/python/any_sdk_test_utils/mocks/retry_app:app run --host 127.0.0.1 --port 1199
"""

from collections import defaultdict
from threading import Lock

from flask import Flask, jsonify, request


def create_app() -> Flask:
    app = Flask(__name__)

    counters: "defaultdict[str, int]" = defaultdict(int)
    lock = Lock()

    @app.post("/reset")
    def reset():
        with lock:
            counters.clear()
        return jsonify({"ok": True})

    @app.get("/count/<key>")
    def count(key: str):
        with lock:
            return jsonify({"key": key, "attempts": counters[key]})

    @app.get("/flaky/<key>")
    def flaky(key: str):
        # Returns 503 for the first `fail_until` calls keyed by `key`, then 200.
        # The response body always reports the current attempt number so robot
        # tests can verify retry counts directly from CLI stdout.
        try:
            fail_until = int(request.args.get("fail_until", "0"))
        except ValueError:
            fail_until = 0
        with lock:
            counters[key] += 1
            attempt = counters[key]
        body = {"key": key, "attempt": attempt, "fail_until": fail_until}
        if attempt <= fail_until:
            return jsonify({**body, "ok": False}), 503
        return jsonify({**body, "ok": True})

    @app.get("/always_503")
    def always_503():
        with lock:
            counters["always_503"] += 1
            attempt = counters["always_503"]
        return jsonify({"attempt": attempt, "ok": False}), 503

    return app


app = create_app()


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=1199)
    parser.add_argument("--host", default="0.0.0.0")
    args = parser.parse_args()
    app.run(host=args.host, port=args.port)
