#!/usr/bin/env python3
"""Rig task service stub v0 (bridge v2 readiness marker).
When run, binds a tailnet-facing port and answers GET /status with a
JSON carrying the will stamp. Readiness only -- no task execution."""
import json, os
from http.server import BaseHTTPRequestHandler, HTTPServer

STAMP = "DAMIEN-WILL-2026-11-10"
BIND = os.environ.get("RIG_TASK_BIND", "NODE_TAILNET_IP")
PORT = int(os.environ.get("RIG_TASK_PORT", "8877"))

class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass
    def do_GET(self):
        if self.path == "/status":
            body = json.dumps({"ok": True, "svc": "rig-task-service",
                               "stub": "v0", "stamp": STAMP}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(404)
            self.end_headers()

if __name__ == "__main__":
    print(f"rig-task-service stub v0 on {BIND}:{PORT} stamp={STAMP}", flush=True)
    HTTPServer((BIND, PORT), H).serve_forever()
