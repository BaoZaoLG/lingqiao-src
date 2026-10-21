#!/usr/bin/env bash
set -Eeuo pipefail

PARITYCHECK_BIN="${PARITYCHECK_BIN:-/opt/lingqiao-java/bin/paritycheck}"
PARITY_MANIFEST="${PARITY_MANIFEST:-/opt/lingqiao-java/contracts/java-cutover-contracts.json}"
GO_BASE_URL="${GO_BASE_URL:-https://127.0.0.1:48901}"
JAVA_BASE_URL="${JAVA_BASE_URL:-http://127.0.0.1:49001}"
GO_DB="${GO_DB:-/opt/injector-server/data/lingqiao.db}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ -x "$PARITYCHECK_BIN" ]] || fail "paritycheck binary is not executable"
[[ -r "$PARITY_MANIFEST" ]] || fail "cutover contract manifest is not readable"
[[ -r "$GO_DB" ]] || fail "Go database is not readable"

token="$(GO_DB="$GO_DB" python3 - <<'PY'
import datetime
import hashlib
import os
import secrets
import sqlite3

database = os.environ["GO_DB"]
token = "admin_parity_" + secrets.token_hex(32)
token_hash = hashlib.sha256(token.encode()).hexdigest()
now = datetime.datetime.now(datetime.timezone.utc)
expires = now + datetime.timedelta(minutes=5)

connection = sqlite3.connect(database)
try:
    row = connection.execute(
        "SELECT id FROM admin_users WHERE disabled = 0 "
        "ORDER BY CASE role WHEN 'owner' THEN 0 ELSE 1 END, created_at LIMIT 1"
    ).fetchone()
    if row is None:
        raise SystemExit("no enabled administrator is available")
    connection.execute(
        "INSERT INTO admin_sessions "
        "(token_hash, admin_id, created_at, expires_at, remote_addr, user_agent) "
        "VALUES (?, ?, ?, ?, ?, ?)",
        (
            token_hash,
            row[0],
            now.isoformat().replace("+00:00", "Z"),
            expires.isoformat().replace("+00:00", "Z"),
            "127.0.0.1",
            "lingqiao-shadow-parity",
        ),
    )
    connection.commit()
finally:
    connection.close()
print(token)
PY
)"

cleanup_session() {
  PARITY_TOKEN="$token" GO_DB="$GO_DB" python3 - <<'PY'
import hashlib
import os
import sqlite3

token_hash = hashlib.sha256(os.environ["PARITY_TOKEN"].encode()).hexdigest()
connection = sqlite3.connect(os.environ["GO_DB"])
try:
    connection.execute(
        "DELETE FROM admin_sessions WHERE token_hash = ?",
        (token_hash,),
    )
    connection.commit()
finally:
    connection.close()
PY
}
trap cleanup_session EXIT

if [[ $# -eq 0 ]]; then
  modules=(exam audit)
else
  modules=("$@")
fi

for module in "${modules[@]}"; do
  [[ "$module" =~ ^[a-z][a-z0-9-]*$ ]] || fail "invalid module name: $module"
  echo "Comparing shadow module: $module"
  ADMIN_TOKEN="$token" "$PARITYCHECK_BIN" \
    -go "$GO_BASE_URL" \
    -java "$JAVA_BASE_URL" \
    -manifest "$PARITY_MANIFEST" \
    -module "$module" \
    -insecure
done

echo "Shadow parity completed successfully."
