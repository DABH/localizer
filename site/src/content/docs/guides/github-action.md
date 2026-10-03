---
title: GitHub Action
description: Keep translations in sync from your own workflow, without giving Localizer access to your repository.
---

The Localizer Action keeps your catalogs in sync from a GitHub Actions workflow. The workflow authenticates
with its short-lived GitHub OIDC token, so there is no API key to store. Localizer never gets write access
to your repository: the service returns the updated catalogs, and your workflow pushes the branch and opens
the pull request with the `github-token` input (its own `GITHUB_TOKEN` by default).

:::note[Subscription]
The service accepts workflows from GitHub accounts with a subscription: see [Pricing](/pricing/).
:::

## Set up

1. Add `.github/workflows/localizer.yml`:

   ```yaml
   name: Localizer
   on:
     push:
       branches: [main]    # your default branch
     workflow_dispatch:

   permissions:
     contents: write       # push the translations branch
     pull-requests: write  # open the pull request
     id-token: write       # authenticate to Localizer with an OIDC token

   concurrency:
     group: localizer      # one run at a time: a second push waits for the first
     cancel-in-progress: false

   jobs:
     translations:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v7
         - uses: DABH/localizer/action@v0.6.0
   ```

2. Turn on **Settings → Actions → General → Workflow permissions → Allow GitHub Actions to create and
   approve pull requests**. In an organization-owned repository, the organization’s own Actions settings
   must allow it as well, or the repository setting can’t be turned on.

3. Push, or run the workflow from the **Actions** tab. The first run opens the onboarding pull request.

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `github-token` | `${{ github.token }}` | Token that pushes the translations branch and opens the pull request. Both `git push` and `gh` use it, whatever credentials `actions/checkout` kept. |
| `branch` | `localizer-translations` | Branch for the translations pull request. |
| `timeout-minutes` | `45` | How long to wait for translations. |
| `api-url` | the Localizer service | Localizer API base URL. |
| `audience` | `localizer` | Audience of the OIDC token. It must match the service. |

## Outputs

| Output | Description |
| --- | --- |
| `status` | `up-to-date`, `pr-opened` or `pr-updated` |
| `pull-request` | URL of the translations pull request, if there is one |

## Notes

- Pull requests created with `GITHUB_TOKEN` don’t trigger other workflows. If required checks must run on
  the translations pull request, pass a GitHub App token or a fine-grained personal access token as
  `github-token`.
- Only `push`, `workflow_dispatch` and `schedule` runs on a branch are accepted. The request to Localizer
  carries nothing but the workflow’s OIDC token, which names the repository running it, so a workflow in a
  fork can only sync the fork itself.
- Localizer serves public repositories only.
- The Action writes only the files the service returns: catalogs, plus the setup files on the first run. It
  writes only relative paths to `*.json`, `*.go` and `*.py` files, `pyproject.toml`, and `.localizer.yml` in
  the repository root, and refuses everything else before writing anything: absolute paths, empty, `.` or
  `..` components, backslashes, colons or control characters, and any component that is `.git` or `.github`
  in any letter case (trailing dots and spaces ignored, as Windows drops them). One rejected path means
  nothing is written.
- It commits as `github-actions[bot]`, force-pushes the `branch` with the `github-token` input, and then
  updates the open pull request for that branch from your own repository, or opens one against the branch
  the workflow ran on. A pull request from a fork with the same branch name is never adopted.
- Pin the Action to a release tag or, for the strongest guarantee, to a full commit SHA.
