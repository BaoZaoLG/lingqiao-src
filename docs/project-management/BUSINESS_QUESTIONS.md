# Business Questions

## API And Compatibility

- Should Java eventually preserve both RFC 9457 `application/problem+json` and older JSON `{code,message}` errors for session endpoints that still use compatibility writers?
- Which clients depend on Chinese `message` strings for control flow instead of stable error `code`?
- Should OpenAPI be expanded from the current partial client/update coverage to include all admin and agent APIs before Java cutover?

## Auth

- Should HMAC nonce replay state remain process-local, or should Java introduce persistent/shared nonce storage for multi-process sidecar mode?
- Should generated `hmac_secret.key`, `upload_key`, and manifest seed files remain writable by both Go and Java, or should Java run read-only until cutover?

## Data

- Are `data/lingqiao.db` and `data/releases.db` expected to be backed up together as one unit?
- Should Java Flyway own future schema changes, or should Go startup schema remain authoritative until a full backend switch?
- Should `exam_records.options_json` remain a raw JSON string or become structured JSON in a future API version?

## Modules

- Is `exam` acceptable as the first low-risk Java sample module, or should only `system-health` be shipped in Java initially?
- Should `scripts` remain file-backed long term, or should script metadata move into SQLite before Java writes are enabled?
- Should `chat` stay ephemeral/JSON-like, or should it become a durable SQLite feature before migration?

