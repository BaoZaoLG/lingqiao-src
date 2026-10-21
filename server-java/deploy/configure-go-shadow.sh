#!/usr/bin/env bash
set -Eeuo pipefail

GO_ENV_FILE="${GO_ENV_FILE:-/etc/injector-server/injector-server.env}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "configuration must run as root"
[[ -r "$GO_ENV_FILE" ]] || fail "Go environment file is not readable"

declare -A expected=(
  [JAVA_SIDECAR_BASE_URL]="http://127.0.0.1:49001"
  [JAVA_CUTOVER_ENABLED]="false"
  [JAVA_CUTOVER_ADMIN]="false"
  [JAVA_CUTOVER_EXAM]="false"
  [JAVA_CUTOVER_AUDIT]="false"
  [JAVA_CUTOVER_SCRIPTS]="false"
  [JAVA_CUTOVER_CARDS]="false"
  [JAVA_CUTOVER_SESSIONS]="false"
  [JAVA_CUTOVER_AGENTS]="false"
  [JAVA_CUTOVER_RELEASES]="false"
  [JAVA_CUTOVER_PAYLOADS]="false"
  [JAVA_CUTOVER_CHAT]="false"
  [JAVA_CUTOVER_BLACKLIST]="false"
  [JAVA_CUTOVER_ANNOUNCEMENT]="false"
  [JAVA_CUTOVER_AGENT_PORTAL]="false"
  [JAVA_CUTOVER_CARDS_WRITE]="false"
  [JAVA_CUTOVER_BLACKLIST_WRITE]="false"
  [JAVA_CUTOVER_EXAM_WRITE]="false"
  [JAVA_CUTOVER_SESSIONS_WRITE]="false"
  [JAVA_CUTOVER_AGENTS_WRITE]="false"
  [JAVA_CUTOVER_SCRIPTS_WRITE]="false"
  [JAVA_CUTOVER_ANNOUNCEMENT_WRITE]="false"
  [JAVA_CUTOVER_CHAT_WRITE]="false"
  [JAVA_CUTOVER_RELEASES_WRITE]="false"
  [JAVA_CUTOVER_PAYLOADS_WRITE]="false"
  [JAVA_CUTOVER_ADMIN_WRITE]="false"
  [JAVA_CUTOVER_AGENT_PORTAL_WRITE]="false"
)

for key in "${!expected[@]}"; do
  current="$(grep -E "^${key}=" "$GO_ENV_FILE" | tail -1 | cut -d= -f2- || true)"
  if [[ -n "$current" && "$current" != "${expected[$key]}" ]]; then
    fail "$key already has a non-shadow value; refusing to overwrite it"
  fi
done

backup="${GO_ENV_FILE}.bak.$(date -u +%Y%m%dT%H%M%SZ)"
cp -p "$GO_ENV_FILE" "$backup"
chmod 0600 "$backup"

temp_file="$(mktemp "${GO_ENV_FILE}.tmp.XXXXXX")"
cleanup() {
  rm -f "$temp_file"
}
trap cleanup EXIT
cat "$GO_ENV_FILE" >"$temp_file"
echo >>"$temp_file"
echo "# Java sidecar routing; installed disabled for shadow deployment." >>"$temp_file"
for key in $(printf '%s\n' "${!expected[@]}" | sort); do
  if ! grep -Eq "^${key}=" "$GO_ENV_FILE"; then
    printf '%s=%s\n' "$key" "${expected[$key]}" >>"$temp_file"
  fi
done

owner="$(stat -c '%u' "$GO_ENV_FILE")"
group="$(stat -c '%g' "$GO_ENV_FILE")"
install -o "$owner" -g "$group" -m 0600 "$temp_file" "$GO_ENV_FILE"
trap - EXIT
rm -f "$temp_file"

echo "GO_SHADOW_CONFIGURED=true"
echo "GO_ENV_BACKUP=$backup"
