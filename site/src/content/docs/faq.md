---
title: FAQ
description: Common questions about Localizer.
---

## Does Localizer slow down my CLI?

Not noticeably. When output is English, Localizer only reads a few environment variables. For other
languages it decodes one catalog at startup, which takes a few milliseconds for a CLI with thousands of
strings. Help is translated only when it’s shown.

## Does it make network calls or collect data?

No. Translations are compiled into your binary or shipped inside your package. The runtime never connects
to anything and reports nothing.

## Will it translate my server’s responses or my JSON output?

No. Only strings from your catalog change. Everything else, including data from servers, IDs, file names
and serialized output, is printed exactly as it was.

## Which languages are supported?

By default: Japanese, Simplified Chinese, Korean, Spanish, French, German and Brazilian Portuguese. You can
add any language with its BCP 47 tag in [`.localizer.yml`](../reference/configuration/).

## Which programming languages and frameworks?

Go with Cobra, and Python with Typer, Click or argparse: one line each. Other Go and Python CLIs use the
output helpers after `Init`/`init`. The catalog format is shared, so a CLI ported from one language to the
other keeps its translations.

## Which PyPI package do I install?

`localizer` (`pip install localizer`), imported as `localizer`. `localizer-py`, the name of the first
release, remains as an alias that installs the same package.

## My CLI builds its commands lazily or through plugins. Does that work?

Yes. In Python, translation happens when help and errors render, on copies of your command objects, so
commands registered after `localize` — plugins attached by an eager callback, lazy groups, app factories —
are covered, and code that compares your original objects’ attributes keeps working. In Go, call
`Localize` on each root you build.

## How good are the translations? Can I fix one?

Translations are made by Claude, with the string’s context, your glossary and a per-language style guide.
Every translation is validated before it’s proposed, and you review each pull request. To fix one, edit the
catalog entry. Localizer never overwrites it.

## What if my CLI uses none of these frameworks?

Call `localizer.Init(locales.FS)` (Go) or `localizer.init("yourcli.locales")` (Python) at startup and
route your output through the helpers. See the [integration guide](../guides/integration/).

## Can a coding agent do the integration?

Yes. Point it at [AGENTS.md](https://locale.dev/AGENTS.md); see [Using a coding agent](../agents/).

## Why only open-source repositories?

The service reads your source to find user-facing strings. Limiting it to public repositories means
Localizer never needs access to private code, and there’s nothing confidential to protect. For private
code, you can maintain catalogs yourself; the runtime library works the same way.

## What does it cost?

The runtime library is free and open source under the University of Illinois/NCSA Open Source License.
The hosted translation service is in private preview.
