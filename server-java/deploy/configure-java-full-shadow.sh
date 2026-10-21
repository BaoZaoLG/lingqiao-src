#!/usr/bin/env bash
set -Eeuo pipefail

JAVA_ENV_FILE="${JAVA_ENV_FILE:-/etc/lingqiao-java/lingqiao-java.env}"
JAVA_SERVICE="${JAVA_SERVICE:-lingqiao-java.service}"
VERIFY_SCRIPT="${VERIFY_SCRIPT:-/opt/lingqiao-java/bin/verify.sh}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "configuration must run as root"
[[ -r "$JAVA_ENV_FILE" ]] || fail "Java environment is not readable: $JAVA_ENV_FILE"

declare -A expected=(
  [JAVA_EXAM_ENABLED]=true
  [JAVA_AUDIT_ENABLED]=true
  [JAVA_SCRIPTS_ENABLED]=true
  [JAVA_CARDS_ENABLED]=true
  [JAVA_SESSIONS_ENABLED]=true
  [JAVA_AGENTS_ENABLED]=true
  [JAVA_CHAT_ENABLED]=true
  [JAVA_BLACKLIST_ENABLED]=true
  [JAVA_ANNOUNCEMENT_ENABLED]=true
  [JAVA_RELEASES_ENABLED]=true
  [JAVA_PAYLOADS_ENABLED]=true
  [JAVA_ADMIN_AUTH_ENABLED]=true
  [JAVA_ADMIN_OVERVIEW_ENABLED]=true
  [JAVA_AGENT_PORTAL_ENABLED]=true
  [JAVA_AGENT_PORTAL_WRITE_ENABLED]=false
  [JAVA_CLIENT_DOWNLOAD_ENABLED]=true
  [JAVA_CLIENT_UPDATE_ENABLED]=true
  [JAVA_CLIENT_ANNOUNCEMENT_ENABLED]=true
  [JAVA_RELEASES_DB_URL]=jdbc:sqlite:file:/opt/injector-server/data/releases.db?mode=ro
  [JAVA_EXAM_WRITE_ENABLED]=false
  [JAVA_AUDIT_WRITE_ENABLED]=false
  [JAVA_SCRIPTS_WRITE_ENABLED]=false
  [JAVA_CARDS_WRITE_ENABLED]=false
  [JAVA_SESSIONS_WRITE_ENABLED]=false
  [JAVA_AGENTS_WRITE_ENABLED]=false
  [JAVA_CHAT_WRITE_ENABLED]=false
  [JAVA_BLACKLIST_WRITE_ENABLED]=false
  [JAVA_ANNOUNCEMENT_WRITE_ENABLED]=false
  [JAVA_RELEASES_WRITE_ENABLED]=false
  [JAVA_PAYLOADS_WRITE_ENABLED]=false
  [JAVA_CLIENT_WRITE_ENABLED]=false
  [JAVA_CLIENT_CHAT_ENABLED]=false
  [JAVA_CUTOVER_ENABLED]=false
)

backup="${JAVA_ENV_FILE}.bak.$(date -u +%Y%m%dT%H%M%SZ)"
cp -a "$JAVA_ENV_FILE" "$backup"
temp_file="$(mktemp "$(dirname "$JAVA_ENV_FILE")/.lingqiao-java.env.XXXXXX")"
trap 'rm -f "$temp_file"' EXIT

awk -v keys="$(printf '%s,' "${!expected[@]}")" '
  BEGIN {
    count = split(keys, items, ",")
    for (i = 1; i <= count; i++) {
      if (items[i] != "") managed[items[i]] = 1
    }
  }
  {
    line = $0
    split(line, pair, "=")
    if (!(pair[1] in managed)) print line
  }
' "$JAVA_ENV_FILE" > "$temp_file"

for key in $(printf '%s\n' "${!expected[@]}" | sort); do
  printf '%s=%s\n' "$key" "${expected[$key]}" >> "$temp_file"
done

install -m 0600 "$temp_file" "$JAVA_ENV_FILE"
if ! systemctl restart "$JAVA_SERVICE"; then
  install -m 0600 "$backup" "$JAVA_ENV_FILE"
  systemctl reset-failed "$JAVA_SERVICE" || true
  systemctl restart "$JAVA_SERVICE" || true
  fail "Java restart failed; restored $backup"
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
  install -m 0600 "$backup" "$JAVA_ENV_FILE"
  systemctl reset-failed "$JAVA_SERVICE" || true
  systemctl restart "$JAVA_SERVICE" || true
  fail "Java did not become healthy; restored $backup"
fi

if ! "$VERIFY_SCRIPT" --shadow; then
  install -m 0600 "$backup" "$JAVA_ENV_FILE"
  systemctl reset-failed "$JAVA_SERVICE" || true
  systemctl restart "$JAVA_SERVICE" || true
  fail "full-shadow verification failed; restored $backup"
fi

echo "JAVA_ENV_BACKUP=$backup"
echo "Full Java shadow surface is enabled; all write ownership remains disabled."
