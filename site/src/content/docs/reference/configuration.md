---
title: Configuration
description: The .localizer.yml reference.
---

`.localizer.yml` sits at the root of your repository. Every setting is optional, and a repository without
the file uses the defaults.

```yaml
# Languages to translate into (BCP 47 tags). Default: ja, zh-Hans, ko, es, fr, de, pt-BR
languages: [ja, zh-Hans, ko, es, fr, de, pt-BR]

# Where the catalogs and embed.go live. Default: locales
locales_dir: locales

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
  # Your own output helpers: "import/path.Func" or "*.Method" -> indexes of user-facing arguments.
  funcs:
    example.com/yourcli/pkg/output.Printf: [1]
    example.com/yourcli/pkg/errors.NewErrorWithSuggestions: [0, 1]
    "*.Warnf": [0]
  # Struct fields whose constant values are user-facing.
  fields: [example.com/yourcli/pkg/examples.Example.Text]
  # Struct tags whose values are user-facing, such as table column headers.
  struct_tags: [human]
  # Extra strings printed by third-party libraries that the extractor can't see.
  strings: ["Start an interactive shell."]
  # Log levels whose messages stay in English. Default: [debug, trace]
  skip_log_levels: [debug, trace]
```

## Settings

| Setting | Default | Description |
| --- | --- | --- |
| `languages` | `ja, zh-Hans, ko, es, fr, de, pt-BR` | Languages to translate into. Adding a tag creates its catalog on the next sync. |
| `locales_dir` | `locales` | Directory for the catalogs and `embed.go`, relative to the repository root. |
| `glossary.keep` | none | Terms that must appear untranslated. Every translation is checked for them. |
| `glossary.<lang>` | none | Preferred translations of terms, for one language. |
| `style.<lang>` | built in | Register and tone for one language, in plain words. |
| `extract.include` | the whole repository | Directories to scan, relative to the repository root. |
| `extract.exclude` | none | Glob patterns of files or directories to skip. Test files, `testdata`, `vendor` and hidden directories are always skipped. |
| `extract.funcs` | none | Output helpers whose arguments are user-facing: `"import/path.Func"` (a package function) or `"*.Method"` (a method on any type) → 0-based argument indexes. |
| `extract.fields` | none | Struct fields (`"import/path.Type.Field"`) whose constant values are user-facing. |
| `extract.struct_tags` | none | Struct tags whose values are user-facing. |
| `extract.strings` | none | Strings to translate that don’t appear in your source. |
| `extract.skip_log_levels` | `debug, trace` | Log levels whose messages stay in English. |

Changes to `.localizer.yml` take effect on the next sync. Existing translations aren’t redone when you
change the glossary or style. To retranslate an entry, delete it from the catalog.
