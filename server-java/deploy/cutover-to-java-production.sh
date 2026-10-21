#!/usr/bin/env bash
set -Eeuo pipefail

JAVA_ENV=/etc/lingqiao-java/lingqiao-java.env
JAVA_UNIT=/etc/systemd/system/lingqiao-java.service
GO_ENV=/etc/injector-server/injector-server.env
GO_UNIT=/etc/systemd/system/injector-server.service
NGINX_LIVE=/www/server/panel/vhost/nginx/lingqiao.conf
NGINX_JAVA=/opt/lingqiao-java/config/lingqiao-java-gateway.conf
JAVA_UNIT_PRODUCTION=/opt/lingqiao-java/config/lingqiao-java-production.service
BACKUP_PARENT=/opt/lingqiao-backups
DATA_DIR=/opt/injector-server/data
JAVA_HEALTH=http://127.0.0.1:49001/java/api/health

backup_dir=
rollback_armed=false

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

wait_java() {
  local attempt
  for attempt in $(seq 1 45); do
    if curl -fsS "$JAVA_HEALTH" >/tmp/lingqiao-java-health.json; then
      return 0
    fi
    sleep 1
  done
  journalctl -u lingqiao-java -n 120 --no-pager >&2 || true
  return 1
}

wait_nginx_release_ports() {
  local attempt
  for attempt in $(seq 1 30); do
    if ! ss -lntp | grep -E ':(48901|38472)\b' | grep -q nginx; then
      return 0
    fi
    sleep 1
  done
  return 1
}

rollback_control_plane() {
  [[ "$rollback_armed" == true && -n "$backup_dir" ]] || return 0
  echo "Cutover failed; restoring the previous Go control plane..." >&2
  cp -a "$backup_dir/nginx/lingqiao.conf" "$NGINX_LIVE"
  cp -a "$backup_dir/config/lingqiao-java.env" "$JAVA_ENV"
  cp -a "$backup_dir/systemd/lingqiao-java.service" "$JAVA_UNIT"
  systemctl daemon-reload
  nginx -t
  systemctl reload nginx
  wait_nginx_release_ports || true
  systemctl restart lingqiao-java || true
  systemctl enable injector-server.service >/dev/null 2>&1 || true
  systemctl restart injector-server.service || true
  echo "Previous Go entrypoint restored. Data was not rewound." >&2
}

on_exit() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    rollback_control_plane
  fi
  exit "$status"
}
trap on_exit EXIT

[[ "$(id -u)" -eq 0 ]] || fail "must run as root"
for path in \
  "$JAVA_ENV" "$JAVA_UNIT" "$GO_ENV" "$GO_UNIT" "$NGINX_LIVE" \
  "$NGINX_JAVA" "$JAVA_UNIT_PRODUCTION" /opt/lingqiao-java/current.jar \
  /opt/lingqiao-java/web-dist/admin/index.html \
  /opt/lingqiao-java/web-dist/agent/index.html; do
  [[ -r "$path" ]] || fail "required path is not readable: $path"
done
[[ -d "$DATA_DIR" ]] || fail "production data directory is missing"
systemctl is-active --quiet injector-server.service ||
  fail "Go must be active before the controlled cutover"
systemctl is-active --quiet lingqiao-java.service ||
  fail "Java shadow must be active before the controlled cutover"
curl -fsS "$JAVA_HEALTH" >/dev/null || fail "Java shadow health check failed"
sqlite3 "$DATA_DIR/lingqiao.db" 'PRAGMA integrity_check;' | grep -qx ok ||
  fail "lingqiao.db integrity check failed"
sqlite3 "$DATA_DIR/releases.db" 'PRAGMA integrity_check;' | grep -qx ok ||
  fail "releases.db integrity check failed"

# Validate the staged gateway without reloading nginx or changing live traffic.
(
  probe="$NGINX_LIVE.java-cutover-probe"
  cp -a "$NGINX_LIVE" "$probe"
  restore_probe() {
    cp -a "$probe" "$NGINX_LIVE"
    rm -f "$probe"
  }
  trap restore_probe EXIT
  install -m 0644 "$NGINX_JAVA" "$NGINX_LIVE"
  nginx -t
)

systemctl stop lingqiao-java-write-smoke.service >/dev/null 2>&1 || true
systemctl stop injector-server.service
! systemctl is-active --quiet injector-server.service ||
  fail "Go did not stop cleanly"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_dir="$BACKUP_PARENT/full-java-cutover-$timestamp"
mkdir -p \
  "$backup_dir/config" "$backup_dir/systemd" "$backup_dir/nginx" "$backup_dir/runtime"
