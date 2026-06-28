# Lingqiao Secrets Management

Never commit runtime secrets.

Ignored local runtime files include:

- `injector-server/data/admin_password.hash`
- `injector-server/data/hmac_secret.key`
- `injector-server/certs/`
- generated client secret headers

Transfer rule:

1. Transfer source code through Git.
2. Transfer production runtime data separately through encrypted backup.
3. Rotate `ADMIN_PASSWORD`, `HMAC_SECRET`, TLS certificates, release signing key, and client embedded configuration when ownership changes.
4. Rebuild the client after rotating embedded client credentials.
