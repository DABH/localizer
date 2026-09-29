---
title: Configuration
description: The .localizer.yml reference.
---

`.localizer.yml` sits at the root of your repository. Every setting is optional, and a repository without
the file uses the defaults.

```yaml
# go or python. Default: detected from go.mod (Go) or pyproject.toml / setup.py / setup.cfg (Python).
language: python

# Languages to translate into (BCP 47 tags). Default: ja, zh-Hans, ko, es, fr, de, pt-BR
languages: [ja, zh-Hans, ko, es, fr, de, pt-BR]

# Where the catalogs live. Default: locales (Go), or the first package of your CLI (Python), e.g. src/yourcli/locales
locales_dir: src/yourcli/locales

glossary:
  # Terms that are never translated (product names, trademarks).
  keep: [Acme Cloud, Kafka, Kubernetes]
  # Preferred translations, per language.
  ja:
    cluster: クラスター
    topic: トピック

# Register and phrasing per language. Sensible defaults are built in.
style:
  de: Address the user formally (Sie).

extract:
  # Directories to scan. Default: the whole repository.
  include: [cmd, internal, pkg]
  # Files or directories to skip ("**" matches any number of path segments).
  exclude: ["test/**", "**/mock/**"]
  # Your own output helpers -> the arguments that are user-facing.
  funcs:
    # Go: "import/path.Func" -> 0-based argument indexes; "*.Method" matches a method on any type.
    example.com/yourcli/pkg/output.Printf: [1]
    "*.Warnf": [0]
    # Python: "pkg.module.func" (a function or class) or "*.method" -> positional indexes and keyword names.
    yourcli.output.MessageResult: [0, message]
    "*.step": [0]
  # Fields whose constant values are user-facing: Go struct fields ("import/path.Type.Field"), Python
  # keyword arguments and attributes ("pkg.module.Class.attr" or "*.attr").
  fields: [example.com/yourcli/pkg/examples.Example.Text, yourcli.output.Result.hint]
  # Go struct tags whose values are user-facing, such as table column headers.
  struct_tags: [human]
  # Extra strings printed by third-party libraries that the extractor can't see.
  strings: ["Start an interactive shell."]
  # Log levels whose messages stay in English. Default: debug and trace (Go); every level (Python).
  skip_log_levels: [debug, trace]
```

## Settings

| Setting | Default | Description |
| --- | --- | --- |
| `language` | detected | `go` or `python`. Required only when the repository root has both `go.mod` and a Python project file. |
| `languages` | `ja, zh-Hans, ko, es, fr, de, pt-BR` | Languages to translate into. Adding a tag creates its catalog on the next sync. |
| `locales_dir` | `locales` (Go); the CLI’s first package + `/locales` (Python) | Directory for the catalogs and the `embed.go` / `__init__.py` file, relative to the repository root. For Python it must be inside an importable package. |
| `glossary.keep` | none | Terms that must appear untranslated. Every translation is checked for them. |
| `glossary.<lang>` | none | Preferred translations of terms, for one language. |
| `style.<lang>` | built in | Register and tone for one language, in plain words. |
| `extract.include` | the whole repository | Directories to scan, relative to the repository root. |
| `extract.exclude` | none | Glob patterns of files or directories to skip. Always skipped: Go test files, `testdata`, `vendor`, hidden directories; Python test directories (`test`, `tests`, `testing`, `testdata` and their `_`/`-` variants), `conftest.py`, `test_*.py`, `*_test.py`, `setup.py`, virtual environments, `__pycache__`, `node_modules`, `site-packages`, `*.egg-info`, and `build`/`dist` at the root. |
| `extract.funcs` | none | Output helpers whose arguments are user-facing. Go: `"import/path.Func"` or `"*.Method"` → 0-based indexes. Python: `"pkg.module.func"` (functions and classes, re-exports followed) or `"*.method"` → indexes and keyword-argument names. |
| `extract.fields` | none | Go: struct fields (`"import/path.Type.Field"`). Python: keyword arguments of a class (`"pkg.module.Class.attr"`) or attribute assignments (`"*.attr"`). |
| `extract.struct_tags` | none | Go struct tags whose values are user-facing. |
| `extract.strings` | none | Strings to translate that don’t appear in your source. |
| `extract.skip_log_levels` | Go: `debug, trace`; Python: all | Log levels whose messages stay in English. Set a list (even `[]`) to have Python `logging` messages translated. |

## Validation

The service checks `.localizer.yml` before it does anything else, and reports a problem instead of guessing:

- Unknown keys are errors, so a misspelled key (`langauges:`) is caught rather than silently selecting the
  defaults.
- `locales_dir` and the `extract.include` entries are cleaned and must stay inside the repository: relative,
  and neither `.`, `..` nor absolute.
- Language tags must be valid [BCP 47](https://www.rfc-editor.org/info/bcp47) and take their canonical
  spelling; duplicates are dropped, and `en`, the source language, is rejected.
- `extract.exclude` patterns must be valid globs with at most eight `**` segments, and are anchored at the
  repository root.
- Glossary, style and `extract` values are bounded in number and length, and control characters are
  stripped from glossary terms and style text.

The directories that extraction always skips are listed under `extract.exclude` above.

Changes to `.localizer.yml` take effect on the next sync. Existing translations aren’t redone when you
change the glossary or style. To retranslate an entry, delete it from the catalog.
