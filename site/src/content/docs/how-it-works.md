---
title: How it works
description: What happens between a push and a translated CLI.
---

Localizer has two parts. The open-source runtime — a Go module and a Python package — runs inside your CLI.
The hosted service keeps your translation catalogs up to date through pull requests.

## Inside your CLI

Translations live in your repository as JSON [catalogs](../reference/catalogs/), one per language, keyed
by the exact English source string. In Go, `go:embed` compiles them into the binary, so a `go install` or a
distro build gets them too. In Python they are data files of your package, read with `importlib.resources`,
so wheels and zip apps carry them.

At startup Localizer picks a language from the environment and the OS settings (see
[language selection](../reference/runtime/#language-selection)). When that language is English, nothing
changes. Otherwise Localizer translates help text as it renders, the errors your framework prints, and
anything your code sends through `localizer.T` / `localizer.t` and the other helpers. Each lookup tries, in
order:

1. the exact string;
2. format strings in reverse. `Deleted topic "orders".` matches the catalog key `Deleted topic "%s".` (or
   `Deleted topic "{name}".` in Python), and the argument is carried into the translation unchanged;
3. the paragraphs and lines of longer text, and `label: message` pairs such as `Error: …`.

Anything that doesn’t match is printed exactly as it was. Server responses, IDs, file names and JSON output
pass through untouched because they aren’t in your catalog.

Before a translation is used, the runtime checks that it keeps the source’s placeholders, template actions,
markup and backquoted spans, and that it adds no control characters or terminal escape sequences. An entry
that fails the check is ignored, and the English source is printed instead.

## In the service

When you push to your default branch, Localizer:

1. **Downloads the source** of your public repository at that commit. Only source files (`.go`, `.py`),
   project files (`go.mod`, `pyproject.toml`, `setup.py`, `setup.cfg`), the configuration and the catalogs
   are unpacked, in memory. Your code is never built, imported or run, and it isn’t kept after the job.
2. **Extracts user-facing strings** by statically analyzing your code. For Go it collects Cobra commands
   and flags; `fmt`, `errors` and `log` messages; `localizer.T` calls; message-like struct fields; and the
   output helpers, struct fields and struct tags (such as table headers) that you list in
   [`.localizer.yml`](../reference/configuration/). For Python it collects Typer, Click and argparse
   definitions and command docstrings, output and prompt calls, exception messages, `localizer.t` calls and
   your configured helpers, turning f-strings into templates. Constants are folded across packages and
   modules, and formats are expanded over the values their arguments can take.
3. **Compares them with your catalogs.** New strings are translated, strings that disappeared are removed,
   and existing translations are kept as they are.
4. **Translates the new strings with Claude.** The strings are sent as data, together with where each one
   appears, your glossary, and a style guide for each language.
5. **Validates every translation.** Placeholders, backquoted spans, markup, URLs, flags, `<args>`, glossary
   terms, quoting, paragraph structure and length are checked. Failures are retried with feedback, and any
   string that still fails stays in English and is listed in the pull request.
6. **Opens or updates one pull request** from the `localizer-translations` branch. With the GitHub Action,
   the service returns the files and your workflow opens the pull request with its own token.

Translations are remembered, so the same string is never paid for twice. Large first runs are split across
several jobs automatically.

## What stays in English

- Output that doesn’t come from your code: server responses, IDs, JSON and YAML.
- Debug and trace logs in Go, and all `logging` output in Python, so bug reports stay searchable. This is
  [configurable](../reference/configuration/).
- Sentences your code assembles from English fragments at runtime, such as `"Deleted " + noun + "."`,
  are only partly translated. Full sentences with placeholders translate well.
- Output that bypasses the helpers you hooked, such as full-screen terminal UIs from other libraries.
