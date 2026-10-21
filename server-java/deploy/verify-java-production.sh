#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

systemctl is-active --quiet lingqiao-java.service ||
  fail "Java service is not active"
! systemctl is-active --quiet injector-server.service ||
  fail "Go service is still active"
[[ "$(systemctl is-enabled injector-server.service 2>/dev/null || true)" == disabled ]] ||
  fail "Go service is not disabled"
nginx -t >/dev/null

curl -fsS http://127.0.0.1:49001/java/api/health \
  >/tmp/lingqiao-java-verify-health.json
curl -kfsS https://127.0.0.1:48901/api/health \
  >/tmp/lingqiao-legacy-verify-health.json
curl -kfsS https://127.0.0.1:48901/admin/ \
  >/tmp/lingqiao-admin-verify.html
curl -kfsS https://127.0.0.1:38472/ \
  >/tmp/lingqiao-agent-verify.html
curl -kfsS https://127.0.0.1:48901/api/v1/announcement \
  >/tmp/lingqiao-announcement-verify.json

python3 - <<'PY'
import json
for path in (
    "/tmp/lingqiao-java-verify-health.json",
    "/tmp/lingqiao-legacy-verify-health.json",
):
    health = json.load(open(path))
    if health.get("status") != "ok":
        raise SystemExit("{} is not healthy".format(path))
    if health.get("production_entrypoint_replaced") is not True:
        raise SystemExit("{} is not in production mode".format(path))
    bad = [key for key, value in health.items()
           if key.endswith("_enabled") and value is not True]
    if bad:
        raise SystemExit("{} has disabled flags: {}".format(path, ", ".join(sorted(bad))))
PY

grep -qi '<html' /tmp/lingqiao-admin-verify.html ||
  fail "admin panel is not HTML"
grep -qi '<html' /tmp/lingqiao-agent-verify.html ||
  fail "agent panel is not HTML"
sqlite3 /opt/injector-server/data/lingqiao.db 'PRAGMA integrity_check;' |
  grep -qx ok || fail "lingqiao.db integrity check failed"
sqlite3 /opt/injector-server/data/releases.db 'PRAGMA integrity_check;' |
  grep -qx ok || fail "releases.db integrity check failed"

ss -lntp | grep -E ':48901\b' | grep -q nginx ||
  fail "nginx does not own legacy port 48901"
ss -lntp | grep -E ':38472\b' | grep -q nginx ||
  fail "nginx does not own legacy agent port 38472"
ss -lntp | grep -E ':49001\b' | grep -q java ||
  fail "Java does not own its loopback port"
! pgrep -x injector-server >/dev/null ||
  fail "a Go injector-server process still exists"

echo "Java production verification passed."
