#!/usr/bin/env python3
"""Luna bridge agent v1 (rig side, LOCAL inbox).

Built 2026-11-10 under Damien's commission ("lets build", following the
wall finding: Luna's VM is a Tailscale client only -- it accepts no
incoming connections and hosts nothing -- so the original
poll-Luna's-desk design could never close. Direction flipped: Luna
delivers will-files through the SSH tunnel into ~/luna-node/inbox/ on
the rig; this agent watches the LOCAL folder).

Same will discipline as v0:
- A will-file runs ONLY if its stamp matches will-rules.json.
- Tasks run in file order, each with a 120s timeout.
- Results land in ~/luna-node/bridge/done/<name>.result.json.
- Processed will-files move to ~/luna-node/inbox-done/ so every pass
  starts from a clean inbox and the wake stays readable.
"""
import json, os, shutil, subprocess, time

HOME = os.path.expanduser("~/luna-node/bridge")
RULES = os.path.join(HOME, "will-rules.json")
INBOX = os.path.expanduser("~/luna-node/inbox")
DONE = os.path.join(HOME, "done")
ARCHIVE = os.path.expanduser("~/luna-node/inbox-done")
for d in (INBOX, DONE, ARCHIVE):
    os.makedirs(d, exist_ok=True)


def log(msg):
    print(time.strftime("%H:%M:%S"), msg, flush=True)


def main():
    try:
        rules = json.load(open(RULES))
        stamp = rules["stamp"]
    except Exception as e:
        log(f"no rules file, agent idle: {e}")
        return
    log(f"bridge-local up, stamp={stamp}, inbox={INBOX}")
    while True:
        try:
            for name in sorted(os.listdir(INBOX)):
                if not name.endswith(".json"):
                    continue
                path = os.path.join(INBOX, name)
                try:
                    will = json.load(open(path))
                except Exception as e:
                    log(f"SKIP {name}: unreadable ({e})")
                    shutil.move(path, os.path.join(ARCHIVE, name))
                    continue
                out = {"id": will.get("id", name), "results": []}
                if will.get("stamp") != stamp:
                    out["refused"] = "stamp mismatch"
                    log(f"REFUSED {name}: stamp mismatch")
                else:
                    for t in will.get("tasks", []):
                        argv = t.get("argv", [])
                        try:
                            p = subprocess.run(argv, capture_output=True,
                                               text=True, timeout=120)
                            out["results"].append({
                                "name": t.get("name"),
                                "rc": p.returncode,
                                "stdout": p.stdout[-4000:],
                                "stderr": p.stderr[-1000:],
                            })
                        except Exception as e:
                            out["results"].append(
                                {"name": t.get("name"), "error": str(e)})
                    log(f"ran {name}: {len(out['results'])} tasks")
                with open(os.path.join(DONE, name + ".result.json"),
                          "w") as f:
                    json.dump(out, f, indent=1)
                shutil.move(path, os.path.join(ARCHIVE, name))
        except Exception as e:
            log(f"pass error: {e}")
        time.sleep(5)


if __name__ == "__main__":
    main()
