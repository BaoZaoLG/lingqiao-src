#!/usr/bin/env bash
set -Eeuo pipefail

JAVA_SERVICE="${JAVA_SERVICE:-lingqiao-java.service}"
JAVA_HOME_DIR="${JAVA_HOME_DIR:-/opt/lingqiao-java}"
VERIFY_SCRIPT="${VERIFY_SCRIPT:-$JAVA_HOME_DIR/bin/verify.sh}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "rollback must run as root"
current_target="$(readlink "$JAVA_HOME_DIR/current.jar" 2>/dev/null || true)"
previous_target="$(readlink "$JAVA_HOME_DIR/previous.jar" 2>/dev/null || true)"
[[ "$current_target" == releases/* ]] || fail "current.jar is not a versioned release"
[[ "$previous_target" == releases/* ]] || fail "previous.jar is not available"
[[ -r "$JAVA_HOME_DIR/$previous_target" ]] || fail "previous artifact is not readable"

ln -sfn "$previous_target" "$JAVA_HOME_DIR/current.jar"
ln -sfn "$current_target" "$JAVA_HOME_DIR/previous.jar"
systemctl restart "$JAVA_SERVICE"

ready=false
for _ in $(seq 1 30); do
  if curl --fail --silent --max-time 2 \
    http://127.0.0.1:49001/java/api/health >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
[[ "$ready" == true ]] || fail "rolled-back service did not become healthy"

if [[ -x "$VERIFY_SCRIPT" ]]; then
  "$VERIFY_SCRIPT" --shadow
fi

echo "ROLLED_BACK_TO=$previous_target"
echo "ROLL_FORWARD_TARGET=$current_target"
