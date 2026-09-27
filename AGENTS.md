# Localizer

Localizer renders a command-line tool's own strings — help text, option descriptions, messages, errors,
prompts — in the user's language. Translations are JSON catalogs kept in the CLI's repository: a hosted
service (GitHub App or GitHub Action) writes and maintains them through pull requests, and a small runtime
library (Go: `github.com/DABH/localizer`; Python: `localizer-py`) applies them when the CLI runs. Nothing is
downloaded at runtime, and English output is unchanged.

This file has two audiences: agents integrating Localizer into **another** repository (next section), and
agents working on **this** repository (last section). Documentation: https://locale.dev/ — index of pages
for agents: https://locale.dev/llms.txt.

## Integrating Localizer into a CLI (any repository)

Follow the steps in order. Never hand-write translations: the service produces them once the repository is
connected, and human edits to its catalogs are preserved.

### 1. Identify the language and framework

- **Go + Cobra** (`github.com/spf13/cobra` in `go.mod`): one line, `localizer.Localize(root, locales.FS)`.
- **Python + Typer, Click or argparse** (`pyproject.toml`, `setup.py` or `setup.cfg`): one line,
  `localizer.localize(app, "yourcli.locales")`.
- Other Go or Python CLIs: `localizer.Init(locales.FS)` / `localizer.init("yourcli.locales")` plus the
  helpers of step 5.
- Other languages are not supported yet: stop and say so.

For Python, find the importable package that holds the CLI (`src/yourcli/…` or `yourcli/…`) and the build
backend (`[build-system]` in `pyproject.toml`: hatchling, poetry, flit, pdm, uv or setuptools).

### 2. Add the dependency

- Go: `go get github.com/DABH/localizer@latest && go mod tidy`.
- Python: add `localizer-py` — **not** `localizer`, an unrelated PyPI project — with the project's own tool
  so lock files stay consistent: `uv add localizer-py`, `poetry add localizer-py`, or add it to
  `[project] dependencies` in `pyproject.toml` and regenerate `uv.lock` / `poetry.lock` /
  `requirements*.txt`. The import name is `localizer`.

### 3. Create the locales package with empty catalogs

Go — `locales/embed.go` (import path `<module>/locales`):

```go
// Package locales holds translation catalogs maintained by Localizer (https://locale.dev).
package locales

import "embed"

// FS contains one "<language>.json" catalog per language.
//
//go:embed *.json
var FS embed.FS
```

Python — `<package>/locales/__init__.py` with a one-line docstring (copy the project's license header if
its modules have one). With setuptools, add `[tool.setuptools.package-data] yourcli = ["locales/*.json"]`
to `pyproject.toml`; hatchling, poetry, flit, pdm and uv include the JSON files automatically.

Both — one empty catalog per language you will configure in step 8, e.g. `locales/ja.json`:

```json
{"version": 1, "language": "ja", "messages": {}}
```

(for Python add `"format": "python"`). `go:embed` needs at least one matching file; the service fills the
catalogs in.

### 4. Add the one line

On the production path only, after every command and option is registered, right before the app runs.
Never in tests, documentation generators or linters that build the command tree directly.

```go
import (
	"github.com/DABH/localizer"
	"example.com/yourcli/locales"
)

func main() {
	root := newRootCmd()
	localizer.Localize(root, locales.FS) // options: localizer.WithEnvVar("YOURCLI_LANG"), localizer.WithLanguage(tag)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
```

```python
import localizer

localizer.localize(app, "yourcli.locales", env_var="YOURCLI_LANG")  # Typer: right before app()
app()

localizer.localize(cli, "yourcli.locales")     # Click: right before cli()
cli()

localizer.localize(parser, "yourcli.locales")  # argparse: right before parse_args()
args = parser.parse_args()
```

When the app is built by a factory (a function or class that returns it), call `localize` where the
finished object is handed to the code that runs it (right before `return app` or `self._app = app`).
Commands registered later — plugins, lazy groups, eager option callbacks — are still covered, because
translation happens when help and errors render.

### 5. Route the CLI's own messages through the helpers

| | Go | Python |
| --- | --- | --- |
| A string | `localizer.T(s)` | `localizer.t(s)` |
| A format string | `localizer.Sprintf(format, args...)`, `localizer.Errorf` | `localizer.tf(fmt, *args, **kwargs)` |
| An error for display | `localizer.Error(err)` | `localizer.error(exc)` |
| A stream / formatted text | `localizer.Writer(w)` | `localizer.translate(text, localizer.Mode.ERROR)` |

