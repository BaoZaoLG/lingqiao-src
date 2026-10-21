#!/usr/bin/env bash
set -Eeuo pipefail

JAVA_SERVICE="${JAVA_SERVICE:-lingqiao-java.service}"
GO_SERVICE="${GO_SERVICE:-injector-server.service}"
JAVA_BASE_URL="${JAVA_BASE_URL:-http://127.0.0.1:49001}"
JAVA_ENV_FILE="${JAVA_ENV_FILE:-/etc/lingqiao-java/lingqiao-java.env}"
EXPECTED_MODULES="${EXPECTED_MODULES:-}"
MODE="${1:---shadow}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

if [[ -z "$EXPECTED_MODULES" && -r "$JAVA_ENV_FILE" ]]; then
  enabled_modules=()
  while IFS='=' read -r key value; do
    value="${value%\"}"
    value="${value#\"}"
    value="${value%\'}"
    value="${value#\'}"
    if [[ "$key" =~ ^JAVA_(EXAM|AUDIT|SCRIPTS|CARDS|SESSIONS|AGENTS|CHAT|BLACKLIST|ANNOUNCEMENT|RELEASES|PAYLOADS)_ENABLED$ ]] \
      && [[ "${value,,}" == "true" ]]; then
      enabled_modules+=("${BASH_REMATCH[1],,}")
    fi
  done < "$JAVA_ENV_FILE"
  EXPECTED_MODULES="$(IFS=,; echo "${enabled_modules[*]}")"
fi

[[ -n "$EXPECTED_MODULES" ]] ||
  fail "no expected Java modules were provided or enabled in $JAVA_ENV_FILE"

systemctl is-active --quiet "$JAVA_SERVICE" || fail "$JAVA_SERVICE is not active"
if [[ "$MODE" == "--shadow" ]]; then
  # Go may be either active or inactive while Java runs in shadow mode.
  :
else
  systemctl is-active --quiet "$GO_SERVICE" &&
    fail "$GO_SERVICE must be inactive in Java production mode"
fi

health_json="$(curl --fail --silent --show-error --max-time 10 \
  "$JAVA_BASE_URL/java/api/health")"
cutover_json="$(curl --fail --silent --show-error --max-time 10 \
  "$JAVA_BASE_URL/java/api/cutover")"

HEALTH_JSON="$health_json" CUTOVER_JSON="$cutover_json" \
EXPECTED_MODULES="$EXPECTED_MODULES" MODE="$MODE" python3 - <<'PY'
import json
import os
import sys

health = json.loads(os.environ["HEALTH_JSON"])
cutover = json.loads(os.environ["CUTOVER_JSON"])
mode = os.environ["MODE"]
expected = [item for item in os.environ["EXPECTED_MODULES"].split(",") if item]

errors = []
if health.get("status") != "ok":
    errors.append("health status is not ok")
if mode == "--shadow" and health.get("production_entrypoint_replaced") is not False:
    errors.append("Java reports that it replaced the production entrypoint in shadow mode")
if mode != "--shadow" and health.get("production_entrypoint_replaced") is not True:
    errors.append("Java does not report production entrypoint replacement")
if cutover.get("status") != "ok":
    errors.append("cutover status is not ok")

modules = cutover.get("modules", {})
for module in expected:
    if not modules.get(module, {}).get("enabled", False):
        errors.append(f"expected shadow module is disabled: {module}")

if mode == "--shadow":
    if cutover.get("enabled") is not False:
        errors.append("global cutover is enabled in shadow mode")
    for name, state in modules.items():
        if state.get("cutover") is not False:
            errors.append(f"module cutover is enabled in shadow mode: {name}")
    for key, value in health.items():
        if key.endswith("_write_enabled") and value is not False:
            errors.append(f"write flag is enabled in shadow mode: {key}")

if errors:
    for error in errors:
        print(f"FAIL: {error}", file=sys.stderr)
    raise SystemExit(1)

print("PASS: Java health and cutover state are valid")
PY

listener="$(ss -H -lntp | grep -E ':49001[[:space:]]' || true)"
[[ "$listener" == *"127.0.0.1"* ]] ||
  fail "Java listener is not restricted to loopback"

for port in 48901 38472; do
  ss -H -lntp | grep -Eq ":${port}[[:space:]]" ||
    fail "legacy TLS port $port is not listening"
done

if [[ "$MODE" == "--shadow" ]]; then
  grep -Eq '^JAVA_WRITE_DB_URL=.*[?&]mode=ro([&].*)?$' "$JAVA_ENV_FILE" ||
    fail "shadow datasource is not read-only"
fi

echo "Verification completed successfully in ${MODE#--} mode."
