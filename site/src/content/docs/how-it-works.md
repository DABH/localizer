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
   `Deleted topic "{name}".` in Python), and the argument is carried into the translation unchanged. A
   captured value must look like a value: a format with little literal text may not capture a lowercase
   phrase, and outside error messages a capture may not span lines or sentences (in help text, it may not
   end one either). Wrapped errors (`%w`) are exempt and translated in turn;
3. the paragraphs and lines of longer text (for help text, tried before single-line format strings), and,
   in help and error text, `label: message` pairs such as `Error: …`.

Anything that doesn’t match is printed exactly as it was. Server responses, IDs, file names and JSON output
pass through untouched as long as they aren’t in your catalog: a line of data that happens to equal a
catalog entry, or to match one of its format strings, is translated like any other line.

Before a translation is used, the runtime checks that it keeps the source’s placeholders, template actions,
markup and backquoted spans, and that it adds no control characters or terminal escape sequences. An entry
that fails the check is ignored, and the English source is printed instead.

## In the service

When you push to your default branch, Localizer:

1. **Downloads the source** of your public repository at that commit. Only source files (`.go`, `.py`),
   project files (`go.mod`, `pyproject.toml`, `setup.py`, `setup.cfg`), the configuration and the catalogs
   are unpacked, in memory. Your code is never built, imported or run, and it isn’t kept after the job.
2. **Extracts user-facing strings** by statically analyzing your code. For Go it collects Cobra, kong and
   urfave/cli commands and flags; `fmt`, `errors` and `log` messages; `localizer.T` calls; message-like struct fields; and the
   output helpers, struct fields and struct tags (such as table headers) that you list in
   [`.localizer.yml`](../reference/configuration/). For Python it collects Typer, Click and argparse
   definitions and command docstrings, output and prompt calls, exception messages, `localizer.t` calls and
   your configured helpers, turning f-strings into templates. Constants are folded across packages and
   modules, and formats are expanded over the values their arguments can take.
3. **Compares them with your catalogs.** New strings are translated. Existing translations are kept as they
   are when they pass validation; an entry that no longer keeps its source’s placeholders, or that adds
   control characters, is translated again and replaced. Strings that disappeared are removed, unless the
   extraction was incomplete or the removal would drop more than 20% of a catalog with more than 20 entries;
   then they are kept and the pull request says so.
4. **Translates the new strings with Claude.** The strings are sent as data, together with where each one
   appears, your glossary, and a style guide for each language.
5. **Validates every translation.** Placeholders, backquoted spans, markup, URLs, flags, `<args>`, glossary
   terms, quoting, paragraph structure and length are checked. Failures are retried with feedback; a string
   that still fails stays in English, and the pull request reports how many failed and why. A string that
   failed validation, or that the model refused to translate, isn’t retried or charged for 30 days.
6. **Opens or updates one pull request** from the `localizer-translations` branch. Catalog entries someone
   edited on that branch are kept; the other files on it are rewritten on each run. With the GitHub Action,
   the service returns the files and your workflow opens the pull request with its own token.

Translations are remembered per repository, so the same string is never paid for twice in the same
repository. Large first runs are split across several jobs automatically.

## What stays in English

- Output that doesn’t come from your code: server responses, IDs, JSON and YAML (unless a line of it matches
  a catalog entry, as described above).
- Debug and trace logs in Go, and all `logging` output in Python, so bug reports stay searchable. This is
  [configurable](../reference/configuration/).
- Sentences your code assembles from English fragments at runtime, such as `"Deleted " + noun + "."`,
  are only partly translated. Full sentences with placeholders translate well.
- Output that bypasses the helpers you hooked, such as full-screen terminal UIs from other libraries.
