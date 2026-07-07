#!/usr/bin/env python3
import json
import os
import subprocess
import sys


def run(cmd):
    return subprocess.run(cmd, capture_output=True, text=True, shell=False)


def main():
    try:
        _ = json.load(sys.stdin)
    except Exception:
        pass

    status = run(["git", "status", "--porcelain"])
    if status.returncode != 0:
        return 0

    dirty = bool(status.stdout.strip())
    if not dirty:
        return 0

    auto_commit = os.getenv("AUTO_COMMIT_ON_STOP", "").lower() in {"1", "true", "yes", "on"}
    if auto_commit:
        subprocess.run(["git", "add", "-A"], check=False)
        msg = os.getenv("AUTO_COMMIT_MESSAGE", "chore: checkpoint before stop")
        commit = run(["git", "commit", "-m", msg])
        if commit.returncode == 0:
            return 0

    out = {
        "decision": "block",
        "reason": "当前 worktree 有未提交改动：请先提交/还原，再退出会话。",
    }
    sys.stdout.write(json.dumps(out, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())