chmod 0700 "$backup_dir"
cp -a "$JAVA_ENV" "$backup_dir/config/lingqiao-java.env"
cp -a "$GO_ENV" "$backup_dir/config/injector-server.env"
cp -a "$JAVA_UNIT" "$backup_dir/systemd/lingqiao-java.service"
cp -a "$GO_UNIT" "$backup_dir/systemd/injector-server.service"
cp -a "$NGINX_LIVE" "$backup_dir/nginx/lingqiao.conf"
readlink -f /opt/lingqiao-java/current.jar >"$backup_dir/runtime/java-artifact.txt"
sha256sum "$(cat "$backup_dir/runtime/java-artifact.txt")" \
  >"$backup_dir/runtime/java-artifact.sha256"
tar -czf "$backup_dir/runtime/injector-data.tar.gz" \
  -C /opt/injector-server data certs
sha256sum "$backup_dir/runtime/injector-data.tar.gz" \
  >"$backup_dir/runtime/injector-data.tar.gz.sha256"
printf '%s\n' "$backup_dir" >/opt/lingqiao-java/last-cutover-backup
rollback_armed=true

JAVA_ENV="$JAVA_ENV" python3 - <<'PY'
import os

path = os.environ["JAVA_ENV"]
updates = {
    "JAVA_SERVER_ADDRESS": "127.0.0.1",
    "JAVA_SERVER_PORT": "49001",
    "JAVA_DATA_DIR": "/opt/injector-server/data",
    "JAVA_DB_URL": "jdbc:sqlite:file:/opt/injector-server/data/lingqiao.db",
    "JAVA_WRITE_DB_URL": "jdbc:sqlite:file:/opt/injector-server/data/lingqiao.db",
    "JAVA_RELEASES_DB_URL": "jdbc:sqlite:file:/opt/injector-server/data/releases.db",
    "JAVA_WEB_DIST_DIR": "/opt/lingqiao-java/web-dist",
    "JAVA_FORWARD_HEADERS_STRATEGY": "native",
    "JAVA_CUTOVER_ENABLED": "false",
    "JAVA_PRODUCTION_ENTRYPOINT_REPLACED": "true",
    "JAVA_EXAM_ENABLED": "true",
    "JAVA_EXAM_WRITE_ENABLED": "true",
    "JAVA_AUDIT_ENABLED": "true",
    "JAVA_AUDIT_WRITE_ENABLED": "true",
    "JAVA_SCRIPTS_ENABLED": "true",
    "JAVA_SCRIPTS_WRITE_ENABLED": "true",
    "JAVA_CARDS_ENABLED": "true",
    "JAVA_CARDS_WRITE_ENABLED": "true",
    "JAVA_SESSIONS_ENABLED": "true",
    "JAVA_SESSIONS_WRITE_ENABLED": "true",
    "JAVA_AGENTS_ENABLED": "true",
    "JAVA_AGENTS_WRITE_ENABLED": "true",
    "JAVA_CHAT_ENABLED": "true",
    "JAVA_CHAT_WRITE_ENABLED": "true",
    "JAVA_BLACKLIST_ENABLED": "true",
    "JAVA_BLACKLIST_WRITE_ENABLED": "true",
    "JAVA_ANNOUNCEMENT_ENABLED": "true",
    "JAVA_ANNOUNCEMENT_WRITE_ENABLED": "true",
    "JAVA_RELEASES_ENABLED": "true",
    "JAVA_RELEASES_WRITE_ENABLED": "true",
    "JAVA_PAYLOADS_ENABLED": "true",
    "JAVA_PAYLOADS_WRITE_ENABLED": "true",
    "JAVA_CLIENT_WRITE_ENABLED": "true",
    "JAVA_CLIENT_DOWNLOAD_ENABLED": "true",
    "JAVA_CLIENT_UPDATE_ENABLED": "true",
    "JAVA_CLIENT_CHAT_ENABLED": "true",
    "JAVA_CLIENT_ANNOUNCEMENT_ENABLED": "true",
    "JAVA_ADMIN_AUTH_ENABLED": "true",
    "JAVA_ADMIN_OVERVIEW_ENABLED": "true",
    "JAVA_AGENT_PORTAL_ENABLED": "true",
    "JAVA_AGENT_PORTAL_WRITE_ENABLED": "true",
}

with open(path, encoding="utf-8") as source:
    lines = source.read().splitlines()
seen = set()
result = []
for line in lines:
    key = line.split("=", 1)[0] if "=" in line and not line.lstrip().startswith("#") else None
    if key in updates:
        result.append("{}={}".format(key, updates[key]))
        seen.add(key)
    else:
        result.append(line)
for key in sorted(set(updates) - seen):
    result.append("{}={}".format(key, updates[key]))
