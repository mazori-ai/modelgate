# Security Policy

## Supported Versions

We release patches for security vulnerabilities in the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of ModelGate seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### How to Report

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to:

**security@modelgate.dev**

You should receive a response within 48 hours. If for some reason you do not, please follow up via email to ensure we received your original message.

### What to Include

Please include the following information in your report:

- **Type of issue** (e.g., buffer overflow, SQL injection, cross-site scripting, etc.)
- **Full paths of source file(s)** related to the issue
- **Location of the affected source code** (tag/branch/commit or direct URL)
- **Step-by-step instructions** to reproduce the issue
- **Proof-of-concept or exploit code** (if possible)
- **Impact of the issue**, including how an attacker might exploit it

### What to Expect

1. **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 48 hours.

2. **Communication**: We will keep you informed about our progress toward a fix and full announcement.

3. **Credit**: We will credit you in the security advisory (unless you prefer to remain anonymous).

4. **Timeline**: We aim to:
   - Confirm the vulnerability within 5 business days
   - Release a fix within 30 days for critical issues
   - Release a fix within 90 days for non-critical issues

## Security Best Practices for Users

### API Keys

- **Never commit API keys** to version control
- Use environment variables or secret management tools
- Rotate API keys regularly
- Use the minimum required permissions

### Deployment

- **Run behind a reverse proxy** (nginx, Traefik) with TLS
- **Enable authentication** for all endpoints
- **Use network policies** to restrict access
- **Keep ModelGate updated** to the latest version

### Configuration

```bash
# Recommended production settings
export MODELGATE_ENCRYPTION_KEY="your-32-byte-encryption-key"
export DATABASE_SSL_MODE="require"
export LOG_LEVEL="info"  # Avoid "debug" in production
```

### Database Security

- Use SSL/TLS connections to PostgreSQL
- Create a dedicated database user with minimal privileges
- Enable connection encryption
- Regular backups with encryption

## Known Security Features

ModelGate includes several built-in security features:

### 1. Prompt Injection Detection
- Pattern-based detection
- Fuzzy matching for obfuscated attacks
- Configurable sensitivity

### 2. PII Detection & Redaction
- Email, phone, SSN, credit card detection
- Configurable actions (block, redact, warn)
- Per-role policies

### 3. Rate Limiting
- Per API key limits
- Token-based limiting
- Burst protection

### 4. Role-Based Access Control
- Fine-grained permissions
- Tool-level authorization
- Model restrictions per role

### 5. Audit Logging
- All API requests logged
- Policy violations tracked
- Searchable audit trail

## Vulnerability Disclosure Policy

We follow a coordinated disclosure process:

1. Reporter submits vulnerability
2. We acknowledge and investigate
3. We develop and test a fix
4. We release the fix
5. We publish a security advisory
6. Reporter is credited (if desired)

## Security Advisories

Security advisories will be published on:
- GitHub Security Advisories
- Our security mailing list (subscribe at security-announce@modelgate.dev)
- Release notes

## Bug Bounty

We currently do not have a formal bug bounty program, but we do offer recognition and swag for responsible disclosure of significant vulnerabilities.

## Contact

- **Security issues**: security@modelgate.dev
- **General questions**: support@modelgate.dev

Thank you for helping keep ModelGate and our users safe!

