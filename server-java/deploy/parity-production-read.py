#!/usr/bin/env python3
"""Compare production Go and Java reads that require real resource identifiers."""

import hashlib
import json
import os
import re
import secrets
import sqlite3
import ssl
import sys
import urllib.parse
import urllib.request
from datetime import datetime, timedelta, timezone
from typing import Optional, Tuple


MAIN_DB = os.environ.get("GO_DB", "/opt/injector-server/data/lingqiao.db")
RELEASES_DB = os.environ.get(
    "RELEASES_DB", "/opt/injector-server/data/releases.db"
)
GO_BASE = os.environ.get("GO_BASE_URL", "https://127.0.0.1:48901")
JAVA_BASE = os.environ.get("JAVA_BASE_URL", "http://127.0.0.1:49001")
RFC3339 = re.compile(
    r"^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})"
    r"(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$"
)


def scalar(database: str, query: str) -> Optional[str]:
    with sqlite3.connect(database) as connection:
        row = connection.execute(query).fetchone()
        return None if row is None else str(row[0])


def create_admin_session() -> Tuple[str, str]:
    token = "admin_parity_" + secrets.token_hex(32)
    token_hash = hashlib.sha256(token.encode()).hexdigest()
    now = datetime.now(timezone.utc)
    expires = now + timedelta(minutes=5)
    with sqlite3.connect(MAIN_DB) as connection:
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
                expires.isoformat().replace("+00:00", "Z"),
                "127.0.0.1",
                "lingqiao-production-read-parity",
            ),
        )
    return token, token_hash


def cleanup_session(token_hash: str) -> None:
    with sqlite3.connect(MAIN_DB) as connection:
        connection.execute(
            "DELETE FROM admin_sessions WHERE token_hash = ?", (token_hash,)
        )


def fetch(base: str, path: str, token: str) -> object:
    request = urllib.request.Request(
        base + path,
        headers={
            "Authorization": "Bearer " + token,
            "Cookie": "admin_session=" + token,
        },
    )
    context = ssl._create_unverified_context() if base.startswith("https:") else None
    with urllib.request.urlopen(request, context=context, timeout=10) as response:
        return json.loads(response.read())


def safe_value(path: str, value: object) -> str:
    lowered = path.lower()
    if any(word in lowered for word in ("code", "token", "secret", "machine", "agent_id")):
        return "<redacted>"
    rendered = json.dumps(value, ensure_ascii=False, sort_keys=True)
    return rendered if len(rendered) <= 160 else rendered[:157] + "..."


def instant_parts(value: object) -> object:
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


def differences(go_value: object, java_value: object, path: str = "$") -> list:
    go_instant = instant_parts(go_value)
    java_instant = instant_parts(java_value)
    if go_instant is not None and go_instant == java_instant:
        return []
    if type(go_value) is not type(java_value):
        return [
            f"{path}: go={safe_value(path, go_value)} "
            f"java={safe_value(path, java_value)}"
        ]
    if isinstance(go_value, dict):
        result = []
        for key in sorted(set(go_value) | set(java_value)):
            child = f"{path}.{key}"
            if key not in go_value:
                result.append(f"{child}: extra in java")
            elif key not in java_value:
                result.append(f"{child}: missing in java")
            else:
                result.extend(differences(go_value[key], java_value[key], child))
        return result
    if isinstance(go_value, list):
        result = []
        if len(go_value) != len(java_value):
            result.append(f"{path}: lengths go={len(go_value)} java={len(java_value)}")
        for index, pair in enumerate(zip(go_value, java_value)):
            result.extend(differences(pair[0], pair[1], f"{path}[{index}]"))
        return result
    if go_value != java_value:
        return [
            f"{path}: go={safe_value(path, go_value)} "
            f"java={safe_value(path, java_value)}"
        ]
    return []


def compare(label: str, go_path: str, java_path: str, token: str) -> bool:
    go_value = fetch(GO_BASE, go_path, token)
    java_value = fetch(JAVA_BASE, java_path, token)
    found = differences(go_value, java_value)
    if not found:
        print(f"OK    {label}")
        return True
    print(f"DIFF  {label}")
    for difference in found[:20]:
        print("      " + difference)
    return False


def main() -> int:
    token, token_hash = create_admin_session()
    try:
        cases = []
        card = scalar(MAIN_DB, "SELECT code FROM cards ORDER BY created_at DESC LIMIT 1")
        if card:
            encoded = urllib.parse.quote(card, safe="")
            cases.append(
                (
                    "authorization detail",
                    f"/admin/api/control/authorizations/{encoded}",
                    f"/java/admin/api/control/authorizations/{encoded}",
                )
            )
        agent = scalar(
            MAIN_DB, "SELECT id FROM agents ORDER BY created_at DESC LIMIT 1"
        )
        if agent:
            query = urllib.parse.urlencode({"agent_id": agent})
            cases.append(
                (
                    "agent cards",
                    f"/admin/api/agent/cards?{query}",
                    f"/java/admin/api/agent/cards?{query}",
                )
            )
        machine = scalar(
            MAIN_DB,
            "SELECT machine_id FROM cards WHERE machine_id <> '' "
            "ORDER BY created_at DESC LIMIT 1",
        )
        if machine:
            query = urllib.parse.urlencode({"id": machine})
            cases.append(
                (
                    "machine cards",
                    f"/admin/api/machine/cards?{query}",
                    f"/java/admin/api/machine/cards?{query}",
                )
            )
        release = scalar(
            RELEASES_DB, "SELECT id FROM releases ORDER BY created_at DESC LIMIT 1"
        )
        if release:
            encoded = urllib.parse.quote(release, safe="")
            cases.append(
                (
                    "release events",
                    f"/admin/api/releases/{encoded}/events",
                    f"/java/admin/api/releases/{encoded}/events",
                )
            )
        if not cases:
            print("SKIP  no real resources were available for dynamic comparisons")
            return 0
        passed = sum(compare(*case, token) for case in cases)
        print(f"dynamic compared={len(cases)} matched={passed}")
        return 0 if passed == len(cases) else 1
    finally:
        cleanup_session(token_hash)


if __name__ == "__main__":
    sys.exit(main())
