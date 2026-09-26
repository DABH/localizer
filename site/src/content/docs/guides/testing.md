---
title: Testing and QA
description: Keep golden tests stable, find strings that aren’t localized yet, and try any language.
---

## Keep golden tests in English

Localization follows the user’s environment, so a developer with a Japanese desktop would otherwise get
Japanese output in your tests. Set `LOCALIZER_LANG=en` in the environment of the process under test. In
most test harnesses this is a one-line change:

```go
cmd := exec.Command(binaryPath, args...)
cmd.Env = append(os.Environ(), "LOCALIZER_LANG=en")
```

Tests that build the command tree without going through your `main` aren’t affected, because
`Localize` is never called there.

## Try a language

```sh
LANG=ja_JP.UTF-8 yourcli --help      # the way your users select a language
LOCALIZER_LANG=de yourcli --help     # force one, whatever the system settings are
```

## Find strings that aren’t localized

Pseudo-localization rewrites every string Localizer knows about, so anything left in plain English doesn’t
go through Localizer yet:

```console
$ LOCALIZER_LANG=qps taskctl --help
⟦ţášķçţļ ķééþš á šɱáļļ ļîšţ öƒ ţášķš.
Ûšé îţ ţö áďď, ļîšţ áñď çöɱþļéţé ţášķš.⟧

⟦Ûšáĝé:⟧
  taskctl [command]
```

Two more switches help with coverage:

- `LOCALIZER_DEBUG=1` reports untranslated help strings on stderr.
- `LOCALIZER_DUMP=<file>` writes every help string in the command tree as JSON, including whether each one
  was translated:

  ```json
  {
    "language": "ja",
    "entries": [
      { "kind": "short", "command": "taskctl add", "text": "Add a task.", "translated": true },
      { "kind": "flag", "command": "taskctl add", "flag": "due", "text": "Due date in YYYY-MM-DD format.", "translated": true }
    ]
  }
  ```

  `kind` is one of `short`, `long`, `example`, `deprecated`, `group`, `flag` or `flag_deprecated`.

## Terminal width

Translations can be longer than English, and CJK characters take two columns. Localizer keeps flag
columns aligned the same way Cobra does. For your own tables, prefer a library that measures display
width (for example one based on `go-runewidth`) over `text/tabwriter`, which counts runes.
