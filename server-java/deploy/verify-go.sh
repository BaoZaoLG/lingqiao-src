#!/usr/bin/env bash
set -Eeuo pipefail

GO_SERVICE="${GO_SERVICE:-injector-server.service}"
JAVA_SERVICE="${JAVA_SERVICE:-lingqiao-java.service}"
GO_HOME_DIR="${GO_HOME_DIR:-/opt/injector-server}"
GO_ENV_FILE="${GO_ENV_FILE:-/etc/injector-server/injector-server.env}"
MODE="${1:---shadow}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

systemctl is-active --quiet "$GO_SERVICE" || fail "$GO_SERVICE is not active"
systemctl is-active --quiet "$JAVA_SERVICE" || fail "$JAVA_SERVICE is not active"
[[ -x "$GO_HOME_DIR/injector-server" ]] || fail "Go gateway binary is not executable"

if ! grep -a -q 'JAVA_CUTOVER_ENABLED' "$GO_HOME_DIR/injector-server"; then
  [[ "${VERIFY_GO_ALLOW_LEGACY:-false}" == "true" ]] ||
    fail "Go gateway does not contain Java cutover support"
  echo "WARNING: verifying a legacy Go rollback binary without cutover support"
fi

for port in 48901 38472; do
  ss -H -lntp | grep -Eq ":${port}[[:space:]]" ||
    fail "Go production port $port is not listening"
done

curl --insecure --fail --silent --show-error --max-time 10 \
  https://127.0.0.1:48901/api/health >/dev/null
curl --insecure --fail --silent --show-error --max-time 10 \
  https://127.0.0.1:38472/api/health >/dev/null

if [[ "$MODE" == "--shadow" ]]; then
  grep -Eq '^JAVA_CUTOVER_ENABLED=false$' "$GO_ENV_FILE" ||
    fail "Go shadow mode requires JAVA_CUTOVER_ENABLED=false"
  if grep -Eq '^JAVA_CUTOVER_[A-Z0-9_]+=true$' "$GO_ENV_FILE"; then
    fail "a Go module cutover flag is enabled in shadow mode"
  fi
fi

db_check="$(sqlite3 -readonly "$GO_HOME_DIR/data/lingqiao.db" 'pragma quick_check;')"
[[ "$db_check" == "ok" ]] || fail "lingqiao.db quick_check failed: $db_check"

if [[ -x /opt/lingqiao-java/bin/verify.sh ]]; then
  /opt/lingqiao-java/bin/verify.sh "$MODE"
fi

echo "Go gateway verification completed successfully in ${MODE#--} mode."
