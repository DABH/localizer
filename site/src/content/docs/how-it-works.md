---
title: How it works
description: What happens between a push and a translated CLI.
---

Localizer has two parts. The open-source Go library runs inside your CLI. The hosted service keeps your
translation catalogs up to date through pull requests.

## Inside your CLI

Translations live in your repository as JSON [catalogs](../reference/catalogs/), one per language, keyed
by the exact English source string. `go:embed` compiles them into the binary, so a `go install` or a distro
build gets them too.

At startup Localizer picks a language from the environment and the OS settings (see
[language selection](../reference/runtime/#language-selection)). When that language is English, nothing
changes. Otherwise Localizer translates help text as it renders, the errors Cobra prints, and anything your
code sends through `localizer.T` and the other helpers. Each lookup tries, in order:

1. the exact string;
2. format strings in reverse. `Deleted topic "orders".` matches the catalog key `Deleted topic "%s".`,
   and the argument is carried into the translation unchanged;
3. the paragraphs and lines of longer text, and `label: message` pairs such as `Error: …`.

Anything that doesn’t match is printed exactly as it was. Server responses, IDs, file names and JSON output
pass through untouched because they aren’t in your catalog.

Before a translation is used, the runtime checks that it keeps the source’s placeholders, template actions
and backquoted spans, and that it adds no control characters or terminal escape sequences. An entry that
fails the check is ignored, and the English source is printed instead.

## In the service

When you push to your default branch, Localizer:

1. **Downloads the source** of your public repository at that commit. Only Go files, `go.mod`, the
   configuration and the catalogs are unpacked, in memory. Your code is never built or run, and it isn’t
   kept after the job.
2. **Extracts user-facing strings** by statically analyzing your Go code. It collects Cobra commands and
   flags; `fmt`, `errors` and `log` messages; `localizer.T` calls; message-like struct fields; and the
   output helpers, struct fields and struct tags (such as table headers) that you list in
   [`.localizer.yml`](../reference/configuration/). Constants are folded across packages, and
   `fmt.Sprintf` calls are expanded over the values their arguments can take.
3. **Compares them with your catalogs.** New strings are translated, strings that disappeared are removed,
   and existing translations are kept as they are.
4. **Translates the new strings with Claude.** The strings are sent as data, together with where each one
   appears, your glossary, and a style guide for each language.
5. **Validates every translation.** Placeholders, backquoted spans, URLs, flags, `<args>`, glossary terms,
   quoting, paragraph structure and length are checked. Failures are retried with feedback, and any string
   that still fails stays in English and is listed in the pull request.
6. **Opens or updates one pull request** from the `localizer-translations` branch. With the GitHub Action,
   the service returns the files and your workflow opens the pull request with its own token.

Translations are remembered, so the same string is never paid for twice. Large first runs are split across
several jobs automatically.

## What stays in English

- Output that doesn’t come from your code: server responses, IDs, JSON and YAML.
- Debug and trace logs, so bug reports stay searchable. This is
  [configurable](../reference/configuration/).
- Sentences your code assembles from English fragments at runtime, such as `"Deleted " + noun + "."`,
  are only partly translated. Full sentences with placeholders translate well.
- Output that bypasses the helpers you hooked, such as full-screen terminal UIs from other libraries.
