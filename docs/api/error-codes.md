# Lingqiao API Error Codes

All new API errors should use RFC 9457 Problem Details with `Content-Type: application/problem+json`.

Required fields:

- `type`: stable documentation URI for the problem type.
- `title`: short English title for logs and tooling.
- `status`: HTTP status code.
- `detail`: localized human-readable message when safe to expose.
- `instance`: request path or request-specific URI.
- `code`: stable Lingqiao business error code.
- `request_id`: correlation ID when available.

Initial error codes:

| Code | HTTP | Meaning |
| --- | --- | --- |
| `INVALID_JSON` | 400 | Request body is not valid JSON. |
| `METHOD_NOT_ALLOWED` | 405 | Endpoint does not allow the HTTP method. |
| `HMAC_MISSING` | 401 | Required HMAC headers are missing. |
| `HMAC_INVALID` | 401 | HMAC verification failed. |
| `CARD_NOT_FOUND` | 404 | Card code does not exist. |
| `CARD_BLACKLISTED` | 403 | Card is blacklisted. |
| `CARD_DISABLED` | 403 | Card is disabled. |
| `CARD_EXPIRED` | 403 | Card expired. |
| `CARD_BOUND_TO_OTHER_MACHINE` | 403 | Card is already bound to another machine. |
| `MAX_SESSIONS_REACHED` | 403 | Card reached active session limit. |
| `SESSION_NOT_FOUND` | 401 | Session token does not exist. |
| `SESSION_EXPIRED` | 401 | Session expired. |
| `MACHINE_MISMATCH` | 403 | Session machine does not match request machine. |
| `RELEASE_SERVICE_UNAVAILABLE` | 503 | Release subsystem is unavailable. |
| `PACKAGE_NOT_FOUND` | 404 | Release package metadata or file is missing. |
