# localizer

Render a Typer, Click or argparse CLI's own strings — help text, option descriptions, messages,
errors, prompts — in the user's language. Translations come from JSON catalogs committed to your
repository and shipped inside your package; nothing is downloaded or executed at runtime, and a CLI
without a matching catalog behaves exactly as before.

The catalogs are written by [Localizer](https://locale.dev/): install the GitHub App or add the
GitHub Action to your repository and it keeps a pull request up to date with translations of every
string it finds in your code. This package is the runtime half — the part that runs on your users'
machines.

```sh
pip install localizer
```

Python 3.10 or newer, no dependencies. Works with Click 8.1+, Typer 0.17+ (including Typer's
bundled Click: help, errors, the prompt text and Click's own prompt messages) and the standard
library's argparse.

## One line

Call `localize` after every command and option is registered, right before the app runs:

```python
import localizer

# Typer
app = typer.Typer()
...
if __name__ == "__main__":
    localizer.localize(app, "yourcli.locales")
    app()

# Click
@click.group()
def cli(): ...

def main():
    localizer.localize(cli, "yourcli.locales")
    cli()

# argparse
parser = argparse.ArgumentParser(prog="yourcli")
...
localizer.localize(parser, "yourcli.locales")
args = parser.parse_args()
```

`"yourcli.locales"` names the package holding your catalogs (`yourcli/locales/ja.json`, …); a
directory path or an `importlib.resources` Traversable works too. If your CLI builds its command
tree in a factory, call `localize` where the finished app object is handed out — translation happens
when help and errors are rendered, so commands registered later (plugins, lazy groups) are covered.

What is translated: your help text and option descriptions, the framework's own messages (`Usage:`,
`Show this message and exit.`, `Missing argument 'NAME'.`, `Aborted!`, `Repeat for confirmation` and
Click's other prompt messages, argparse's `error:` lines — built-in catalogs for these ship in this
package), the prompt text of `prompt`/`confirm`, the messages of exceptions your CLI raises, and
whatever your program passes through the helpers below. Nothing else changes: command and
option names, values, JSON/YAML output and logs stay as they are.

## Your own messages

```python
from localizer import t, tf, error

print(t("Nothing to do."))                          # exact lookup, or the English text
print(tf("Added task {n}: {title!r}", n=3, title=s))  # translated format string, then .format()
console.print(t(f"Deleted {count} files"))          # formatted text is matched against the catalog
raise click.ClickException(error(exc))              # an exception's message, for display
```

`t` on a format string looks the template up exactly (call it before formatting, or use `tf`); on
finished text it reverse-matches the catalog's templates, so `f"Deleted {count} files"` finds
`Deleted {count} files` and keeps the number. `localizer.translate(text, localizer.Mode.ERROR)`
does the same for chokepoints that receive already-formatted messages. Every helper returns its input
unchanged when there is no translation and never raises.

## Catalogs

One file per language, named by BCP 47 tag, keyed by the exact English source string:

```json
{
  "version": 1,
  "language": "ja",
  "format": "python",
  "messages": {
    "Add a task.": "タスクを追加します。",
    "Added task {n}: {title!r}": "タスク {n} を追加しました: {title!r}"
  }
}
```

Localizer generates and maintains these; you review them like any other pull request. Make sure the
JSON files are packaged: hatchling, poetry, flit, pdm and uv include them automatically, setuptools
needs `[tool.setuptools.package-data] yourcli = ["locales/*.json"]`.

## Language selection

`LOCALIZER_LANG`, then `LC_ALL`, `LC_MESSAGES`, `LANG` (and `LANGUAGE`), then the operating system's
preferred languages on macOS and Windows; the best available catalog wins, English is the default.
`localize(app, ..., env_var="YOURCLI_LANG")` adds an application-specific override checked first;
`language="de"` forces a language. Setting the variable to `off` keeps a CLI in English;
`LOCALIZER_LANG=qps` pseudo-localizes every translatable string (`⟦Ûšáĝé:⟧`) so you can see what is
covered without a catalog. `LOCALIZER_DEBUG=1` reports untranslated strings and swallowed hook errors
on stderr; `LOCALIZER_DUMP=path.json` writes the help tree with a translated/untranslated flag per
entry.

Pin your tests to English so snapshots don't depend on the machine's locale:

```python
# conftest.py
import os

def pytest_configure(config):
    os.environ.setdefault("LOCALIZER_LANG", "en")
```

## More

- Documentation: https://locale.dev/ — [runtime reference](https://locale.dev/reference/runtime/),
  [integration guide](https://locale.dev/guides/integration/)
- Coding agents: point yours at https://locale.dev/AGENTS.md to integrate Localizer into a CLI
- Source: https://github.com/DABH/localizer (`python/`); license: NCSA
