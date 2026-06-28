# Lingqiao OWASP ASVS Checklist

Target level: ASVS L1 now, L2 for admin/release/security-sensitive flows.

## Authentication

- [ ] Admin password stored with bcrypt.
- [ ] Legacy SHA-256 admin password migration remains one-way.
- [ ] Agent password stored with bcrypt.
- [ ] Session tokens are random and not stored in plaintext where avoidable.

## API Protection

- [ ] Client API requires HMAC headers.
- [ ] HMAC timestamp and nonce are checked.
- [ ] Rate limiting protects activation and login.
- [ ] Problem responses do not leak secrets.

## Uploads And Releases

- [ ] Package upload validates file type, size, hash, and safe path.
- [ ] Release manifest is signed.
- [ ] Client verifies package SHA256.
- [ ] Admin actions write audit entries.

## Web Security

- [ ] CSP is set for admin and agent panels.
- [ ] CORS is same-origin by default.
- [ ] Cookies use secure settings.
- [ ] Sensitive logs are masked.
