# taskctl (Typer demo)

The Python counterpart of `examples/demo`: a small Typer CLI localized with one line,
`localizer.localize(app, "taskctl.locales")`, and catalogs shipped inside the package.

```sh
cd python/examples/taskctl        # uv installs the runtime from this checkout (see pyproject.toml)
uv run taskctl --help
LANG=ja_JP.UTF-8 uv run taskctl add --help
LANG=de_DE.UTF-8 uv run taskctl done 9
LOCALIZER_LANG=qps uv run taskctl --help
LOCALIZER_DEBUG=1 LANG=fr_FR.UTF-8 uv run taskctl add "Write docs"
```

`taskctl list -o json` prints the same JSON in every language: only the CLI's own strings change.
