# Lingqiao Release Signing

Every production release package must have:

- semantic version
- channel
- package kind
- file size
- SHA256
- signed manifest
- release notes
- rollout percentage

Client acceptance rules:

1. Manifest signature must verify.
2. Manifest HMAC must verify for the active client session when used.
3. Package SHA256 must match the manifest.
4. Package kind must be supported by the client.
5. Force update must lock risky client actions until completed.
