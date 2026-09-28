---
title: GitHub App
description: Install the Localizer GitHub App and merge the pull requests it opens.
---

The GitHub App is the hands-off option. Install it on a repository, and Localizer opens a pull request
whenever your strings change. You don’t need to add anything to your workflows.

**[Install the Localizer GitHub App](https://github.com/apps/localedev/installations/new)**, then choose the
account and the repositories to translate. On GitHub the App is called **localedev**, and its pull requests
come from `localedev[bot]`.

:::note[Subscription]
The App works for GitHub accounts with a subscription: see [Pricing](/pricing/).
:::

## What it does

- On installation, it opens the onboarding pull request, **Localize this CLI with Localizer**.
- On every push to your default branch that changes source files (`.go`, `.py`), project files (`go.mod`,
  `pyproject.toml`, `setup.py`, `setup.cfg`) or `.localizer.yml`, it translates the new strings and updates
  one rolling pull request, **Update translations**, from the `localizer-translations` branch.
- It never pushes to your default branch. It writes only your catalogs, plus, in the onboarding pull
  request, `.localizer.yml`, the locales package file (`locales/embed.go` or `locales/__init__.py`), the
  one-line integration and, for Python, the `localizer` dependency in `pyproject.toml`. An allowlist in
  the service enforces this.

## Permissions

| Permission | Why |
| --- | --- |
| Contents: read and write | Read your source, and push the `localizer-translations` branch. |
| Pull requests: read and write | Open and update the translations pull request. |
| Metadata: read | Required by GitHub for every App. |

The App listens to push events to notice string changes, and to installation events to set up new
repositories. Each job uses an installation token scoped to the one repository it works on. The token is
valid for at most an hour and is never stored.

## Private repositories

Localizer serves open-source (public) repositories only. Private repositories are refused at installation
and again on every job. If you need translations for private code, the runtime library works with
catalogs you maintain yourself (see [Catalogs](../../reference/catalogs/)).
