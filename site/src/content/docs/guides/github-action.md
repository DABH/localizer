---
title: GitHub Action
description: Keep translations in sync from your own workflow, without giving Localizer access to your repository.
---

The Localizer Action keeps your catalogs in sync from a GitHub Actions workflow. The workflow authenticates
with its short-lived GitHub OIDC token, so there is no API key to store. Localizer never gets write access
to your repository: the service returns the updated catalogs, and your workflow opens the pull request
with its own `GITHUB_TOKEN`.

:::note[Private preview]
During the preview, the service accepts workflows from accounts that have access.
[Request access](https://github.com/DABH/localizer/issues/new?template=access.yml).
:::

## Set up

1. Add `.github/workflows/localizer.yml`:

   ```yaml
   name: Localizer
   on:
     push:
       branches: [main]
     workflow_dispatch:

   permissions:
     contents: write       # push the translations branch
     pull-requests: write  # open the pull request
     id-token: write       # authenticate to Localizer with an OIDC token

   jobs:
     translations:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v7
         - uses: DABH/localizer/action@v0.4.1
   ```

2. Turn on **Settings → Actions → General → Workflow permissions → Allow GitHub Actions to create and
   approve pull requests**.

3. Push, or run the workflow from the **Actions** tab. The first run opens the onboarding pull request.

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `github-token` | `${{ github.token }}` | Token that pushes the translations branch and opens the pull request. |
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
- Only `push`, `workflow_dispatch` and `schedule` runs on a branch are accepted. Pull requests, including
  pull requests from forks, can’t spend your quota.
- Localizer serves public repositories only.
- The Action writes only the files the service returns: catalogs, plus the setup files on the first run. It
  refuses absolute paths, `..`, and anything under `.git/` or `.github/`.
- Pin the Action to a release tag or, for the strongest guarantee, to a full commit SHA.
