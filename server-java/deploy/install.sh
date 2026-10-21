#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
JAVA_HOME_DIR="${JAVA_HOME_DIR:-/opt/lingqiao-java}"
JAVA_CONFIG_DIR="${JAVA_CONFIG_DIR:-/etc/lingqiao-java}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
ENV_SOURCE="${1:-}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "installation must run as root"
[[ -r "$SCRIPT_DIR/lingqiao-java.service" ]] || fail "service template is missing"
for config_name in lingqiao-java-production.service lingqiao-java-gateway.conf; do
  [[ -r "$SCRIPT_DIR/$config_name" ]] ||
    fail "production template is missing: $config_name"
done
for script_name in install.sh preflight.sh verify.sh deploy.sh rollback.sh backup.sh \
  verify-go.sh deploy-go.sh rollback-go.sh configure-go-shadow.sh \
  configure-java-full-shadow.sh parity-shadow.sh \
  cutover-to-java-production.sh rollback-to-go.sh verify-java-production.sh \
  parity-production-read.py parity-agent-portal.py production-write-smoke.py; do
  [[ -r "$SCRIPT_DIR/$script_name" ]] || fail "deployment script is missing: $script_name"
done

java_major="$(java -version 2>&1 | awk -F'[\".]' '/version/ {print $2; exit}')"
[[ "$java_major" =~ ^[0-9]+$ && "$java_major" -ge 17 ]] ||
  fail "Java 17 or newer must be installed first"

install -d -m 0755 \
  "$JAVA_HOME_DIR/releases" "$JAVA_HOME_DIR/bin" "$JAVA_HOME_DIR/config"
install -d -m 0700 "$JAVA_CONFIG_DIR"
for script_name in install.sh preflight.sh verify.sh deploy.sh rollback.sh backup.sh \
  verify-go.sh deploy-go.sh rollback-go.sh configure-go-shadow.sh \
  configure-java-full-shadow.sh parity-shadow.sh \
  cutover-to-java-production.sh rollback-to-go.sh verify-java-production.sh \
  parity-production-read.py parity-agent-portal.py production-write-smoke.py; do
  install -m 0755 "$SCRIPT_DIR/$script_name" "$JAVA_HOME_DIR/bin/$script_name"
done
install -m 0644 "$SCRIPT_DIR/lingqiao-java-production.service" \
  "$JAVA_HOME_DIR/config/lingqiao-java-production.service"
install -m 0644 "$SCRIPT_DIR/lingqiao-java-gateway.conf" \
  "$JAVA_HOME_DIR/config/lingqiao-java-gateway.conf"
install -m 0644 "$SCRIPT_DIR/lingqiao-java.service" \
  "$SYSTEMD_DIR/lingqiao-java.service"

if [[ -n "$ENV_SOURCE" ]]; then
  [[ -r "$ENV_SOURCE" ]] || fail "environment source is not readable: $ENV_SOURCE"
  if [[ -e "$JAVA_CONFIG_DIR/lingqiao-java.env" ]]; then
    echo "PRESERVED_ENV=$JAVA_CONFIG_DIR/lingqiao-java.env"
  else
    install -m 0600 "$ENV_SOURCE" "$JAVA_CONFIG_DIR/lingqiao-java.env"
    echo "INSTALLED_ENV=$JAVA_CONFIG_DIR/lingqiao-java.env"
  fi
fi

systemctl daemon-reload
systemctl enable lingqiao-java.service

echo "INSTALLATION_COMPLETE=true"
echo "NEXT_STEP=$JAVA_HOME_DIR/bin/deploy.sh <jar> [release-id]"
