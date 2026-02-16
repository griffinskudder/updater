# GitHub Pages with Versioned Docs — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Publish the MkDocs site to GitHub Pages with versioned docs managed by `mike` — `dev` on every push to `main`, tagged releases as version numbers, with the latest stable release as the default landing page.

**Architecture:** A new `.github/workflows/docs.yml` workflow triggers on pushes to `main` and `v*` tags. It installs `mkdocs-material` and `mike` via pip, then uses `mike deploy` to publish to the `gh-pages` branch. MkDocs Material's native version picker reads the `mike`-managed version list from `gh-pages`.

**Tech Stack:** `mike` (MkDocs versioning), `mkdocs-material`, GitHub Actions, GitHub Pages (serving from `gh-pages` branch).

---

### Task 1: Set `site_url` in `mkdocs.yml`

**Files:**
- Modify: `mkdocs.yml:4`

`mike` requires `site_url` to resolve cross-version URLs correctly. It is currently empty.

**Step 1: Update `site_url`**

In `mkdocs.yml`, change line 4 from:

```yaml
site_url: ""
```

to:

```yaml
site_url: https://griffinskudder.github.io/updater/
```

**Step 2: Verify the docs build**

```bash
make docs-build
```

Expected: build completes without errors. The `site/` directory is populated.

**Step 3: Commit**

```bash
git add mkdocs.yml
git commit -m "chore(docs): set site_url for GitHub Pages"
```

---

### Task 2: Create the GitHub Actions docs workflow

**Files:**
- Create: `.github/workflows/docs.yml`

**Step 1: Create the workflow file**

Create `.github/workflows/docs.yml` with the following content:

```yaml
name: Docs

on:
  push:
    branches: [main]
  push:
    tags: ['v*']

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-python@v5
        with:
          python-version: '3.x'

      - name: Install dependencies
        run: pip install mkdocs-material mike

      - name: Configure git credentials
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"

      - name: Deploy dev docs
        if: github.ref == 'refs/heads/main'
        run: mike deploy --push --update-aliases dev

      - name: Deploy versioned docs
        if: startsWith(github.ref, 'refs/tags/v')
        run: |
          VERSION=${GITHUB_REF#refs/tags/}
          mike deploy --push --update-aliases "$VERSION" latest
          mike set-default --push latest
```

Note: `fetch-depth: 0` is required — `mike` reads and writes the `gh-pages` branch, which needs the full git history to avoid overwriting previous versions.

`--update-aliases` ensures the `latest` alias always points to the version being deployed, replacing any previous pointer.

**Step 2: Commit**

```bash
git add .github/workflows/docs.yml
git commit -m "ci: add GitHub Pages docs deployment workflow"
```

---

### Task 3: Add implementation plan to docs nav and commit

**Files:**
- Modify: `mkdocs.yml`

**Step 1: Add implementation plan to nav**

In `mkdocs.yml`, find the `Plans:` section in `nav` and add:

```yaml
    - GitHub Pages Implementation: plans/2026-02-16-github-pages-implementation.md
```

after the existing `GitHub Pages Design` entry.

**Step 2: Verify the docs build**

```bash
make docs-build
```

Expected: build completes without errors.

**Step 3: Commit**

```bash
git add mkdocs.yml docs/plans/2026-02-16-github-pages-implementation.md
git commit -m "docs: add GitHub Pages implementation plan"
```

---

### Task 4: Configure GitHub Pages in repository settings (manual)

This is a one-time action in the GitHub web UI. `mike` creates and manages the `gh-pages` branch automatically on first deploy, but GitHub Pages won't serve it until you point the setting at it.

**Step 1: Push changes to `main`**

```bash
git push
```

This triggers the workflow and creates the `gh-pages` branch with the `dev` docs.

**Step 2: Wait for the workflow to complete**

In the GitHub Actions tab, confirm the `Docs` workflow run succeeds and the `gh-pages` branch now exists.

**Step 3: Configure GitHub Pages source**

Go to the repository on GitHub: *Settings → Pages → Build and deployment → Source*.

Set:
- Source: `Deploy from a branch`
- Branch: `gh-pages`
- Folder: `/ (root)`

Save. GitHub Pages will begin serving from `https://griffinskudder.github.io/updater/`.

**Step 4: Verify**

Visit `https://griffinskudder.github.io/updater/`. The site should load and show the `dev` docs (there are no tagged releases yet, so `latest` does not exist; the `dev` docs will be visible directly).

Once a `v*` tag is pushed, the `latest` alias will be created and the default will switch to the tagged version.