#!/usr/bin/env bash
set -Eeuo pipefail

GO_SERVICE="${GO_SERVICE:-injector-server.service}"
GO_HOME_DIR="${GO_HOME_DIR:-/opt/injector-server}"
VERIFY_SCRIPT="${VERIFY_GO_SCRIPT:-/opt/lingqiao-java/bin/verify-go.sh}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "rollback must run as root"
previous_target="$(readlink "$GO_HOME_DIR/injector-server.previous" 2>/dev/null || true)"
[[ "$previous_target" == releases/* ]] || fail "no previous Go gateway is available"
[[ -x "$GO_HOME_DIR/$previous_target" ]] || fail "previous Go gateway is not executable"

current_sha="$(sha256sum "$GO_HOME_DIR/injector-server" | awk '{print $1}')"
rollforward_target="releases/injector-server-rollforward-${current_sha:0:12}"
install -m 0755 "$GO_HOME_DIR/injector-server" "$GO_HOME_DIR/$rollforward_target"
next_path="$GO_HOME_DIR/.injector-server.next"
install -m 0755 "$GO_HOME_DIR/$previous_target" "$next_path"
mv -f "$next_path" "$GO_HOME_DIR/injector-server"
ln -sfn "$rollforward_target" "$GO_HOME_DIR/injector-server.previous"
systemctl restart "$GO_SERVICE"

ready=false
for _ in $(seq 1 30); do
  if curl --insecure --fail --silent --max-time 2 \
    https://127.0.0.1:48901/api/health >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
[[ "$ready" == true ]] || fail "rolled-back Go gateway did not become healthy"

if [[ -x "$VERIFY_SCRIPT" ]]; then
  VERIFY_GO_ALLOW_LEGACY=true "$VERIFY_SCRIPT" --shadow
fi

echo "ROLLED_BACK_GO_TO=$previous_target"
echo "GO_ROLL_FORWARD_TARGET=$rollforward_target"