temporary = path + ".java-cutover.tmp"
with open(temporary, "w", encoding="utf-8", newline="\n") as target:
    target.write("\n".join(result) + "\n")
os.chmod(temporary, 0o600)
os.rename(temporary, path)
PY

install -o root -g root -m 0644 "$JAVA_UNIT_PRODUCTION" "$JAVA_UNIT"
systemctl daemon-reload
systemctl restart lingqiao-java.service
wait_java || fail "Java production mode did not become healthy"

python3 - <<'PY'
import json
health = json.load(open("/tmp/lingqiao-java-health.json"))
required = [key for key, value in health.items() if key.endswith("_enabled") and key != "cutover_enabled"]
bad = [key for key in required if health.get(key) is not True]
if health.get("production_entrypoint_replaced") is not True:
    bad.append("production_entrypoint_replaced")
if bad:
    raise SystemExit("Java production health flags are false: " + ", ".join(sorted(bad)))
PY

# The health flags above say nothing about whether the sidecar can actually
# verify a client request. HmacAuthService.resolveSecret() reads HMAC_SECRET, or
# the file at lingqiao.auth.hmac-secret-file — which defaults to
# ./data/hmac_secret.key relative to WorkingDirectory=/opt/lingqiao-java, NOT the
# Go data dir. If neither resolves, every client request fails with
# "HMAC secret unavailable" and, once Go is stopped, the whole client fleet is
# down. Go reads <DATA_DIR>/hmac_secret.key, so the paths differ by default.
#
# Probe with a deliberately wrong signature: the two failure modes are
# distinguishable, so a healthy secret answers "HMAC verification failed".
probe_response="$(curl -ksS -X POST \
  "http://127.0.0.1:49001/java/api/v1/chat/presence" \
  -H 'Content-Type: application/json' \
  -H 'X-Client-ID: injector_v1' \
  -H "X-Timestamp: $(date +%s)" \
  -H 'X-Nonce: cutoverprobe000000000000000000' \
  -H 'X-HMAC-Signature: 0000000000000000000000000000000000000000000000000000000000000000' \
  --data '{"client_id":"cutover-probe"}' || true)"
case "$probe_response" in
  *"HMAC secret unavailable"*)
    fail "Java cannot resolve the HMAC secret — every client request would fail after cutover. Set JAVA_HMAC_SECRET_FILE (or HMAC_SECRET) to the value Go uses, normally ${DATA_DIR}/hmac_secret.key" ;;
  *"HMAC verification failed"*)
    echo "HMAC secret resolves correctly." ;;
  *)
    fail "unexpected HMAC probe response: ${probe_response}" ;;
esac

install -o root -g root -m 0644 "$NGINX_JAVA" "$NGINX_LIVE"
nginx -t
systemctl reload nginx

for attempt in $(seq 1 30); do
  if curl -kfsS https://127.0.0.1:48901/api/health \
      >/tmp/lingqiao-production-health.json &&
     curl -kfsS https://127.0.0.1:48901/admin/ \
      >/tmp/lingqiao-production-admin.html &&
     curl -kfsS https://127.0.0.1:38472/ \
      >/tmp/lingqiao-production-agent.html; then
    break
  fi
  sleep 1
done
grep -qi '<html' /tmp/lingqiao-production-admin.html ||
  fail "admin panel did not load through the legacy port"
grep -qi '<html' /tmp/lingqiao-production-agent.html ||
  fail "agent panel did not load through the legacy port"
python3 - <<'PY'
import json
health = json.load(open("/tmp/lingqiao-production-health.json"))
if health.get("status") != "ok" or not health.get("production_entrypoint_replaced"):
    raise SystemExit("legacy health route is not served by Java production")
PY

systemctl disable injector-server.service >/dev/null
! systemctl is-active --quiet injector-server.service ||
  fail "Go unexpectedly became active"
ss -lntp | grep -E ':48901\b' | grep -q nginx ||
  fail "nginx does not own legacy port 48901"
ss -lntp | grep -E ':38472\b' | grep -q nginx ||
  fail "nginx does not own legacy agent port 38472"
ss -lntp | grep -E ':49001\b' | grep -q java ||
  fail "Java does not own its loopback application port"

cat >"$backup_dir/runtime/cutover-complete.txt" <<EOF
completed_at_utc=$timestamp
java_artifact=$(readlink -f /opt/lingqiao-java/current.jar)
legacy_primary_owner=nginx
legacy_agent_owner=nginx
business_backend=java
go_service=disabled_inactive
EOF

rollback_armed=false
trap - EXIT
echo "Java production cutover completed."
echo "Backup: $backup_dir"
