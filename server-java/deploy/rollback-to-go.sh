#!/usr/bin/env bash
set -Eeuo pipefail

POINTER=/opt/lingqiao-java/last-cutover-backup
NGINX_LIVE=/www/server/panel/vhost/nginx/lingqiao.conf
JAVA_ENV=/etc/lingqiao-java/lingqiao-java.env
JAVA_UNIT=/etc/systemd/system/lingqiao-java.service

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ "$(id -u)" -eq 0 ]] || fail "must run as root"
[[ -r "$POINTER" ]] || fail "cutover backup pointer is missing"
backup_dir="$(cat "$POINTER")"
case "$backup_dir" in
  /opt/lingqiao-backups/full-java-cutover-*) ;;
  *) fail "cutover backup path is outside the expected directory" ;;
esac
for path in \
  "$backup_dir/nginx/lingqiao.conf" \
  "$backup_dir/config/lingqiao-java.env" \
  "$backup_dir/systemd/lingqiao-java.service"; do
  [[ -r "$path" ]] || fail "rollback artifact is missing: $path"
done

cp -a "$backup_dir/nginx/lingqiao.conf" "$NGINX_LIVE"
nginx -t
systemctl reload nginx

for attempt in $(seq 1 30); do
  if ! ss -lntp | grep -E ':(48901|38472)\b' | grep -q nginx; then
    break
  fi
  sleep 1
done
! ss -lntp | grep -E ':(48901|38472)\b' | grep -q nginx ||
  fail "nginx did not release the legacy Go ports"

cp -a "$backup_dir/config/lingqiao-java.env" "$JAVA_ENV"
cp -a "$backup_dir/systemd/lingqiao-java.service" "$JAVA_UNIT"
systemctl daemon-reload
systemctl enable injector-server.service >/dev/null
systemctl restart injector-server.service

for attempt in $(seq 1 30); do
  if curl -kfsS https://127.0.0.1:48901/api/health >/dev/null; then
    break
  fi
  sleep 1
done
curl -kfsS https://127.0.0.1:48901/api/health >/dev/null ||
  fail "Go did not become healthy"

systemctl restart lingqiao-java.service
for attempt in $(seq 1 45); do
  if curl -fsS http://127.0.0.1:49001/java/api/health \
      >/tmp/lingqiao-java-rollback-health.json; then
    break
  fi
  sleep 1
done
curl -fsS http://127.0.0.1:49001/java/api/health \
  >/tmp/lingqiao-java-rollback-health.json ||
  fail "Java shadow did not become healthy"

python3 - <<'PY'
import json
health = json.load(open("/tmp/lingqiao-java-rollback-health.json"))
if health.get("production_entrypoint_replaced"):
    raise SystemExit("Java still reports itself as the production entrypoint")
bad = [key for key, value in health.items()
       if key.endswith("_write_enabled") and value]
if bad:
    raise SystemExit("Java shadow still has writes enabled: " + ", ".join(sorted(bad)))
PY

ss -lntp | grep -E ':48901\b' | grep -q injector-server ||
  fail "Go does not own port 48901"
ss -lntp | grep -E ':38472\b' | grep -q injector-server ||
  fail "Go does not own port 38472"

echo "Rollback completed. Go is the production entrypoint again."
echo "Current data was retained; the backup was not restored over it."
