---
name: Bug Report
about: Report a bug to help us improve ModelGate
title: '[BUG] '
labels: bug
assignees: ''
---

## Description

A clear and concise description of what the bug is.

## Steps to Reproduce

1. Go to '...'
2. Click on '...'
3. Send request '...'
4. See error

## Expected Behavior

A clear and concise description of what you expected to happen.

## Actual Behavior

What actually happened instead.

## Screenshots / Logs

If applicable, add screenshots or log output to help explain your problem.

```
Paste logs here
```

## Environment

- **ModelGate Version**: [e.g., 1.0.0]
- **Deployment Method**: [Docker / Binary / Kubernetes]
- **OS**: [e.g., Ubuntu 22.04, macOS 14]
- **Go Version** (if building from source): [e.g., 1.22]
- **PostgreSQL Version**: [e.g., 16]
- **Browser** (if UI issue): [e.g., Chrome 120]

## Configuration

Relevant configuration (redact sensitive values):

```yaml
# Paste relevant config here
```

## API Request (if applicable)

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{...}'
```

## Additional Context

Add any other context about the problem here.

## Checklist

- [ ] I have searched existing issues to ensure this is not a duplicate
- [ ] I have included all relevant information above
- [ ] I have redacted any sensitive information (API keys, passwords, etc.)

