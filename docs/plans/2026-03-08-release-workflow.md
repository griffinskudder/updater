# Release Workflow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the tag-triggered release job with a controlled `workflow_dispatch` release workflow, and configure a GitHub ruleset to block direct `v*` tag creation by humans.

**Architecture:** A new `release.yml` workflow accepts `version` and `sha` inputs, validates them, verifies CI has passed on the given SHA, then builds/pushes the Docker image, creates the git tag, and creates the GitHub release. The existing `publish.yml` is trimmed to branch-only pushes. A GitHub ruleset (configured in the UI) blocks direct `v*` tag creation by all humans while allowing the GitHub Actions app to bypass.

**Tech Stack:** GitHub Actions, `docker/login-action`, `actions/checkout`, `gh` CLI (pre-installed on `ubuntu-latest`), GitHub REST API (via `gh api`), GNU Make.

**Design doc:** `docs/plans/2026-03-08-release-workflow-design.md`

---

### Task 1: Commit the design doc

**Files:**
- Add: `docs/plans/2026-03-08-release-workflow-design.md`

**Step 1: Stage and commit the design doc**

```bash
git add docs/plans/2026-03-08-release-workflow-design.md
git commit -m "docs: add release workflow design doc"
```

**Step 2: Verify**

```bash
git log --oneline -3
```

Expected: design doc commit is HEAD.

---

### Task 2: Trim `publish.yml`

Remove the `tags: ['v*']` trigger and the entire `release` job from `publish.yml`. The file should retain only the top-level `on`, `permissions`, and the `publish` job.

**Files:**
- Modify: `.github/workflows/publish.yml`

**Step 1: Edit `publish.yml`**

Replace the entire file contents with:

```yaml
name: Publish

on:
  push:
    branches: [main]
  pull_request:

permissions: {}

jobs:
  publish:
    name: Build and push to GHCR
    runs-on: ubuntu-latest
    if: github.ref_type == 'branch'
    permissions:
      contents: read
      packages: write
    steps:
      - name: Checkout
        uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4

      - name: Log in to GHCR
        uses: docker/login-action@c94ce9fb468520275223c153574b00df6fe4bcc9 # v3.7.0
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        env:
          DOCKER_REGISTRY: ghcr.io/${{ github.repository_owner }}
          LATEST_TAG: latest-dev
        run: |
          chmod +x ./scripts/docker-build.sh
          make docker-push
```

**Step 2: Commit**

```bash
git add .github/workflows/publish.yml
git commit -m "ci: remove tag-triggered release job from publish workflow"
```

---

### Task 3: Create `release.yml`

**Files:**
- Create: `.github/workflows/release.yml`

**Step 1: Create the workflow file**

```yaml
name: Release

on:
  workflow_dispatch:
    inputs:
      version:
        description: 'Semantic version tag to create (e.g. v0.0.4)'
        required: true
        type: string
      sha:
        description: 'Full commit SHA to tag and release'
        required: true
        type: string

permissions: {}

jobs:
  release:
    name: Validate, build, tag, and release
    runs-on: ubuntu-latest
    permissions:
      contents: write
      packages: write
    steps:
      - name: Validate version format
        run: |
          if ! echo "${{ inputs.version }}" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
            echo "ERROR: version '${{ inputs.version }}' does not match v<major>.<minor>.<patch>"
            exit 1
          fi

      - name: Checkout (full history for ancestry check)
        uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4
        with:
          fetch-depth: 0

      - name: Validate SHA is on main
        run: |
          if ! git merge-base --is-ancestor "${{ inputs.sha }}" origin/main; then
            echo "ERROR: SHA '${{ inputs.sha }}' is not reachable from origin/main"
            exit 1
          fi

      - name: Verify CI checks passed on SHA
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          echo "Checking CI status for ${{ inputs.sha }}..."
          RUNS=$(gh api \
            "repos/${{ github.repository }}/commits/${{ inputs.sha }}/check-runs" \
            --jq '.check_runs[] | {name: .name, status: .status, conclusion: .conclusion}')

          if [ -z "$RUNS" ]; then
            echo "ERROR: No check runs found for SHA '${{ inputs.sha }}'"
            exit 1
          fi

          FAILED=$(gh api \
            "repos/${{ github.repository }}/commits/${{ inputs.sha }}/check-runs" \
            --jq '[.check_runs[] | select(.status != "completed" or .conclusion != "success")] | length')

          if [ "$FAILED" -gt 0 ]; then
            echo "ERROR: $FAILED check run(s) are not completed and successful:"
            gh api \
              "repos/${{ github.repository }}/commits/${{ inputs.sha }}/check-runs" \
              --jq '.check_runs[] | select(.status != "completed" or .conclusion != "success") | "  - \(.name): status=\(.status) conclusion=\(.conclusion)"'
            exit 1
          fi

          echo "All check runs passed."

      - name: Checkout at release SHA
        uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4
        with:
          ref: ${{ inputs.sha }}

      - name: Log in to GHCR
        uses: docker/login-action@c94ce9fb468520275223c153574b00df6fe4bcc9 # v3.7.0
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push Docker image
        env:
          DOCKER_REGISTRY: ghcr.io/${{ github.repository_owner }}
          VERSION: ${{ inputs.version }}
        run: |
          chmod +x ./scripts/docker-build.sh
          make docker-push

      - name: Create and push git tag
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git tag "${{ inputs.version }}" "${{ inputs.sha }}"
          git push origin "${{ inputs.version }}"

      - name: Create GitHub release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          gh release create "${{ inputs.version }}" \
            --title "${{ inputs.version }}" \
            --generate-notes \
            --target "${{ inputs.sha }}"
```

**Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add controlled release workflow with validation and CI verification"
```

---

### Task 4: Configure the GitHub ruleset (manual step)

This is a one-time manual step in the GitHub UI. It cannot be automated via files in the repo.

**Step 1: Navigate to the ruleset settings**

Go to: `https://github.com/<owner>/<repo>/settings/rules`

Click **New ruleset > New tag ruleset**.

**Step 2: Configure the ruleset**

| Field | Value |
|-------|-------|
| Ruleset name | `Protect release tags` |
| Enforcement status | Active |
| Target tags | Add pattern: `v*` |
| Rules | Check **Restrict creations** and **Restrict deletions** |
| Bypass actors | Click **Add bypass** > select **GitHub Actions** > set bypass mode to **Always** |

Click **Create**.

**Step 3: Verify**

Attempt to push a test tag locally:

```bash
git tag v0.0.0-test
git push origin v0.0.0-test
```

Expected: push is rejected with a ruleset error. Then clean up:

```bash
git tag -d v0.0.0-test
```

---

### Task 5: Open a pull request

**Step 1: Push the branch**

```bash
git push -u origin feat/release-workflow
```

**Step 2: Open a PR**

```bash
gh pr create \
  --title "ci: controlled release workflow with GitHub ruleset" \
  --body "Replaces the tag-triggered release job with a workflow_dispatch release workflow. Requires a GitHub ruleset to be configured per the design doc."
```

---

## Notes

- The `docs.yml` workflow is **not changed**. It still triggers on `v*` tag pushes, which are now created by `release.yml` rather than a human.
- The ruleset (Task 4) must be applied **before** merging this PR to avoid a window where humans can still push `v*` tags using the old workflow.
- If the GitHub Actions app bypass is unavailable on the current plan, fall back to storing a fine-grained PAT as `RELEASE_TOKEN` secret and using it in the tag push step (`git push` authenticated with the PAT), then add the PAT owner as the bypass actor in the ruleset.