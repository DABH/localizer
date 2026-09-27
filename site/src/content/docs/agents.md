---
title: Using a coding agent
description: Let Claude Code, Codex or another coding agent integrate Localizer into your CLI.
---

Localizer's integration instructions are written for coding agents as well as people:
[`AGENTS.md`](https://locale.dev/AGENTS.md), the `AGENTS.md` at the root of the repository. It walks
through detecting the language and framework, adding the dependency, creating the locales package, adding
the one line, hooking the CLI's own output helpers, pinning tests to English, verifying with
pseudo-localization, and connecting the repository. An index of these pages for agents is at
[`llms.txt`](https://locale.dev/llms.txt).

Give your agent a prompt such as:

> Read https://locale.dev/AGENTS.md and integrate Localizer into this CLI. Add the GitHub Action workflow it
> describes.

Then review the diff like any other change: a dependency, a `locales` package with empty catalogs, one
line before the app runs, a few `T`/`t` calls at output chokepoints, an English pin in the tests,
`.localizer.yml` and a workflow. Once merged (and the repository is connected), the first Localizer run
opens the pull request that fills in the catalogs.

The agent doesn't translate anything itself: translations always come from the service, and your
corrections to them are never overwritten.
