#!/usr/bin/env bash
set -Eeuo pipefail

JAVA_SERVICE="${JAVA_SERVICE:-lingqiao-java.service}"
JAVA_HOME_DIR="${JAVA_HOME_DIR:-/opt/lingqiao-java}"
VERIFY_SCRIPT="${VERIFY_SCRIPT:-$JAVA_HOME_DIR/bin/verify.sh}"
VERIFY_MODE="${VERIFY_MODE:---shadow}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "deployment must run as root"
[[ $# -ge 1 && $# -le 2 ]] || fail "usage: $0 <jar> [release-id]"

source_jar="$1"
release_id="${2:-$(date -u +%Y%m%dT%H%M%SZ)}"
[[ -r "$source_jar" ]] || fail "artifact is not readable: $source_jar"
[[ "$release_id" =~ ^[A-Za-z0-9._-]+$ ]] || fail "release-id contains unsafe characters"

install -d -m 0755 "$JAVA_HOME_DIR/releases"
artifact_sha="$(sha256sum "$source_jar" | awk '{print $1}')"
short_sha="${artifact_sha:0:12}"
target_name="lingqiao-server-java-${release_id}-${short_sha}.jar"
target_path="$JAVA_HOME_DIR/releases/$target_name"

if command -v jar >/dev/null 2>&1; then
  jar tf "$source_jar" | grep -q '^BOOT-INF/classes/' ||
    fail "artifact is not an executable Spring Boot jar"
fi

current_target="$(readlink "$JAVA_HOME_DIR/current.jar" 2>/dev/null || true)"
previous_target_before="$(readlink "$JAVA_HOME_DIR/previous.jar" 2>/dev/null || true)"
# Accept both the historical relative form (releases/foo.jar) and the
# production absolute form (/opt/lingqiao-java/releases/foo.jar), but always
# validate the resolved target before using it for rollback.
normalize_release_link() {
  local value="$1"
  if [[ "$value" == /* ]]; then
    local rel="${value#"$JAVA_HOME_DIR/"}"
    [[ "$rel" != "$value" ]] || fail "release link points outside JAVA_HOME_DIR: $value"
    value="$rel"
  fi
  [[ "$value" == releases/* && "$value" != *'/'*'/'* ]] || fail "release link is not a direct releases entry: $value"
  printf '%s' "$value"
}
if [[ -n "$current_target" ]]; then
  current_target="$(normalize_release_link "$current_target")"
fi
if [[ -n "$previous_target_before" ]]; then
  previous_target_before="$(normalize_release_link "$previous_target_before")"
fi

install -m 0644 "$source_jar" "$target_path"
installed_sha="$(sha256sum "$target_path" | awk '{print $1}')"
[[ "$installed_sha" == "$artifact_sha" ]] || fail "installed artifact checksum mismatch"

if [[ -n "$current_target" ]]; then
  ln -sfn "$current_target" "$JAVA_HOME_DIR/previous.jar"
fi
ln -sfn "releases/$target_name" "$JAVA_HOME_DIR/current.jar"

rollback_failed_deploy() {
  echo "Deployment verification failed; restoring the previous artifact." >&2
  if [[ -n "$current_target" ]]; then
    ln -sfn "$current_target" "$JAVA_HOME_DIR/current.jar"
    systemctl restart "$JAVA_SERVICE" || true
  else
    systemctl stop "$JAVA_SERVICE" || true
  fi
  if [[ -n "$previous_target_before" ]]; then
    ln -sfn "$previous_target_before" "$JAVA_HOME_DIR/previous.jar"
  else
    rm -f "$JAVA_HOME_DIR/previous.jar"
  fi
  if [[ -n "$current_target" ]]; then
    for _ in $(seq 1 30); do
      if curl --fail --silent --max-time 2 \
        http://127.0.0.1:49001/java/api/health >/dev/null 2>&1; then
        return
      fi
      sleep 1
    done
    echo "WARNING: previous artifact was restored but did not become healthy" >&2
  fi
}

if ! systemctl restart "$JAVA_SERVICE"; then
  rollback_failed_deploy
  fail "service restart failed"
fi

ready=false
for _ in $(seq 1 30); do
  if curl --fail --silent --max-time 2 \
    http://127.0.0.1:49001/java/api/health >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  rollback_failed_deploy
  fail "service did not become healthy within 30 seconds"
fi

if [[ -x "$VERIFY_SCRIPT" ]] && ! "$VERIFY_SCRIPT" "$VERIFY_MODE"; then
  rollback_failed_deploy
  fail "post-deployment verification failed"
fi

echo "DEPLOYED_RELEASE=$target_name"
echo "DEPLOYED_SHA256=$artifact_sha"
echo "PREVIOUS_RELEASE=${current_target:-none}"
