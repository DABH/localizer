---
title: FAQ
description: Common questions about Localizer.
---

## Does Localizer slow down my CLI?

Not noticeably, but it isn’t free. Deciding the language reads a few environment variables or, when none of
them is set, the operating system’s language setting, which on macOS is read from a preferences file; either
takes well under a millisecond.
For a language other than English, one catalog is decoded at startup: a few milliseconds for a CLI with
thousands of strings. The Python runtime costs about 25 ms to import and about 5 ms to select the language,
load the catalogs and install its hooks (Python 3.13 on a recent laptop, measured on the Typer demo in
`python/examples/taskctl`). It imports nothing of Rich or of Typer’s help renderer itself; those load when
help or an error is shown, as they would without Localizer. Help is translated only when it’s shown.

## How big are the catalogs?

Roughly 0.5 MB of JSON per language for every 3,700 strings or so. The Confluent CLI fork’s seven
catalogs total about 3.5 MB, about 4% of its binary; a Python wheel grows by the same JSON.

## Does it make network calls or collect data?

No. Translations are compiled into your binary or shipped inside your package. The runtime never connects
to anything and reports nothing.

## Will it translate my server’s responses or my JSON output?

Not unless a line of it happens to match your catalog. Only text that equals a catalog entry or matches one
of its format strings changes; everything else, including data from servers, IDs, file names and serialized
output, is printed exactly as it was. Keep serialized output (`--output json`) away from the helpers and it
can’t be matched at all.

## Which languages are supported?

By default: Japanese, Simplified Chinese, Korean, Spanish, French, German and Brazilian Portuguese. You can
add any language with its BCP 47 tag in [`.localizer.yml`](../reference/configuration/).

## Which programming languages and frameworks?

Go with Cobra, kong or urfave/cli v3, and Python with Typer, Click or argparse: one line each. Other Go and Python CLIs use the
output helpers after `Init`/`init`. The catalog format is shared, so a CLI ported from one language to the
other keeps its translations.

## Which PyPI package do I install?

`localizer`: `pip install localizer`. The import name is `localizer` as well.

## My CLI builds its commands lazily or through plugins. Does that work?

Yes. In Python, translation happens when help and errors render, on copies of your command objects, so
commands registered after `localize` — plugins attached by an eager callback, lazy groups, app factories —
are covered, and code that compares your original objects’ attributes keeps working. In Go, call
`Localize` on each root you build.

## Can I use it on a monorepo with several CLIs?

A repository has one `.localizer.yml` and one locales directory today, so several CLIs in one repository
share one set of catalogs, and they must be in the same language (Go or Python). Separate configurations
per directory aren’t supported yet.

## How good are the translations? Can I fix one?

Translations are made by Claude, with the string’s context, your glossary and a per-language style guide.
Every translation is validated before it’s proposed, and you review each pull request. To fix one, edit the
catalog entry. Localizer keeps it as long as it is valid (same placeholders as the source, no new control
characters); an entry that fails that check is replaced on the next run.

## Which model translates?

Claude, through Amazon Bedrock. The body of each translations pull request names the model that produced
it.

## How do I retranslate after changing the glossary or the style?

Delete the entries you want redone from the catalogs and push: a valid entry is never retranslated on its
own, so a new glossary term or style rule applies only to new strings and to the entries you delete.

## How long does the first pull request take?

Minutes for a small CLI. A large first run continues across several jobs, each updating the same pull
request.

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
The hosted translation service is a monthly subscription for the GitHub account that owns your
repositories: Solo ($12), Team ($39) and Enterprise ($199), or a custom contract. See [Pricing](/pricing/).

## Is there a trial?

No. Solo is month to month, and if you cancel within 14 days of your first purchase, email us and we refund
that charge (see the [Terms of Service](/terms/)).

## What happens when I cancel?

Your catalogs and pull requests stay in your repository, and the runtime keeps working: nothing in your CLI
changes. The service stops opening and updating pull requests at the end of the period you paid for, and
deletes its copy of your repository’s translation memory 30 days later (see the
[Privacy Policy](/privacy/)).
