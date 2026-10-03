# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Typer hooks, on an app shaped like the Snowflake CLI: the command tree is rebuilt on every
invocation, plugin groups are attached to the live root by an eager option callback, help is Markdown,
a group filters its subcommands by comparing ``rich_help_panel`` values, and errors are f-strings."""

import json

import pytest

typer = pytest.importorskip("typer")
from typer.testing import CliRunner  # noqa: E402

import localizer  # noqa: E402
from localizer import _hooks_click  # noqa: E402

JA = {
    "Usage:": "使い方:",
    "Options": "オプション",
    "Commands": "コマンド",
    "Arguments": "引数",
    "Error": "エラー",
    "Show this message and exit.": "このメッセージを表示して終了します。",
    "[default: {}]": "[デフォルト: {}]",
    "[required]": "[必須]",
    "Try [blue]'{command_path} {help_option}'[/] for help.": "ヘルプは [blue]'{command_path} {help_option}'[/] をお試しください。",
    "No such command {name!r}.": "コマンド {name!r} はありません。",
    "Missing argument": "引数がありません",
    "Missing argument{param_hint}.{msg}": "引数がありません{param_hint}。{msg}",
    "Snowflake CLI tool for developers [v{__about__.VERSION}]": "開発者向け Snowflake CLI ツール [v{__about__.VERSION}]",
    "Manages connections.": "接続を管理します。",
    "Adds a connection.\n\nUse `snow connection test` afterwards.": "接続を追加します。\n\nその後 `snow connection test` を使用してください。",
    "Adds a connection.": "接続を追加します。",
    "Name of the connection.": "接続の名前。",
    "Connection configuration": "接続設定",
    "Native App commands": "Native App コマンド",
    "Deploys the app.": "アプリをデプロイします。",
    "Runs the app.": "アプリを実行します。",
    "Role {role} does not own the application {name}.": "ロール {role} はアプリケーション {name} を所有していません。",
    "Invalid connection configuration. {message}": "接続設定が無効です。{message}",
    "Origin url": "オリジン URL",
    "Are you sure?": "本当によろしいですか?",
}

VERSION = "0.0.0-test"


def make_app():
    click = _click()

    class CliError(click.ClickException):
        exit_code = 1

    class ConfigError(CliError):
        def format_message(self):
            return f"Invalid connection configuration. {self.message}"

    class SmartGroup(typer.core.TyperGroup):
        """Hides commands whose panel is not "Native App commands" (compares the real objects)."""

        def list_commands(self, ctx):
            return [n for n in super().list_commands(ctx) if self.get_command(ctx, n).rich_help_panel == "Native App commands"]

    ConnectionOption = typer.Option(None, "--connection", "-c", help="Name of the connection.", rich_help_panel="Connection configuration")

    def build_plugins():
        conn = typer.Typer(name="connection", help="Manages connections.", rich_markup_mode="markdown")

        @conn.command("add")
        def add(name: str = typer.Argument(..., help="Name of the connection."), connection: str = ConnectionOption):
            """Adds a connection.

            Use `snow connection test` afterwards.
            """
            if name == "bad":
                raise ConfigError(f"Role {'admin'} does not own the application {name}.")
            typer.echo(localizer.t(f"Adds a connection."))

        @conn.command("test", hidden=True)
        def test(connection: str = ConnectionOption):
            """Tests a connection."""

        app_group = typer.Typer(name="app", help="Manages apps.", rich_markup_mode="markdown", cls=SmartGroup)

        @app_group.command("deploy", rich_help_panel="Native App commands")
        def deploy():
            """Deploys the app."""

        @app_group.command("run", rich_help_panel="Snowflake App Runtime commands")
        def run():
            """Runs the app."""

        @app_group.command("prompt")
        def prompt():
            url = typer.prompt("Origin url")
            if typer.confirm("Are you sure?"):
                typer.echo(url)

        return [conn, app_group]

    # Like the Snowflake CLI: no default help option on the root (its own --help flag does nothing by
    # itself), plugins are attached by an eager option callback while the root's options are parsed, and
    # the callback body prints the root help, so it sees the full tree.
    root = typer.Typer(add_completion=False, add_help_option=False)

    def register(ctx: typer.Context, value: bool):
        if value and not ctx.resilient_parsing:
            group = ctx.command
            for plugin in build_plugins():
                group.add_command(typer.main.get_command(plugin))
        return value

    @root.callback(invoke_without_command=True, help=f"Snowflake CLI tool for developers [v{VERSION}]")
    def main(
        ctx: typer.Context,
        help: bool = typer.Option(False, "--help", "-h", is_eager=True, help="Show this message and exit."),
        registration: bool = typer.Option(True, "--commands-registration", hidden=True, is_eager=True, callback=register),
    ):
        if not ctx.invoked_subcommand:
            typer.echo(ctx.get_help())

    return root


def _click():
    try:
        return typer._click  # Typer >= 0.26 bundles Click
    except AttributeError:
        import click

        return click


@pytest.fixture
def locales(tmp_path):
    (tmp_path / "ja.json").write_text(json.dumps({"version": 1, "language": "ja", "format": "python", "messages": JA}, ensure_ascii=False), encoding="utf-8")
    return tmp_path


@pytest.fixture(autouse=True)
def clean(monkeypatch):
    for var in ("LOCALIZER_LANG", "LOCALIZER_DEBUG", "LOCALIZER_DUMP", "LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE"):
        monkeypatch.delenv(var, raising=False)
    monkeypatch.setenv("COLUMNS", "120")
    monkeypatch.setenv("TERM", "dumb")
    monkeypatch.setattr(localizer._locale, "os_languages", lambda: [])
    yield
    localizer.uninstall()


def run(app, *args, **kw):
    return CliRunner().invoke(app, list(args), **kw)


def everything(result):
    """stdout and stderr of a run (Click 8.1 mixes them into ``output``)."""
    try:
        return result.output + result.stderr
    except ValueError:
        return result.output


def test_lazy_tree_and_markdown_help(locales):
    app = make_app()
    english_root = run(app, "--help").output
    assert "Snowflake CLI tool for developers" in english_root and "connection" in english_root
    assert localizer.localize(app, locales, language="ja") == "ja"
    out = run(app, "--help").output
    assert "開発者向け Snowflake CLI ツール" in out  # Typer 0.26+ parses "[v…]" as markup
    assert "接続を管理します。" in out  # plugin group short help, registered mid-parse
    assert "使い方:" in out
    assert "コマンド" in out and "オプション" in out
    sub = run(app, "connection", "add", "--help").output
    assert "接続を追加します。" in sub and "snow connection test" in sub
    assert "接続の名前。" in sub
    assert "接続設定" in sub  # the shared option's panel title
    assert "引数" in sub
    assert "このメッセージを表示して終了します。" in sub
    assert "[必須]" in sub or "必須" in sub


def test_panel_filtering_uses_real_objects(locales):
    app = make_app()
    localizer.localize(app, locales, language="ja")
    out = run(app, "app", "--help").output
    assert "deploy" in out and "アプリをデプロイします。" in out
    assert "run" not in out.split("Native App")[0] if "Native App" in out else "run" not in out
    assert "Native App コマンド" in out


def test_errors_and_prompts(locales):
    app = make_app()
    localizer.localize(app, locales, language="ja")
    r = run(app, "nope")
    assert r.exit_code == 2
    assert "エラー" in r.output and "コマンド 'nope' はありません。" in r.output
    r = run(app, "connection", "add", "x", "--bogus")
    assert r.exit_code == 2
    assert "ヘルプは" in r.output and "connection add --help" in r.output  # the "Try ... for help." hint
    assert "使い方: root connection add" in r.output
    r = run(app, "connection", "add")
    assert "引数がありません" in r.output
    r = run(app, "connection", "add", "bad")
    assert r.exit_code == 1
    assert "接続設定が無効です。ロール admin はアプリケーション bad を所有していません。" in r.output
    r = run(app, "connection", "add", "prod")
    assert "接続を追加します。" in r.output
    r = run(app, "app", "prompt", input="https://x\ny\n")
    assert "オリジン URL" in r.output and "本当によろしいですか?" in r.output


def test_english_is_byte_identical(locales, monkeypatch):
    app = make_app()
    before = [run(app, *a).output for a in (("--help",), ("connection", "add", "--help"), ("nope",), ("connection", "add", "bad"))]
    monkeypatch.setenv("LOCALIZER_LANG", "en")
    localizer.localize(app, locales)
    after = [run(app, *a).output for a in (("--help",), ("connection", "add", "--help"), ("nope",), ("connection", "add", "bad"))]
    assert before == after


def test_cli_runner_without_localize_hooks_stays_english(locales):
    app = make_app()
    out = run(app, "--help").output
    assert "Snowflake CLI tool for developers" in out


def test_translation_that_breaks_rich_markup_falls_back_to_english(locales, monkeypatch):
    """A translation the catalog validation lets through but Rich rejects at render time (a reordered
    tag) must not crash --help: the original renders, and LOCALIZER_DEBUG says why."""
    app = typer.Typer(rich_markup_mode="rich")

    @app.command()
    def main(name: str = typer.Option("x", help="[bold]Name[/bold] of the thing.")):
        """[bold]Manage[/bold] tasks."""

    assert "Manage tasks." in run(app, "--help").output
    localizer.localize(app, locales, language="ja")
    broken = "[/bold]タスク[bold]を管理します。"
    monkeypatch.setattr(_hooks_click, "_help", lambda s: broken if s == "[bold]Manage[/bold] tasks." else s)
    monkeypatch.setenv("LOCALIZER_DEBUG", "1")
    r = run(app, "--help")
    assert r.exit_code == 0, everything(r)
    assert "Manage tasks." in r.output and "タスク" not in r.output
    assert "localizer: rich help: " in everything(r) and "MarkupError" in everything(r)


def test_prompt_messages_of_the_bundled_click(locales):
    """Typer's bundled Click has no gettext: the confirmation prompt and Click's own prompt errors
    come from the built-in catalog through the hooks (with real Click, through gettext)."""
    app = typer.Typer()

    @app.command()
    def main():
        token = typer.prompt("Token", confirmation_prompt=True)
        if typer.confirm("Are you sure?"):
            typer.echo(token)

    localizer.localize(app, locales, language="ja")
    r = run(app, input="a\nb\na\na\nmaybe\ny\n")
    assert r.exit_code == 0, everything(r)
    assert "確認のため再入力してください: " in r.output and "Repeat for confirmation" not in r.output
    assert "エラー: 入力された 2 つの値が一致しません。" in r.output
    assert "本当によろしいですか? [y/N]: " in r.output
    assert "エラー: 入力が無効です" in r.output and "invalid input" not in r.output
    assert r.output.rstrip().endswith("\na")


def test_english_after_a_language_leaves_typer_panels_english(locales):
    rich_utils = pytest.importorskip("typer.rich_utils")
    app = make_app()
    english = run(app, "connection", "add", "--help").output
    localizer.localize(app, locales, language="ja")
    assert "オプション" in run(app, "connection", "add", "--help").output
    assert rich_utils.OPTIONS_PANEL_TITLE == "Options"  # translated only while a render runs
    assert localizer.localize(app, locales, language="en") == ""
    assert run(app, "connection", "add", "--help").output == english
    assert rich_utils.OPTIONS_PANEL_TITLE == "Options"