Hook chokepoints, not call sites: the CLI's printing helpers (console wrappers, `echo` helpers), its
error display, prompts, table headers. Call `T`/`t` on the format string **before** formatting (or use
`Sprintf`/`tf`), or on the finished text (reverse matching restores placeholders). Never pass serialized
output (JSON, YAML, CSV), paths, identifiers or data from servers through them; logs stay in English.

### 6. Pin tests to English

- Go, subprocess tests: `cmd.Env = append(os.Environ(), "LOCALIZER_LANG=en")`. In-process tests that build
  the tree without `main` are unaffected.
- Python, root `conftest.py`:

  ```python
  import os

  def pytest_configure(config):
      os.environ.setdefault("LOCALIZER_LANG", "en")
  ```

### 7. Verify

- `LOCALIZER_LANG=qps <cli> --help` — every translatable string shows as `⟦ţéxţ⟧`; plain English left
  means that string doesn't go through Localizer yet (hook it in step 5 or leave it if it's data).
- `LOCALIZER_LANG=qps <cli> no-such-command` — the framework's own error is pseudo-localized too.
- `LOCALIZER_LANG=en <cli> --help` — byte-identical to before the change.
- The test suite passes.

### 8. Commit the configuration and connect the repository

`.localizer.yml` at the repository root (every key is optional):

```yaml
language: python                 # or go; detected from go.mod / pyproject.toml when omitted
languages: [ja, zh-Hans, ko, es, fr, de, pt-BR]
locales_dir: src/yourcli/locales # Go default: locales
glossary:
  keep: [YourProduct]            # never translated
extract:
  exclude: ["scripts/**"]
  funcs:                         # your own output helpers -> which arguments are user-facing
    "*.step": [0]                # Python: .step(msg) on any receiver
    yourcli.output.Result: [0, message]
    # example.com/yourcli/pkg/output.Printf: [1]   # Go
```

Then connect it. Either add `.github/workflows/localizer.yml`:

```yaml
name: Localizer
on:
  push:
    branches: [main]
  workflow_dispatch:
permissions:
  contents: write
  pull-requests: write
  id-token: write
jobs:
  translations:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: DABH/localizer/action@v0.4.1
```

(and tell the user to enable **Settings → Actions → General → Allow GitHub Actions to create and approve
pull requests**), or tell the user to install the GitHub App: https://locale.dev/guides/github-app/. The
service accepts public repositories only. The first run opens a pull request with translated catalogs;
later pushes update it.

### 9. What not to do

- Don't translate command names, option names, environment variable names, formats or code samples.
- Don't write or edit catalog entries yourself except to correct a translation.
- Don't change strings the code compares against (`rich_help_panel` values, Cobra `Use` lines, error
  strings other code inspects).
- Don't call `Localize`/`localize` from library code imported by other programs; it belongs to the entry
  point.
- Don't leave the test suite unpinned: CI machines and contributors have their own locales.

## Working on this repository

- Layout: the Go runtime at the root (`localizer.go`, `catalog/`, `engine/`, `msgfmt/`, `internal/`), the
  Python runtime in `python/` (`localizer/`, `tests/`, `examples/taskctl/`; published as `localizer-py`),
  the GitHub Action in `action/`, the documentation site in `site/` (Astro Starlight, deployed to
  https://locale.dev), and the shared conformance corpus in `testdata/conformance/` (its README is the
  specification of the placeholder grammar and the engine; both runtimes run the same vectors).
- Commands: `make` runs gofmt (check), `go vet`, the Go tests and the Python tests (needs `uv`);
  `make python`; `cd python && uv run --with pytest --with typer --with click pytest -q`; `make site`.
  Regenerate the locale-matching golden with `go test ./internal/locale -run Golden -update`.
- Conventions: new files carry `Copyright (c) 2026 Snizyx Software LLC. All rights reserved.` and
  `SPDX-License-Identifier: NCSA` headers; contributors sign the CLA (a bot asks on the pull request); a
  behavior change to the grammar or the engine is made in the corpus first, then in both runtimes; the
  repository's `vX.Y.Z` tags release the Go module and the PyPI package together.
- The hosted service lives in a separate, private repository. Never copy files from it into this one, and
  never commit credentials or private data here.
