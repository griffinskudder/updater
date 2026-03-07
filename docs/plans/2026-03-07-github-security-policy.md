# GitHub Security Policy Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `.github/SECURITY.md` — the GitHub-standard vulnerability disclosure policy file.

**Architecture:** A single new file in `.github/`. Minimal and focused: supported versions, how to report, disclosure timeline, out-of-scope list, and a link to `docs/SECURITY.md`. No code changes.

**Tech Stack:** Markdown, GitHub repository conventions

---

### Task 1: Create `.github/SECURITY.md`

**Files:**
- Create: `.github/SECURITY.md`

No tests apply — this is a policy document, not code.

**Step 1: Create the file**

Create `.github/SECURITY.md` with the following content (verbatim):

```markdown
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
```

**Step 2: Verify the file looks correct**

Open `.github/SECURITY.md` and confirm:
- The supported versions table has three rows (latest, previous major, older)
- The advisory URL points to `griffinskudder/updater`
- All five sections are present

**Step 3: Commit**

```bash
git add .github/SECURITY.md
git commit -m "feat: add GitHub security policy (SECURITY.md)"
```

Expected: one file committed, `feat/github-security-policy` branch.

---

### Task 2: Open a pull request

**Step 1: Push the branch**

```bash
git push -u origin feat/github-security-policy
```

**Step 2: Create the PR**

Use the GitHub MCP (`mcp__plugin_github_github__create_pull_request`) with:

- **title:** `feat: add GitHub security policy`
- **body:**

```
## Summary

- Adds `.github/SECURITY.md` — the GitHub-standard vulnerability disclosure policy
- Supported versions: latest release and previous major
- Reporting channel: GitHub private security advisories
- 90-day coordinated disclosure window
- Links to `docs/SECURITY.md` for full security architecture documentation

## Test plan

- [ ] Verify the file renders correctly on GitHub (Security tab shows the policy)
- [ ] Confirm the "Report a vulnerability" button is active on the repository Security tab
```

- **head:** `feat/github-security-policy`
- **base:** `main`