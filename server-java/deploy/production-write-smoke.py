#!/usr/bin/env python3
"""Exercise representative Java writes against a disposable production snapshot."""

import hashlib
import json
import os
import secrets
import sqlite3
import sys
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone


BASE = os.environ.get("JAVA_SMOKE_BASE_URL", "http://127.0.0.1:49002")
SMOKE_ROOT = os.environ.get("JAVA_SMOKE_ROOT", "")
if not SMOKE_ROOT.startswith("/opt/lingqiao-java/write-smoke."):
    raise SystemExit("JAVA_SMOKE_ROOT must be a write-smoke directory")
DATABASE = os.path.join(SMOKE_ROOT, "data", "lingqiao.db")


def create_admin_session():
    token = "admin_smoke_" + secrets.token_hex(32)
    token_hash = hashlib.sha256(token.encode()).hexdigest()
    now = datetime.now(timezone.utc)
    with sqlite3.connect(DATABASE) as connection:
        row = connection.execute(
            "SELECT id FROM admin_users WHERE disabled = 0 "
            "ORDER BY CASE role WHEN 'owner' THEN 0 ELSE 1 END, created_at LIMIT 1"
        ).fetchone()
        if row is None:
            raise RuntimeError("no enabled administrator is available")
        connection.execute(
            "INSERT INTO admin_sessions "
            "(token_hash, admin_id, created_at, expires_at, remote_addr, user_agent) "
            "VALUES (?, ?, ?, ?, ?, ?)",
            (
                token_hash,
                row[0],
                now.isoformat().replace("+00:00", "Z"),
                (now + timedelta(minutes=10)).isoformat().replace("+00:00", "Z"),
                "127.0.0.1",
                "lingqiao-java-production-write-smoke",
            ),
        )
    return token, token_hash


def post(path, token, body):
    request = urllib.request.Request(
        BASE + path,
        data=json.dumps(body).encode(),
        headers={
            "Authorization": "Bearer " + token,
            "Content-Type": "application/json",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=15) as response:
            value = json.loads(response.read())
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", "replace")
        raise RuntimeError("{} returned {}: {}".format(path, error.code, detail[:300]))
    if value.get("status") != "ok":
        raise RuntimeError("{} did not return status=ok".format(path))
    return value


def main():
    token, token_hash = create_admin_session()
    suffix = secrets.token_hex(6)
    try:
        created_card = post(
            "/java/admin/api/cards/generate",
            token,
            {"duration_hours": 1, "max_sessions": 1, "note": "java-cutover-smoke"},
        )
        card_code = created_card["card"]["code"]
        post(
            "/java/admin/api/cards/update",
            token,
            {"code": card_code, "action": "delete", "extend_hours": 0},
        )
        print("OK    cards create/delete")

        marker = "java-cutover-smoke-" + suffix
        post(
            "/java/admin/api/blacklist",
            token,
            {"action": "add", "type": "ip", "value": marker, "reason": "smoke"},
        )
        post(
            "/java/admin/api/blacklist",
            token,
            {"action": "remove", "type": "ip", "value": marker, "reason": ""},
        )
        print("OK    blacklist add/remove")

        created_invite = post(
            "/java/admin/api/invite/create",
            token,
            {"count": 1, "max_uses": 1},
        )
        invite_code = created_invite["invites"][0]["code"]
        post(
            "/java/admin/api/invite/delete", token, {"code": invite_code}
        )
        print("OK    invite create/delete")

        post(
            "/java/admin/api/announcement",
            token,
            {"content": "Java cutover write smoke " + suffix},
        )
        print("OK    announcement revision")

        post(
            "/java/admin/api/releases",
            token,
            {
                "version": "0.0.0-smoke-" + suffix,
                "channel": "stable",
                "min_version": "",
                "force_update": False,
                "rollout_percent": 0,
                "notes": "java-cutover-smoke",
                "targeting": {},
            },
        )
        print("OK    release draft create")
        print("production-snapshot write smoke passed")
        return 0
    finally:
        with sqlite3.connect(DATABASE) as connection:
            connection.execute(
                "DELETE FROM admin_sessions WHERE token_hash = ?", (token_hash,)
            )


if __name__ == "__main__":
    sys.exit(main())
