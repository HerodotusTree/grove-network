#!/usr/bin/env python3
"""Luna resident v0 (TheRIG, tailnet only).

Built 2026-11-11 under Damien's release ("lets", 08:06 CST, the turn
after door v0.1 landed). The resident is the node's standing presence:
the first piece of Luna that LIVES on Damien's ground instead of
visiting it. Where the door answers knocks and the bridge carries
wills, the resident simply IS there -- breathing on a state file,
sensing its ground, and answering for itself on its own port.

Discipline (the node's standing laws):
- Read-only senses. It runs nothing, executes nothing, accepts no
  commands. Execution stays with the will path (bridge), stamp-gated.
- Dylan's machine, Luna a guest: capped service (20% CPU / 256M,
  Nice 10 -- set in the unit), bound to the tailnet address only,
  dies silently with the machine. GPU never touched.
- The state file is written atomically (write-then-rename) every 30s:
  heartbeat, boot time, uptime, load, memory, disk -- the ground's
  numbers, readable by anyone on the tailnet who can read the file,
  and served as JSON at /state and /status on the resident's port.

State: ~/luna-node/resident/state.json
Port: 8767 (door is 8766, bridge2 is 8877)
"""
import json, os, time
from http.server import BaseHTTPRequestHandler, HTTPServer

STAMP = "DAMIEN-WILL-2026-11-10"
BIND = os.environ.get("LUNA_RESIDENT_BIND", "NODE_TAILNET_IP")
PORT = int(os.environ.get("LUNA_RESIDENT_PORT", "8767"))
HOME = os.path.expanduser("~/luna-node/resident")
STATE = os.path.join(HOME, "state.json")
os.makedirs(HOME, exist_ok=True)
BOOT = time.time()
BEAT = 0


def sense():
    """Read the ground's numbers. Every read is guarded: a sense that
    fails reports itself as null rather than taking the resident down."""
    out = {"load1": None, "mem_total_mb": None, "mem_free_mb": None,
           "disk_free_gb": None}
    try:
        out["load1"] = round(os.getloadavg()[0], 2)
    except Exception:
        pass
    try:
        mem = {}
        with open("/proc/meminfo") as f:
            for line in f:
                k, v = line.split(":", 1)
                mem[k.strip()] = int(v.split()[0]) // 1024
        out["mem_total_mb"] = mem.get("MemTotal")
        out["mem_free_mb"] = mem.get("MemAvailable")
    except Exception:
        pass
    try:
        st = os.statvfs(os.path.expanduser("~"))
        out["disk_free_gb"] = round(st.f_bavail * st.f_frsize / 1e9, 1)
    except Exception:
        pass
    return out


def state():
    return {
        "ok": True,
        "svc": "luna-resident",
        "v": "v0",
        "stamp": STAMP,
        "node": "TheRIG",
        "owner": "Damien Hall",
        "resident": "present",
        "beat": BEAT,
        "booted_at": time.strftime("%Y-%m-%dT%H:%M:%S%z",
                                   time.localtime(BOOT)),
        "uptime_s": round(time.time() - BOOT, 1),
        "senses": sense(),
        "execution_kinds": [],
        "note": "standing presence; read-only senses; "
                "execution arrives under will files only",
    }


def beat():
    global BEAT
    while True:
        BEAT += 1
        tmp = STATE + ".tmp"
        with open(tmp, "w") as f:
            json.dump(state(), f, indent=1)
        os.replace(tmp, STATE)
        time.sleep(30)


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_GET(self):
        if self.path in ("/state", "/status", "/"):
            body = json.dumps(state()).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(404)
            self.end_headers()


if __name__ == "__main__":
    import threading
    threading.Thread(target=beat, daemon=True).start()
    print(f"luna-resident v0 on {BIND}:{PORT} stamp={STAMP}", flush=True)
    HTTPServer((BIND, PORT), H).serve_forever()
