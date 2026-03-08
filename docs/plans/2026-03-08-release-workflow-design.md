# Release Workflow Design

## Overview

Introduce a controlled, workflow-dispatch-driven release process that prevents direct `v*` tag creation by humans. Releases are initiated by a developer with appropriate permissions via a GitHub Actions workflow, which validates inputs, verifies CI, and performs all release steps in a single auditable workflow.

## Goals

- Block direct `v*` tag creation by any human actor
- Provide a single, explicit workflow that owns the full release process
- Require CI to have passed on the target commit before releasing
- Keep the `docs.yml` versioned-docs deployment working without changes

## GitHub Ruleset

Configured in the GitHub UI under **Settings > Rules > Rulesets**. Not stored in-repo.

| Field | Value |
|-------|-------|
| Name | `Protect release tags` |
| Target | Tags matching `v*` |
| Enforcement | Active |
| Rules | Restrict creations, Restrict deletions |
| Bypass actors | GitHub Actions app — bypass mode: Always |

The GitHub Actions built-in app is the bypass actor. No new app needs to be created. The `GITHUB_TOKEN` used by workflows authenticates as this app, so the release workflow can push `v*` tags while all human pushes are blocked.

## New Workflow: `release.yml`

Trigger: `workflow_dispatch` only.

### Inputs

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | string | yes | Semantic version tag to create, e.g. `v0.0.4` |
| `sha` | string | yes | Full commit SHA to tag and release |

### Permissions

```yaml
permissions:
  contents: write   # tag creation, GitHub release
  packages: write   # GHCR push
```

### Job Steps

```
1. Validate version format     — must match v\d+\.\d+\.\d+; fail fast with clear error
2. Validate SHA on main        — git merge-base --is-ancestor; ensures SHA is reachable from main
3. Verify CI checks passed     — GitHub API: all check runs on SHA must be completed and successful
4. Checkout at SHA             — actions/checkout at the given SHA
5. Log in to GHCR              — docker/login-action
6. Build and push Docker image — make docker-push with VERSION=${{ inputs.version }}
7. Create and push git tag     — git tag $version $sha && git push origin $version
8. Create GitHub release       — gh release create with --generate-notes
```

Steps 1–3 are pure validation with no side effects. Any failure aborts the workflow before touching Docker or git.

Step 7 (tag push) triggers `docs.yml` to deploy versioned documentation, unchanged from current behaviour.

## Modified Workflow: `publish.yml`

Remove the `tags: ['v*']` entry from `on.push` and delete the `release` job entirely. The file retains only the `publish` job, which builds and pushes a `latest-dev` Docker image on pushes to `main`.

## Unchanged Workflows

| Workflow | Reason |
|----------|--------|
| `docs.yml` | Still triggered by `v*` tag push; the tag is now created by `release.yml` rather than a human, so behaviour is identical |
| `ci.yml` | Unchanged; CI runs on `main` pushes and pull requests as before |

## Security Considerations

- No human can create or delete a `v*` tag directly; all release tag operations go through the workflow
- The workflow validates that the SHA is on `main` and that CI has passed, preventing releases from unreviewed or broken commits
- `GITHUB_TOKEN` permissions are scoped to only what is needed (`contents: write`, `packages: write`)
- The PAT fallback (storing a fine-grained PAT as a secret and adding its owner as a bypass actor) is available if GitHub Actions app bypass is unavailable on the current plan

## Out of Scope

- Rate limiting or approval gates on `workflow_dispatch` (can be added later via GitHub Environments if needed)
- Changelog generation beyond `--generate-notes`
- Automatic version bumping