#!/usr/bin/env python3
"""Compare authenticated agent-portal reads without retaining a test session."""

import hashlib
import json
import os
import re
import secrets
import sqlite3
import ssl
import subprocess
import sys
import time
import urllib.request
from datetime import datetime, timedelta, timezone


DATA_DIR = "/opt/injector-server/data"
DATABASE = os.path.join(DATA_DIR, "lingqiao.db")
SESSIONS = os.path.join(DATA_DIR, "agent_sessions.json")
TOKEN_MAP = os.path.join(DATA_DIR, "agent_token_map.json")
GO_BASE = "https://127.0.0.1:38472"
JAVA_BASE = "http://127.0.0.1:49001/java/agent"
PRODUCTION_GATEWAY = os.environ.get("JAVA_PRODUCTION_GATEWAY") == "true"
RFC3339 = re.compile(
    r"^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})"
    r"(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$"
)


def run(*command):
    subprocess.check_call(command)


def wait_ready():
    for _ in range(45):
        try:
            request = urllib.request.Request(
                "http://127.0.0.1:49001/java/api/health"
            )
            with urllib.request.urlopen(request, timeout=2):
                return
        except Exception:
            time.sleep(1)
    raise RuntimeError("Java did not become healthy")


def restart_services():
    if not PRODUCTION_GATEWAY:
        run("systemctl", "restart", "injector-server.service")
    run("systemctl", "restart", "lingqiao-java.service")
    wait_ready()


def write_json(path, value):
    temporary = path + ".parity-agent.tmp"
    with open(temporary, "w", encoding="utf-8") as target:
        json.dump(value, target, separators=(",", ":"))
    os.chmod(temporary, 0o600)
    os.rename(temporary, path)


def fetch(base, path, token):
    request = urllib.request.Request(
        base + path, headers={"Authorization": "Bearer " + token}
    )
    context = ssl._create_unverified_context() if base.startswith("https:") else None
    with urllib.request.urlopen(request, context=context, timeout=10) as response:
        return json.loads(response.read())


def instant_parts(value):
    if not isinstance(value, str):
        return None
    match = RFC3339.match(value)
    if not match:
        return None
    base = datetime.strptime(match.group(1), "%Y-%m-%dT%H:%M:%S")
    fraction = (match.group(2) or "").ljust(9, "0")
    offset = match.group(3)
    if offset == "Z":
        offset_minutes = 0
    else:
        sign = 1 if offset[0] == "+" else -1
        offset_minutes = sign * (int(offset[1:3]) * 60 + int(offset[4:6]))
    utc_base = base - timedelta(minutes=offset_minutes)
    return (utc_base.strftime("%Y-%m-%dT%H:%M:%S"), fraction)


def equal(left, right):
    left_instant = instant_parts(left)
    right_instant = instant_parts(right)
    if left_instant is not None and left_instant == right_instant:
        return True
    if type(left) is not type(right):
        return False
    if isinstance(left, dict):
        return set(left) == set(right) and all(
            equal(left[key], right[key]) for key in left
        )
    if isinstance(left, list):
        return len(left) == len(right) and all(
            equal(a, b) for a, b in zip(left, right)
        )
    return left == right


def main():
    with sqlite3.connect(DATABASE) as connection:
        row = connection.execute(
            "SELECT id FROM agents WHERE disabled = 0 ORDER BY created_at LIMIT 1"
        ).fetchone()
    if row is None:
        print("SKIP  no enabled agent is available")
        return 0
    agent_id = row[0]
    originals = {}
    for path in (SESSIONS, TOKEN_MAP):
        with open(path, "rb") as source:
            originals[path] = source.read()

    token = "agent_parity_" + secrets.token_hex(32)
    token_hash = hashlib.sha256(token.encode()).hexdigest()
    try:
        with open(SESSIONS, encoding="utf-8") as source:
            sessions = json.load(source)
        with open(TOKEN_MAP, encoding="utf-8") as source:
            token_map = json.load(source)
        expires = datetime.now(timezone.utc) + timedelta(minutes=5)
        sessions[token_hash] = expires.isoformat().replace("+00:00", "Z")
        token_map[token_hash] = agent_id
        write_json(SESSIONS, sessions)
        write_json(TOKEN_MAP, token_map)
        restart_services()

        failed = False
        for endpoint in ("/api/dashboard", "/api/cards", "/api/info"):
            go_value = fetch(GO_BASE, endpoint, token)
            java_value = fetch(JAVA_BASE, endpoint, token)
            if equal(go_value, java_value):
                print("OK    agent portal " + endpoint)
            else:
                print("DIFF  agent portal " + endpoint)
                failed = True
        return 1 if failed else 0
    finally:
        for path, content in originals.items():
            temporary = path + ".parity-agent.restore"
            with open(temporary, "wb") as target:
                target.write(content)
            os.chmod(temporary, 0o600)
            os.rename(temporary, path)
        restart_services()


if __name__ == "__main__":
    sys.exit(main())
