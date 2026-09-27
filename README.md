# Localizer

Localizer renders a Go CLI's own strings (help text, flag descriptions, messages, errors) in the user's
language. Translations are produced by AI, kept in sync automatically through pull requests, and compiled
into your binary. Localization adds no network calls and no noticeable startup cost.

**Documentation: https://locale.dev/**

```go
import (
	"github.com/DABH/localizer"
	"example.com/yourcli/locales"
)

func main() {
	root := newRootCmd()
	localizer.Localize(root, locales.FS) // ← the whole integration for Cobra CLIs
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
```

```console
$ LANG=ja_JP.UTF-8 taskctl add --help
タスクを追加します。

使い方:
  taskctl add <title> [flags]

フラグ:
      --due string       期限 (YYYY-MM-DD 形式)。
  -h, --help             add のヘルプ
      --priority level   タスクの優先度: level は low、normal、high のいずれかです。 (デフォルト: "normal")
```

`locales/embed.go` embeds one JSON catalog per language (`ja.json`, `de.json`, …), keyed by the exact
English source string. The hosted Localizer service keeps the catalogs up to date. On every push that
changes user-facing strings, it translates the new ones and opens or updates one pull request. Connect it
with:

- the **[Localizer GitHub Action](https://locale.dev/guides/github-action/)**. Your workflow
  authenticates with its short-lived GitHub OIDC token, so there's no API key, and Localizer never gets
  write access to your repository; or
- the **[Localizer GitHub App](https://locale.dev/guides/github-app/)**. Install it and merge
  the pull requests it opens.

The first pull request sets everything up, including the line above. The hosted service is in private
preview: see [Getting started](https://locale.dev/getting-started/).

Catalogs are plain JSON that you can also write or edit by hand. Localizer never overwrites an existing
translation.

## What gets translated

With the one line above: every command's `Short`, `Long`, `Example` and deprecation text, all flag
descriptions, help and usage headings, Cobra's built-in `help` and `completion` commands, shell-completion
descriptions, and the errors Cobra prints (unknown commands and flags, argument counts, required
flags, …).

Your own runtime messages go through a few helpers at your output chokepoints:

| Helper | Use |
| --- | --- |
| `localizer.T(s)` | Translate a string. For a format string, call it before formatting. |
| `localizer.Sprintf(format, args...)` | `fmt.Sprintf` with a translated format. |
| `localizer.Error(err)` | Translate an error chain for display. Server text stays as it is. |
| `localizer.Writer(w)` | An `io.Writer` that translates CLI strings written through it. |

Dynamic data (server responses, IDs, JSON and YAML output) is never translated. Only strings found in your
catalog change, and anything else passes through untouched.

## Language selection

`LOCALIZER_LANG` (or an app-specific variable added with `localizer.WithEnvVar`) → `LC_ALL` →
`LC_MESSAGES` → `LANG` (with GNU `LANGUAGE`) → the OS setting (macOS preferred languages, Windows display
language) → English. `C` and `POSIX` locales keep English, so scripts stay stable. `LOCALIZER_LANG=off`
disables localization. `LOCALIZER_LANG=qps` pseudo-localizes every known string, which is handy for
spotting strings that don't go through Localizer yet.

## Try it

```sh
go run ./examples/demo --help
LANG=ja_JP.UTF-8 go run ./examples/demo add --help
LANG=ja_JP.UTF-8 go run ./examples/demo done 9
LOCALIZER_LANG=qps go run ./examples/demo --help
```

## Repository layout

| Path | What |
| --- | --- |
| `/` | The runtime library: the only package your CLI imports. Dependencies: cobra, pflag, x/text, x/sys. |
| `catalog/`, `engine/`, `msgfmt/` | Catalog format, lookup engine, and format-string and template parsing, exported for tools. |
| `internal/` | Locale detection, and built-in translations of Cobra's own strings. |
| `action/` | The GitHub Action. |
| `examples/demo/` | A small Cobra CLI that uses Localizer. |
| `site/` | The documentation site, published with GitHub Pages. |

## Security

See the [security model](https://locale.dev/security/). Report vulnerabilities privately as
described in [SECURITY.md](SECURITY.md).

## Contributing

Contributions are welcome. Before your first pull request can be merged, you'll be asked to sign the
[Contributor Assignment Agreement](CLA.md); a bot comments on the pull request with instructions. See
[CONTRIBUTING.md](CONTRIBUTING.md).

## License

Copyright (c) 2026 Snizyx Software LLC. Licensed under the
[University of Illinois/NCSA Open Source License](LICENSE).
