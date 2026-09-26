---
title: Catalogs
description: The translation catalog format, editing translations, and adding languages.
---

A catalog is a JSON file named after its language (`locales/ja.json`, `locales/pt-BR.json`). It maps
each English source string, exactly as it appears in your code, to its translation:

```json
{
  "language": "ja",
  "messages": {
    "Add a task.": "タスクを追加します。",
    "Due date in YYYY-MM-DD format.": "期限 (YYYY-MM-DD 形式)。",
    "task %d not found": "タスク %d が見つかりません"
  },
  "version": 1
}
```

Keys are sorted, and the file is indented with two spaces, so diffs stay readable. When a source string
changes, it becomes a new key. A stale translation can never be shown: the new string stays in English
until its translation is merged.

## What a translation must keep

The runtime uses a translation only if it keeps everything a program depends on:

- the same format verbs, including argument indexes, flags, width and precision (`%s`, `%[2]d`, `%-8s`);
- the same template actions (`{{.CommandPath}}`);
- the same backquoted spans. pflag shows the first backquoted word as a flag’s value name, so
  `` `level` `` must stay `` `level` ``;
- no control characters or terminal escape sequences that the source doesn’t have.

An entry that breaks a rule is ignored, and the English source is printed instead. The service applies
the same checks, plus a few stricter ones, before it proposes a translation.

## Editing translations

Edit an entry directly and commit it, or suggest a change in the translations pull request. Localizer never
overwrites an existing entry. To have an entry translated again, delete it, and the next sync fills it in.

## Adding a language

Add its [BCP 47](https://www.rfc-editor.org/info/bcp47) tag to `languages` in
[`.localizer.yml`](../configuration/). The next sync creates the catalog.

Users are matched to the closest catalog: `ja_JP.UTF-8` selects `ja`, `de_AT` selects `de`, and `pt_PT`
selects `pt-BR`. Only confident matches are used. Traditional Chinese (`zh_TW`) doesn’t match Simplified
Chinese (`zh-Hans`), so those users see English until you add `zh-Hant`.

## Cobra’s own strings

The library includes translations of Cobra’s and pflag’s built-in text (help headings, the `help` and
`completion` commands, and argument and flag errors) for Japanese, Simplified Chinese, Korean, Spanish,
French, German and Brazilian Portuguese. Your catalogs take precedence over them.

## Catalogs without the service

The runtime reads any catalogs in this format, so you can also write and maintain them yourself, for
example for a private CLI.
