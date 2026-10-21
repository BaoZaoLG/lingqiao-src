"""
Verify Java is the production authority for admin/client APIs.

Usage:
  python scripts/check_java_production_entrypoint.py [--host https://127.0.0.1:48901]

Checks:
  1) /api/health reports cards_write_enabled and production_entrypoint_replaced
  2) /admin/api/card/generate and /admin/api/cards/generate both reach Java
     (401 unauthorized is OK; 500/404 is fail)
  3) /java/api/cutover on localhost:49001 when --local-java is passed

Exit 0 on pass.
"""
from __future__ import annotations

import argparse
import json
import ssl
import sys
import urllib.error
import urllib.request

CTX = ssl._create_unverified_context()


def fetch(url: str, method: str = "GET", data: bytes | None = None):
    req = urllib.request.Request(
        url, data=data, method=method,
        headers={"Content-Type": "application/json"} if data else {},
    )
    try:
        with urllib.request.urlopen(req, timeout=10, context=CTX) as r:
            body = r.read()
            return r.status, body
    except urllib.error.HTTPError as e:
        return e.code, e.read()
    except Exception as e:  # noqa: BLE001
        return 0, str(e).encode()


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--host", default="https://127.0.0.1:48901")
    ap.add_argument("--local-java", action="store_true", help="also check 127.0.0.1:49001 cutover")
    args = ap.parse_args()
    host = args.host.rstrip("/")
    errors: list[str] = []

    status, body = fetch(f"{host}/api/health")
    if status != 200:
        errors.append(f"health HTTP {status}: {body[:200]!r}")
    else:
        try:
            health = json.loads(body.decode())
        except json.JSONDecodeError as exc:
            errors.append(f"health not json: {exc}")
        else:
            if not health.get("cards_write_enabled"):
                errors.append("health.cards_write_enabled is false")
            if not health.get("production_entrypoint_replaced"):
                errors.append("health.production_entrypoint_replaced is false")
            if not health.get("cards_enabled"):
                errors.append("health.cards_enabled is false")

    empty = b'{"duration_hours":1,"max_sessions":1,"note":"entrypoint-check"}'
    for path in (
        "/admin/api/card/generate",
        "/admin/api/cards/generate",
    ):
        status, body = fetch(f"{host}{path}", method="POST", data=empty)
        # Java should reject missing auth with 401, not 404/500.
        if status == 0:
            errors.append(f"{path}: connection failed {body[:120]!r}")
        elif status >= 500:
            errors.append(f"{path}: HTTP {status} {body[:160]!r}")
        elif status == 404:
            errors.append(f"{path}: HTTP 404 (not mapped to Java)")
        elif status not in (400, 401, 403):
            # Unexpected success without auth would also be a problem.
            errors.append(f"{path}: unexpected HTTP {status} {body[:160]!r}")

    if args.local_java:
        status, body = fetch("http://127.0.0.1:49001/java/api/cutover")
        if status != 200:
            errors.append(f"java cutover HTTP {status}")
        else:
            try:
                c = json.loads(body.decode())
                if c.get("production_entrypoint_replaced") is not True:
                    errors.append("cutover.production_entrypoint_replaced != true")
                if c.get("enabled") is not False:
                    errors.append("cutover.enabled should be false when Java is authority")
            except json.JSONDecodeError as exc:
                errors.append(f"cutover not json: {exc}")

    # Go shadow must not expose cutover when Java is the production entrypoint.
    status, _body = fetch(f"{host}/api/cutover")
    if status == 200:
        errors.append("Go /api/cutover is exposed — Java should be sole authority")

    if errors:
        print("JAVA ENTRYPOINT FAIL")
        for e in errors:
            print(" ", e)
        return 1
    print(f"JAVA ENTRYPOINT OK host={host}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
