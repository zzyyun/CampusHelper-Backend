#!/usr/bin/env python3
import datetime
import json
import sys
from pathlib import Path


def main():
    try:
        data = json.load(sys.stdin)
    except Exception:
        return 0

    log_path = Path(".claude/hooks/notifications.log")
    log_path.parent.mkdir(parents=True, exist_ok=True)

    entry = {
        "ts": datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z"),
        **data,
    }
    with log_path.open("a", encoding="utf-8") as f:
        f.write(json.dumps(entry, ensure_ascii=False) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())