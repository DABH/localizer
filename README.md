<p align="center"><img src="brand/logo.svg" width="96" height="96" alt="Localizer logo"></p>
<h1 align="center">Localizer</h1>

Localizer renders a CLI's own strings (help text, flag descriptions, messages, errors, prompts) in the
user's language. Translations are produced by AI, kept in sync automatically through pull requests, and
shipped inside your binary or package. Localization adds no network calls and no noticeable startup cost.
Go with Cobra, kong or urfave/cli; Python with Typer, Click or argparse.

**Documentation: https://locale.dev/** · **Coding agents: [AGENTS.md](AGENTS.md)**

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

kong: `kong.Parse(&cli, kongx.Localize(locales.FS))` (package `github.com/DABH/localizer/kongx`).
urfave/cli v3: `urfave.Localize(cmd, locales.FS)` right before `cmd.Run` (package `github.com/DABH/localizer/urfave`).

```python
import localizer  # pip install localizer

localizer.localize(app, "yourcli.locales")  # ← the whole integration for Typer, Click and argparse CLIs
app()
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

A locales package holds one JSON catalog per language (`ja.json`, `de.json`, …), keyed by the exact
English source string: `locales/embed.go` embeds them into a Go binary, `yourcli/locales/__init__.py`
ships them inside a Python package. The hosted Localizer service keeps the catalogs up to date. On every
push that changes user-facing strings, it translates the new ones and opens or updates one pull request.
Connect it with:

- the **[Localizer GitHub Action](https://locale.dev/guides/github-action/)**. Your workflow
  authenticates with its short-lived GitHub OIDC token, so there's no API key, and Localizer never gets
  write access to your repository; or
- the **[Localizer GitHub App](https://locale.dev/guides/github-app/)**. Install it and merge
  the pull requests it opens.

The first pull request sets everything up, including the line above. The hosted service needs a
[subscription](https://locale.dev/pricing/); see [Getting started](https://locale.dev/getting-started/). A coding agent can do the integration
from [AGENTS.md](AGENTS.md) (also at https://locale.dev/AGENTS.md).

Catalogs are plain JSON that you can also write or edit by hand. Localizer keeps a translation you edited
as long as it is valid (same placeholders as the source, no new control characters); only an entry that
fails that check is replaced, and entries whose source string is gone are removed.

## What gets translated

With the one line above: every command's description, help and deprecation text, all flag and option
descriptions, help and usage headings, the framework's built-in commands and messages (Cobra's `help` and
`completion`, kong's `Show context-sensitive help.`, urfave/cli's `help` command, Click's and Typer's
`Show this message and exit.`, `[default: …]`, argparse's `positional arguments`, …), shell-completion
descriptions, prompts, and the errors the framework prints (unknown commands and flags, missing arguments,
invalid values, …).

Your own runtime messages go through a few helpers at your output chokepoints:

| Go | Python | Use |
| --- | --- | --- |
| `localizer.T(s)` | `localizer.t(s)` | Translate a string. For a format string, call it before formatting. |
| `localizer.Sprintf(format, args...)` | `localizer.tf(fmt, *args, **kwargs)` | Format with a translated format string. |
| `localizer.Error(err)` | `localizer.error(exc)` | Translate an error for display. Server text stays as it is. |
| `localizer.Translate(s, mode)` | `localizer.translate(text, mode)` | Translate already formatted text at a chokepoint. The mode (output, help or error) decides how composite text may be split. |
| `localizer.Writer(w)` | — | An `io.Writer` that translates what is written through it, one `Write` at a time. |

Only text that matches an entry in your catalog changes: the exact string, the formatted output of a
format string in the catalog (its values are carried over), or, in text of several lines, a paragraph or
line that does either. Everything else passes through byte for byte. Dynamic data (server responses, IDs,
JSON and YAML output) is therefore left alone, unless a line of it happens to equal a catalog entry or to
match one of its format strings; that line is translated too. Keep serialized output away from the helpers.

## Language selection

An app-specific variable (`localizer.WithEnvVar` / `env_var=`) → `LOCALIZER_LANG` → the first of `LC_ALL`,
`LC_MESSAGES` and `LANG` that has a value, with GNU `LANGUAGE` listing preferred languages ahead of it →
`LANGUAGE` alone → the OS setting (the macOS preferred languages, read from the user's preferences file;
the Windows display languages) → English. `C` and `POSIX` locales keep English, so scripts stay stable.
`LOCALIZER_LANG=off` disables localization. `LOCALIZER_LANG=qps` pseudo-localizes every known string, which
is handy for spotting strings that don't go through Localizer yet.

## Try it

```sh
# Go (Cobra; kong and urfave/cli demos live in examples/kong-demo and examples/urfave-demo)
go run ./examples/demo --help
LANG=ja_JP.UTF-8 go run ./examples/demo add --help
LOCALIZER_LANG=qps go run ./examples/demo --help

# Python (Typer); needs uv
cd python/examples/taskctl
LANG=ja_JP.UTF-8 uv run taskctl --help
LANG=de_DE.UTF-8 uv run taskctl done 9
```

## Repository layout

| Path | What |
| --- | --- |
| `/` | The Go runtime library and its Cobra integration: the only package a Cobra CLI imports. Dependencies: cobra, pflag, x/text, x/sys. |
| `kongx/`, `urfave/` | The kong and urfave/cli v3 integrations (each imports its framework; a CLI imports the one it uses). |
| `catalog/`, `engine/`, `msgfmt/` | Catalog format, lookup engine, and placeholder grammar (Go and Python), exported for tools. |
| `internal/` | Locale detection, and built-in translations of the supported frameworks' own strings (one directory per framework). |
| `python/` | The Python runtime, published to PyPI as `localizer`, with its tests and a Typer demo (`python/alias/` is a compatibility distribution). |
| `testdata/conformance/` | The shared specification and test vectors both runtimes must satisfy. |
| `action/` | The GitHub Action. |
| `examples/demo/`, `examples/kong-demo/`, `examples/urfave-demo/` | Small Cobra, kong and urfave/cli CLIs that use Localizer. |
| `site/` | The documentation site, published at https://locale.dev. |

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
