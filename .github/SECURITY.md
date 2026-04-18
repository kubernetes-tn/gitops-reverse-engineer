# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest release | ✅ |
| older releases | ❌ |

## Reporting a Vulnerability

If you discover a security vulnerability in this project, **please do not open a public issue**.

Instead, report it privately via [GitHub Security Advisories](https://github.com/kubernetes-tn/gitops-reverse-engineer/security/advisories/new).

You will receive a response within **72 hours** acknowledging the report. We will work with you to understand the scope and develop a fix before any public disclosure.

## Scope

The following are in scope for security reports:

- Secret data leaking into Git commits (obfuscation bypass)
- Webhook TLS certificate handling issues
- Git credential exposure in logs or environment
- Container image vulnerabilities in our Dockerfile
- RBAC escalation via the admission controller's ServiceAccount

## Out of Scope

- Vulnerabilities in upstream dependencies (report to the upstream project)
- Denial of service via high admission request volume (by design, the webhook always allows requests)
- Issues requiring cluster-admin access (assumed trusted)
