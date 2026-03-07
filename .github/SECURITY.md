# Security Policy

## Supported Versions

The following versions of the updater service receive security updates:

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Previous major | Yes |
| Older versions | No |

## Reporting a Vulnerability

Report security vulnerabilities using the GitHub private security advisory workflow:

https://github.com/griffinskudder/updater/security/advisories/new

Please do not open a public issue for security vulnerabilities.

We aim to acknowledge reports within 5 business days.

## Disclosure Policy

This project follows a 90-day coordinated disclosure policy. We ask that you:

1. Report the vulnerability privately using the link above.
2. Allow up to 90 days for us to investigate and release a fix.
3. Coordinate public disclosure timing with us after a fix is available.

We will publish a GitHub Security Advisory when a fix is released.

## Out of Scope

The following are outside the scope of this policy:

- DDoS and resource exhaustion attacks — rate limiting is delegated to the reverse proxy in front of the service, not the application itself.
- Vulnerabilities in upstream dependencies that have not yet been patched by their maintainers.
- Issues that have already been publicly disclosed elsewhere.

## Security Documentation

For the full security architecture, threat model, API key management guidance, and operational security documentation, see [docs/SECURITY.md](../docs/SECURITY.md).
