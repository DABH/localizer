---
title: FAQ
description: Common questions about Localizer.
---

## Does Localizer slow down my CLI?

Not noticeably. When output is English, Localizer only reads a few environment variables. For other
languages it decodes one embedded catalog at startup, which takes a few milliseconds for a CLI with
thousands of strings. Help is translated only when it’s shown.

## Does it make network calls or collect data?

No. Translations are compiled into your binary. The runtime never connects to anything and reports nothing.

## Will it translate my server’s responses or my JSON output?

No. Only strings from your catalog change. Everything else, including data from servers, IDs, file names
and serialized output, is printed exactly as it was.

## Which languages are supported?

By default: Japanese, Simplified Chinese, Korean, Spanish, French, German and Brazilian Portuguese. You can
add any language with its BCP 47 tag in [`.localizer.yml`](../reference/configuration/).

## How good are the translations? Can I fix one?

Translations are made by Claude, with the string’s context, your glossary and a per-language style guide.
Every translation is validated before it’s proposed, and you review each pull request. To fix one, edit the
catalog entry. Localizer never overwrites it.

## What if my CLI doesn’t use Cobra?

Call `localizer.Init(locales.FS)` at startup and route your output through `localizer.T`,
`localizer.Sprintf`, `localizer.Error` or `localizer.Writer`. See the
[integration guide](../guides/integration/).

## Why only open-source repositories?

The service reads your source to find user-facing strings. Limiting it to public repositories means
Localizer never needs access to private code, and there’s nothing confidential to protect. For private
code, you can maintain catalogs yourself; the runtime library works the same way.

## Does it support Python?

Not yet. Python support (argparse, Click, Typer) is planned, using the same catalog format.

## What does it cost?

The runtime library is free and open source under the University of Illinois/NCSA Open Source License.
The hosted translation service is in private preview.
