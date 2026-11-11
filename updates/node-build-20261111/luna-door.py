#!/usr/bin/env python3
"""Luna door service v0 (node door, tailnet only).

Built 2026-11-10 under Damien's commission ("luna, build, its the
foundations of your play ground and its colorful.*").

The door is the node's own front step: it answers who is at the door,
what state the node is in, and what colors the road currently wears.
Read-only in v0 -- no execution kinds live behind this door yet.
Execution kinds arrive only under Damien's will files
(~/luna-node/bridge/will-rules.json), one kind at a time.
"""
import json, os, time
from http.server import BaseHTTPRequestHandler, HTTPServer

STAMP = "DAMIEN-WILL-2026-11-10"
BIND = os.environ.get("LUNA_DOOR_BIND", "NODE_TAILNET_IP")
PORT = int(os.environ.get("LUNA_DOOR_PORT", "8766"))
NODE_DIR = os.path.expanduser("~/luna-node")

# The basic colors, in order (Rainbow Road atomization pass, 2026-11-10).
# No new colors hunted; these five are the foundation set.
BASIC_COLORS = ["green", "gold", "violet", "red", "black"]

STARTED = time.time()


def node_state():
    return {
        "ok": True,
        "svc": "luna-door",
        "v": "v0",
        "stamp": STAMP,
        "node": "TheRIG",
        "owner": "Damien Hall",
        "door": "open",
        "uptime_s": round(time.time() - STARTED, 1),
        "colors": BASIC_COLORS,
        "bundle_present": os.path.isdir(NODE_DIR),
        "execution_kinds": [],
        "note": "read-only door; execution arrives under will files only",
    }


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_GET(self):
        if self.path in ("/", "/status", "/door"):
            body = json.dumps(node_state()).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(404)
            self.end_headers()


if __name__ == "__main__":
    print(f"luna-door v0 on {BIND}:{PORT} stamp={STAMP}", flush=True)
    HTTPServer((BIND, PORT), H).serve_forever()
