#!/usr/bin/env bash
set -Eeuo pipefail

GO_SERVICE="${GO_SERVICE:-injector-server.service}"
GO_HOME_DIR="${GO_HOME_DIR:-/opt/injector-server}"
GO_ENV_FILE="${GO_ENV_FILE:-/etc/injector-server/injector-server.env}"
VERIFY_SCRIPT="${VERIFY_GO_SCRIPT:-/opt/lingqiao-java/bin/verify-go.sh}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "deployment must run as root"
[[ $# -ge 1 && $# -le 2 ]] || fail "usage: $0 <binary> [release-id]"
source_binary="$1"
release_id="${2:-$(date -u +%Y%m%dT%H%M%SZ)}"
[[ -r "$source_binary" ]] || fail "binary is not readable: $source_binary"
[[ "$release_id" =~ ^[A-Za-z0-9._-]+$ ]] || fail "release-id contains unsafe characters"
[[ -r "$GO_ENV_FILE" ]] || fail "Go environment file is not readable"
grep -Eq '^JAVA_CUTOVER_ENABLED=false$' "$GO_ENV_FILE" ||
  fail "deploy the cutover-capable gateway with global cutover disabled"
if grep -Eq '^JAVA_CUTOVER_[A-Z0-9_]+=true$' "$GO_ENV_FILE"; then
  fail "module cutover flags must be false during gateway deployment"
fi

binary_type="$(file "$source_binary")"
grep -Eq 'ELF 64-bit.*x86-64' <<<"$binary_type" ||
  fail "artifact is not a Linux amd64 executable"
grep -a -q 'JAVA_CUTOVER_ENABLED' "$source_binary" ||
  fail "artifact does not contain Java cutover support"

install -d -m 0755 "$GO_HOME_DIR/releases"
new_sha="$(sha256sum "$source_binary" | awk '{print $1}')"
old_sha="$(sha256sum "$GO_HOME_DIR/injector-server" | awk '{print $1}')"
old_release="releases/injector-server-${release_id}-previous-${old_sha:0:12}"
old_release_path="$GO_HOME_DIR/$old_release"
previous_target_before="$(readlink "$GO_HOME_DIR/injector-server.previous" 2>/dev/null || true)"
if [[ -n "$previous_target_before" && "$previous_target_before" != releases/* ]]; then
  fail "injector-server.previous points outside releases/"
fi

install -m 0755 "$GO_HOME_DIR/injector-server" "$old_release_path"
next_path="$GO_HOME_DIR/.injector-server.next"
install -m 0755 "$source_binary" "$next_path"
[[ "$(sha256sum "$next_path" | awk '{print $1}')" == "$new_sha" ]] ||
  fail "staged gateway checksum mismatch"
mv -f "$next_path" "$GO_HOME_DIR/injector-server"
ln -sfn "$old_release" "$GO_HOME_DIR/injector-server.previous"

restore_previous_gateway() {
  echo "Gateway verification failed; restoring the previous binary." >&2
  install -m 0755 "$old_release_path" "$next_path"
  mv -f "$next_path" "$GO_HOME_DIR/injector-server"
  if [[ -n "$previous_target_before" ]]; then
    ln -sfn "$previous_target_before" "$GO_HOME_DIR/injector-server.previous"
  else
    rm -f "$GO_HOME_DIR/injector-server.previous"
  fi
  systemctl restart "$GO_SERVICE" || true
  for _ in $(seq 1 30); do
    if curl --insecure --fail --silent --max-time 2 \
      https://127.0.0.1:48901/api/health >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  echo "WARNING: previous Go gateway was restored but did not become healthy" >&2
}

if ! systemctl restart "$GO_SERVICE"; then
  restore_previous_gateway
  fail "Go service restart failed"
fi

ready=false
for _ in $(seq 1 30); do
  if curl --insecure --fail --silent --max-time 2 \
    https://127.0.0.1:48901/api/health >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  restore_previous_gateway
  fail "Go gateway did not become healthy within 30 seconds"
fi

if [[ -x "$VERIFY_SCRIPT" ]] && ! "$VERIFY_SCRIPT" --shadow; then
  restore_previous_gateway
  fail "Go post-deployment verification failed"
fi

echo "DEPLOYED_GO_SHA256=$new_sha"
echo "PREVIOUS_GO_RELEASE=$old_release"
