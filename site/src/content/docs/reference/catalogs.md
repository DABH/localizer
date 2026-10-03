---
title: Catalogs
description: The translation catalog format, editing translations, and adding languages.
---

A catalog is a JSON file named after its language (`locales/ja.json`, `locales/pt-BR.json`). It maps
each English source string, exactly as it appears in your code, to its translation:

```json
{
  "version": 1,
  "language": "ja",
  "messages": {
    "Add a task.": "タスクを追加します。",
    "Due date in YYYY-MM-DD format.": "期限 (YYYY-MM-DD 形式)。",
    "task %d not found": "タスク %d が見つかりません"
  }
}
```

Python catalogs carry `"format": "python"` and use Python placeholders:

```json
{
  "version": 1,
  "language": "ja",
  "format": "python",
  "messages": {
    "Add a task.": "タスクを追加します。",
    "Added task {n}: {title!r}": "タスク {n} を追加しました: {title!r}",
    "task {id} not found": "タスク {id} が見つかりません"
  }
}
```

Keys are sorted, and the file is indented with two spaces, so diffs stay readable. When a source string
changes, it becomes a new key. A stale translation can never be shown: the new string stays in English
until its translation is merged.

## What a translation must keep

The runtime uses a translation only if it keeps everything a program depends on:

- **Go:** the same format verbs, including argument indexes, flags, width and precision (`%s`, `%[2]d`,
  `%-8s`), and the same template actions (`{{.CommandPath}}`);
- **Python:** the same `str.format` fields (`{name}`, `{0}`, `{title!r}`, `{x:>8}`) and printf-style verbs
  (`%s`, `%(name)s`, `%d`); named and numbered fields may be reordered, `{}` and `%s` may not; and the same
  Rich markup tags (`[bold]…[/]`) when the source has any;
- the same backquoted spans, with the first one first: pflag shows the first backquoted word as a flag’s
  value name, so `` `level` `` must stay `` `level` `` and stay first, while later spans may move with the
  sentence; Typer/Click help shows them as code;
- for Python, balanced Rich markup: a closing tag must close a tag that is open at that point;
- no control characters, terminal escape sequences, or characters that reorder or hide text (bidirectional
  controls, line separators, tag characters) that the source doesn’t have; a carriage return only where the
  source has one.

An entry that breaks a rule is ignored, and the English source is printed instead. The service applies
the same checks, plus a few stricter ones, before it proposes a translation, and an existing entry that
fails them is translated again and replaced on the next sync.

## Editing translations

Edit an entry directly and commit it, or edit it on the `localizer-translations` branch of the translations
pull request. Localizer keeps a valid entry wherever it finds it: only an entry that fails the checks above
is replaced, while the files other than catalogs on that branch are rewritten on each run. Entries whose
source string no longer appears in your code are removed, unless the extraction was incomplete or the
removal would drop more than 20% of a catalog with more than 20 entries; then they are kept and the pull
request says so. To have an entry translated again, delete it, and the next sync fills it in.

## Adding a language

Add its [BCP 47](https://www.rfc-editor.org/info/bcp47) tag to `languages` in
[`.localizer.yml`](../configuration/). The next sync creates the catalog.

Users are matched to the closest catalog: `ja_JP.UTF-8` selects `ja`, `de_AT` selects `de`, and `pt_PT`
selects `pt-BR`. Only confident matches are used. Traditional Chinese (`zh_TW`) doesn’t match Simplified
Chinese (`zh-Hans`), so those users see English until you add `zh-Hant`.

## Your framework’s own strings

The runtimes include translations of the frameworks’ built-in text for Japanese, Simplified Chinese,
Korean, Spanish, French, German and Brazilian Portuguese: Cobra’s and pflag’s help headings, `help` and
`completion` commands and argument and flag errors, kong’s and urfave/cli’s headings, help flag and parse
errors (Go); Click’s, Typer’s and argparse’s usage and help
headings, `Show this message and exit.`, `[default: …]`, `[required]`, `Missing argument`, `No such
command`, `Invalid value`, `the following arguments are required`, … across the framework versions in use
(Python). Your catalogs take precedence over them.

## Catalogs without the service

The runtime reads any catalogs in this format, so you can also write and maintain them yourself, for
example for a private CLI.
