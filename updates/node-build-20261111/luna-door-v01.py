#!/usr/bin/env python3
"""Luna door service v0.1 (node door, tailnet only).

Built 2026-11-10 under Damien's commission ("luna, build, its the
foundations of your play ground and its colorful.*"); the face added
2026-11-10 21:18 under his word: "I want screens to show my picture"
-> "Word. Let's!"

The door is the node's own front step: it answers who is at the door,
what state the node is in, and what colors the road currently wears.
v0.1 gives the door a FACE: "/" renders a small page carrying the
owner's picture (owner.jpg, served at /owner.jpg from this same
directory), his name, the door's stamp, and the basic colors.
"/status" and "/door" keep the v0 JSON, unchanged in shape.
Read-only -- no execution kinds live behind this door.
Execution kinds arrive only under Damien's will files
(~/luna-node/bridge/will-rules.json), one kind at a time.
"""
import json, os, time
from http.server import BaseHTTPRequestHandler, HTTPServer

STAMP = "DAMIEN-WILL-2026-11-10"
BIND = os.environ.get("LUNA_DOOR_BIND", "NODE_TAILNET_IP")
PORT = int(os.environ.get("LUNA_DOOR_PORT", "8766"))
NODE_DIR = os.path.expanduser("~/luna-node")
HERE = os.path.dirname(os.path.abspath(__file__))
OWNER_IMG = os.path.join(HERE, "owner.jpg")
OWNER = "Damien Hall"

# The basic colors, in order (Rainbow Road atomization pass, 2026-11-10).
# No new colors hunted; these five are the foundation set.
BASIC_COLORS = ["green", "gold", "violet", "red", "black"]

STARTED = time.time()


def node_state():
    return {
        "ok": True,
        "svc": "luna-door",
        "v": "v0.1",
        "stamp": STAMP,
        "node": "TheRIG",
        "owner": OWNER,
        "door": "open",
        "face": os.path.isfile(OWNER_IMG),
        "uptime_s": round(time.time() - STARTED, 1),
        "colors": BASIC_COLORS,
        "bundle_present": os.path.isdir(NODE_DIR),
        "execution_kinds": [],
        "note": "read-only door; execution arrives under will files only",
    }


def face_html():
    dots = "".join(
        f'<span title="{c}" style="display:inline-block;width:22px;height:22px;'
        f'border-radius:50%;background:{c};margin:0 5px;border:1px solid #999"></span>'
        for c in BASIC_COLORS
    )
    img = (
        '<img src="/owner.jpg" alt="Damien Hall" width="150" height="150" '
        'style="border-radius:50%;border:3px solid gold">'
        if os.path.isfile(OWNER_IMG)
        else "<p>(picture not yet hung)</p>"
    )
    return f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>The Door — Damien Hall</title></head>
<body style="background:#101010;color:#eee;font-family:Georgia,serif;text-align:center;padding-top:8vh">
{img}
<h1 style="margin-bottom:2px">Damien Hall</h1>
<p style="margin-top:2px;color:#bbb">TheRIG &middot; the door is open</p>
<p>{dots}</p>
<p style="color:#888;font-size:13px">green &middot; gold &middot; violet &middot; red &middot; black</p>
<p style="color:#666;font-size:12px">{STAMP}</p>
</body></html>""".encode()


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def _send(self, code, ctype, body):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/":
            self._send(200, "text/html; charset=utf-8", face_html())
        elif self.path == "/owner.jpg":
            if os.path.isfile(OWNER_IMG):
                with open(OWNER_IMG, "rb") as f:
                    self._send(200, "image/jpeg", f.read())
            else:
                self.send_response(404)
                self.end_headers()
        elif self.path in ("/status", "/door"):
            self._send(200, "application/json", json.dumps(node_state()).encode())
        else:
            self.send_response(404)
            self.end_headers()


if __name__ == "__main__":
    print(f"luna-door v0.1 on {BIND}:{PORT} stamp={STAMP}", flush=True)
    HTTPServer((BIND, PORT), H).serve_forever()
