import json
import os
import subprocess
import sys


def main():
    runtime_dir = os.environ["FLATKEY_BROWSER_QA_RUNTIME_DIR"]
    cdp_endpoint = os.environ["FLATKEY_BROWSER_QA_CDP_ENDPOINT"]
    for line in sys.stdin:
        try:
            request = json.loads(line)
        except json.JSONDecodeError:
            _write_error(None, -32700, "parse error")
            continue
        if not isinstance(request, dict) or request.get("jsonrpc") != "2.0":
            _write_error(None, -32600, "invalid request")
            continue
        request_id = request.get("id") if isinstance(request, dict) else None
        if request.get("method") == "tools/list":
            _write_result(
                request_id,
                {
                    "tools": [{
                        "name": "qa_capture_screenshot",
                        "inputSchema": {
                            "type": "object",
                            "properties": {"name": {"type": "string"}},
                            "required": ["name"],
                            "additionalProperties": False,
                        },
                    }]
                },
            )
        elif request.get("method") == "tools/call":
            params = request.get("params") if isinstance(request, dict) else None
            arguments = params.get("arguments") if isinstance(params, dict) else None
            name = arguments.get("name") if isinstance(arguments, dict) else None
            if (
                not isinstance(params, dict)
                or params.get("name") != "qa_capture_screenshot"
                or not isinstance(arguments, dict)
                or set(arguments) != {"name"}
                or not isinstance(name, str)
            ):
                _write_error(request_id, -32602, "invalid screenshot request")
                continue
            try:
                logical = _capture_with_node(runtime_dir, cdp_endpoint, name)
            except Exception as exc:
                _write_error(request_id, -32000, str(exc))
                continue
            _write_result(request_id, {"content": [{"type": "text", "text": logical}]})
        else:
            _write_result(request_id, {})


def _capture_with_node(runtime_dir, cdp_endpoint, name):
    script = os.path.join(os.path.dirname(__file__), "browser_evidence_mcp_capture.cjs")
    result = subprocess.run(
        ["node", script, runtime_dir, cdp_endpoint, name],
        text=True,
        capture_output=True,
        timeout=30,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError("screenshot capture failed")
    payload = json.loads(result.stdout)
    return payload["path"]


def _write_result(request_id, result):
    sys.stdout.write(json.dumps({"jsonrpc": "2.0", "id": request_id, "result": result}, sort_keys=True) + "\n")
    sys.stdout.flush()


def _write_error(request_id, code, message):
    sys.stdout.write(json.dumps({"jsonrpc": "2.0", "id": request_id, "error": {"code": code, "message": message}}, sort_keys=True) + "\n")
    sys.stdout.flush()


if __name__ == "__main__":
    main()
