#!/usr/bin/env bash
set -Eeuo pipefail

GO_SERVICE="${GO_SERVICE:-injector-server.service}"
JAVA_SERVICE="${JAVA_SERVICE:-lingqiao-java.service}"
BACKUP_ROOT="${BACKUP_ROOT:-/opt/lingqiao-backups}"
MODE="${1:---consistent}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "backup must run as root"
[[ "$MODE" == "--consistent" || "$MODE" == "--dry-run" ]] ||
  fail "usage: $0 [--consistent|--dry-run]"

backup_id="$(date -u +%Y%m%dT%H%M%SZ)"
archive="$BACKUP_ROOT/lingqiao-runtime-$backup_id.tar.gz"
manifest="$archive.sha256"
paths=(
  opt/injector-server/data
  opt/injector-server/certs
  opt/injector-server/injector-server
  etc/injector-server
  etc/lingqiao-java
  etc/systemd/system/injector-server.service
  etc/systemd/system/lingqiao-java.service
  opt/lingqiao-java/releases
  opt/lingqiao-java/current.jar
  opt/lingqiao-java/previous.jar
)

if [[ -d /opt/injector-server/releases ]]; then
  paths+=(opt/injector-server/releases)
fi
if [[ -L /opt/injector-server/injector-server.previous ]]; then
  paths+=(opt/injector-server/injector-server.previous)
fi

for path in "${paths[@]}"; do
  [[ -e "/$path" || -L "/$path" ]] || fail "required backup path is missing: /$path"
done

if [[ "$MODE" == "--dry-run" ]]; then
  echo "Backup preflight succeeded."
  printf '/%s\n' "${paths[@]}"
  exit 0
fi

install -d -m 0700 "$BACKUP_ROOT"
go_was_active=false
java_was_active=false
systemctl is-active --quiet "$GO_SERVICE" && go_was_active=true
systemctl is-active --quiet "$JAVA_SERVICE" && java_was_active=true

resume_services() {
  if [[ "$go_was_active" == true ]]; then
    systemctl start "$GO_SERVICE" || true
  fi
  if [[ "$java_was_active" == true ]]; then
    systemctl start "$JAVA_SERVICE" || true
  fi
}
trap resume_services EXIT

[[ "$java_was_active" == false ]] || systemctl stop "$JAVA_SERVICE"
[[ "$go_was_active" == false ]] || systemctl stop "$GO_SERVICE"

for database in /opt/injector-server/data/lingqiao.db \
  /opt/injector-server/data/releases.db; do
  if [[ -f "$database" ]]; then
    result="$(sqlite3 "$database" 'pragma quick_check;')"
    [[ "$result" == "ok" ]] || fail "database check failed before backup: $database"
  fi
done

tar -C / -czf "$archive" "${paths[@]}"
chmod 0600 "$archive"
sha256sum "$archive" >"$manifest"
chmod 0600 "$manifest"
tar -tzf "$archive" >/dev/null

trap - EXIT
resume_services
systemctl is-active --quiet "$GO_SERVICE" || fail "$GO_SERVICE did not resume"
if [[ "$java_was_active" == true ]]; then
  systemctl is-active --quiet "$JAVA_SERVICE" || fail "$JAVA_SERVICE did not resume"
fi

echo "BACKUP_ARCHIVE=$archive"
echo "BACKUP_MANIFEST=$manifest"
