#!/usr/bin/env bash
set -Eeuo pipefail

JAVA_SERVICE="${JAVA_SERVICE:-lingqiao-java.service}"
GO_SERVICE="${GO_SERVICE:-injector-server.service}"
JAVA_HOME_DIR="${JAVA_HOME_DIR:-/opt/lingqiao-java}"
GO_HOME_DIR="${GO_HOME_DIR:-/opt/injector-server}"
JAVA_ENV_FILE="${JAVA_ENV_FILE:-/etc/lingqiao-java/lingqiao-java.env}"
MODE="${1:---shadow}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "PASS: $*"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command is missing: $1"
}

for command_name in systemctl java curl ss sqlite3 sha256sum readlink grep awk stat df; do
  require_command "$command_name"
done

[[ "$MODE" == "--shadow" || "$MODE" == "--write-ready" ]] ||
  fail "usage: $0 [--shadow|--write-ready]"
[[ -r "$JAVA_ENV_FILE" ]] || fail "environment file is not readable: $JAVA_ENV_FILE"
[[ "$(stat -c '%a' "$JAVA_ENV_FILE")" == "600" ]] ||
  fail "environment file must have mode 600"
[[ -r "$GO_HOME_DIR/data/lingqiao.db" ]] || fail "main database is not readable"

java_major="$(java -version 2>&1 | awk -F'[\".]' '/version/ {print $2; exit}')"
[[ "$java_major" =~ ^[0-9]+$ && "$java_major" -ge 17 ]] ||
  fail "Java 17 or newer is required"
pass "Java runtime is version $java_major or newer"

systemctl is-active --quiet "$GO_SERVICE" || fail "$GO_SERVICE is not active"
pass "$GO_SERVICE is active"

if systemctl is-enabled --quiet "$JAVA_SERVICE"; then
  pass "$JAVA_SERVICE is enabled"
else
  fail "$JAVA_SERVICE is not enabled"
fi

current_target="$(readlink "$JAVA_HOME_DIR/current.jar" 2>/dev/null || true)"
[[ "$current_target" == releases/* ]] ||
  fail "current.jar must point to a versioned artifact under releases/"
[[ -r "$JAVA_HOME_DIR/$current_target" ]] || fail "current Java artifact is not readable"
previous_target="$(readlink "$JAVA_HOME_DIR/previous.jar" 2>/dev/null || true)"
if [[ -n "$previous_target" ]]; then
  [[ "$previous_target" == releases/* ]] ||
    fail "previous.jar must point to a versioned artifact under releases/"
  [[ -r "$JAVA_HOME_DIR/$previous_target" ]] || fail "previous Java artifact is not readable"
fi
sha256sum "$JAVA_HOME_DIR/current.jar"
pass "current Java artifact is versioned and readable"

# The cutover records the artifact sha256 but has nothing to compare it against,
# so an older JAR deploys silently. Any JAR built before the client-contract fix
# rejects the client's bodies (client_id is not in the DTOs; Go's encoding/json
# ignores unknown fields, Jackson does not) and reports it as INVALID_JSON.
# After the cutover there is no Go fallback, so chat and update checks simply
# break. Read the class files directly — no javac is required on the server.
python3 - "$JAVA_HOME_DIR/current.jar" <<'PY'
import sys, zipfile

jar = sys.argv[1]
needed = (
    "BOOT-INF/classes/com/lingqiao/server/chat/ClientChatController$ChatRequest.class",
    "BOOT-INF/classes/com/lingqiao/server/client/ClientUpdateController$UpdateCheckRequest.class",
)
marker = b"Lcom/fasterxml/jackson/annotation/JsonIgnoreProperties;"

with zipfile.ZipFile(jar) as archive:
    names = set(archive.namelist())
    absent = [n for n in needed if n not in names]
    if absent:
        raise SystemExit("artifact is missing: " + ", ".join(absent))
    for name in needed:
        if marker not in archive.read(name):
            raise SystemExit(
                "artifact predates the unknown-field tolerance fix (" + name +
                "); rebuild it from the current source before cutting over")
print("Java artifact carries the client contract fix.")
PY
pass "current Java artifact carries the client contract fix"

grep -Eq '^JAVA_SERVER_ADDRESS=127\.0\.0\.1$' "$JAVA_ENV_FILE" ||
  fail "Java sidecar must bind to 127.0.0.1"
grep -Eq '^JAVA_SERVER_PORT=49001$' "$JAVA_ENV_FILE" ||
  fail "Java sidecar must use port 49001"

if [[ "$MODE" == "--shadow" ]]; then
  grep -Eq '^JAVA_CUTOVER_ENABLED=false$' "$JAVA_ENV_FILE" ||
    fail "shadow mode requires JAVA_CUTOVER_ENABLED=false"
  if grep -Eq '^JAVA_[A-Z0-9_]*WRITE_ENABLED=true$' "$JAVA_ENV_FILE"; then
    fail "shadow mode forbids Java write flags"
  fi
  grep -Eq '^JAVA_WRITE_DB_URL=.*[?&]mode=ro([&].*)?$' "$JAVA_ENV_FILE" ||
    fail "shadow mode requires JAVA_WRITE_DB_URL with mode=ro"
  pass "shadow configuration disables cutover and write ownership"
fi

db_check="$(sqlite3 -readonly "$GO_HOME_DIR/data/lingqiao.db" 'pragma quick_check;')"
[[ "$db_check" == "ok" ]] || fail "lingqiao.db quick_check failed: $db_check"
pass "lingqiao.db quick_check is ok"

available_kb="$(df -Pk "$JAVA_HOME_DIR" | awk 'NR==2 {print $4}')"
[[ "$available_kb" -ge 524288 ]] || fail "less than 512 MiB is available for deployment"
pass "at least 512 MiB is available"

if systemctl is-active --quiet "$JAVA_SERVICE"; then
  java_listener="$(ss -H -lntp | grep -E ':49001[[:space:]]' || true)"
  [[ "$java_listener" == *"127.0.0.1"* ]] ||
    fail "active Java service is not loopback-only on port 49001"
  pass "$JAVA_SERVICE is active and loopback-only"
fi

for port in 48901 38472; do
  ss -H -lntp | grep -Eq ":${port}[[:space:]]" ||
    fail "Go production port $port is not listening"
done
pass "Go production ports 48901 and 38472 are listening"

echo "Preflight completed successfully in ${MODE#--} mode."
