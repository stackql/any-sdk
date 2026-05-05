"""Local mock for Google Cloud Asset Inventory `searchAllResources`.

Exists solely to provide end-to-end evidence that an OpenAPI path parameter
whose value contains forward slashes (e.g. `scope = "projects/p1/folders/f2"`)
flows through anysdk's substitution + queryrouter + HTTP layers and lands at
the upstream server with the literal `/` preserved on the wire.

Path shape mirrors the live Google API:
    GET /v1/{scope}:searchAllResources

Run with:
    flask --app=test/python/any_sdk_test_utils/mocks/cloudasset_app:app \
          run --host 127.0.0.1 --port 1198
"""

from threading import Lock

from flask import Flask, jsonify, request


def create_app() -> Flask:
    app = Flask(__name__)

    last_scope: dict = {"scope": None, "query": None, "path": None}
    lock = Lock()

    # `<path:scope>` is the Flask converter that allows '/' inside the captured
    # value — the equivalent of mux's `[^?#]+` regex on the receiving end.
    @app.get("/v1/<path:scope>:searchAllResources")
    def search_all_resources(scope: str):
        with lock:
            last_scope["scope"] = scope
            last_scope["query"] = request.args.get("query")
            last_scope["path"] = request.path
        return jsonify({
            "results": [
                {
                    "name": f"//compute.googleapis.com/{scope}/instances/inst-1",
                    "assetType": "compute.googleapis.com/Instance",
                    "scope_echo": scope,
                },
                {
                    "name": f"//storage.googleapis.com/{scope}/buckets/b-1",
                    "assetType": "storage.googleapis.com/Bucket",
                    "scope_echo": scope,
                },
            ],
        })

    @app.get("/lastrequest")
    def last_request():
        with lock:
            return jsonify(last_scope)

    @app.post("/reset")
    def reset():
        with lock:
            last_scope.update({"scope": None, "query": None, "path": None})
        return jsonify({"ok": True})

    return app


app = create_app()


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=1198)
    parser.add_argument("--host", default="127.0.0.1")
    args = parser.parse_args()
    app.run(host=args.host, port=args.port)
