# Design: GitHub Security Policy

Date: 2026-03-07

## Summary

Add `.github/SECURITY.md` — the GitHub-standard vulnerability disclosure policy file. GitHub surfaces this file in the repository's "Security" tab and in the "Report a vulnerability" UI, directing security researchers to the project's responsible disclosure process.

This file is intentionally minimal (Option A). It does not duplicate the security architecture documentation already in `docs/SECURITY.md`; it links there instead.

## What Changes

- **New file**: `.github/SECURITY.md`
- **No changes** to `docs/SECURITY.md` or any other file

## Content Design

### Supported Versions

A two-row table listing:

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Previous major | Yes |
| Older versions | No |

### Reporting a Vulnerability

One paragraph directing researchers to GitHub's private security advisory workflow (the same URL referenced in `docs/SECURITY.md`). No email fallback — GitHub private advisories are the sole reporting channel.

### Disclosure Policy

90-day coordinated disclosure window from the date of the initial report. The maintainer aims to acknowledge reports within 5 business days and will publish a GitHub Security Advisory upon releasing a fix.

### Out of Scope

Issues explicitly outside the scope of this policy:

- DDoS and resource exhaustion (rate limiting is delegated to the reverse proxy, not the application)
- Vulnerabilities in upstream dependencies not yet patched by their maintainers
- Issues already publicly disclosed elsewhere

### Reference

Single link to `docs/SECURITY.md` for the full security architecture, threat model, API key management guidance, and operational security documentation.

## Rationale

GitHub's convention is that `SECURITY.md` is a disclosure policy, not a security manual. Keeping it short and focused avoids duplication and maintenance drift between two security documents.

## Implementation Steps

1. Create `.github/SECURITY.md` with the five sections above.
2. Commit on `feat/github-security-policy` branch.
3. Open a pull request targeting `main`.