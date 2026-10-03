# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""rich-click hooks: its command classes render help themselves (bypassing Click's ``format_help``), list
subcommands through ``ctx.command``, and draw panels whose titles and labels come from a configuration
object; errors are panels too."""

import json

import pytest

rich_click = pytest.importorskip("rich_click")
from click.testing import CliRunner  # noqa: E402

import localizer  # noqa: E402
from localizer import _hooks_click  # noqa: E402

JA = {
    "Usage:": "使い方:",
    "Options": "オプション",
    "Commands": "コマンド",
    "Arguments": "引数",
    "Error": "エラー",
    "Aborted.": "中止しました。",
    "Show this message and exit.": "このメッセージを表示して終了します。",
    "[default: {}]": "[デフォルト: {}]",
    "[env var: {}]": "[環境変数: {}]",
    "Manage tasks.": "タスクを管理します。",
    "Add a task.": "タスクを追加します。",
    "Name of the thing.": "対象の名前。",
    "Priority of the task.": "タスクの優先度。",
    "Mark a task as done.": "タスクを完了にします。",
    "Advanced": "詳細設定",
    "No such command {name!r}.": "コマンド {name!r} はありません。",
    "Try '{command} {option}' for help.": "ヘルプは '{command} {option}' を実行してください。",
    "Missing argument": "引数がありません",
    "task {id} not found": "タスク {id} が見つかりません",
    "Continue?": "続行しますか?",
}


def make_cli():
    @rich_click.group(help="Manage tasks.")
    @rich_click.option("--name", help="Name of the thing.")
    def cli(name):
        pass

    @cli.command(help="Add a task.")
    @rich_click.argument("title")
    @rich_click.option("--priority", default="normal", show_default=True, envvar="PRIO", show_envvar=True, help="Priority of the task.")
    def add(title, priority):
        rich_click.echo(title)

    @cli.command()
    @rich_click.argument("id")
    def done(id):
        """Mark a task as done."""
        if id == "0":
            raise rich_click.Abort()
        if rich_click.confirm("Continue?"):
            raise rich_click.ClickException(f"task {id} not found")

    option_panel = getattr(rich_click, "option_panel", None)
    if option_panel is not None:
        option_panel("Advanced", options=["--name"])(cli)
    return cli


@pytest.fixture
def locales(tmp_path):
    (tmp_path / "ja.json").write_text(json.dumps({"version": 1, "language": "ja", "format": "python", "messages": JA}, ensure_ascii=False), encoding="utf-8")
    return tmp_path


@pytest.fixture(autouse=True)
def clean(monkeypatch):
    for var in ("LOCALIZER_LANG", "LOCALIZER_DEBUG", "LOCALIZER_DUMP", "LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE", "RICH_CLICK_THEME"):
        monkeypatch.delenv(var, raising=False)
    monkeypatch.setenv("COLUMNS", "120")
    monkeypatch.setenv("TERM", "dumb")
    monkeypatch.setenv("NO_COLOR", "1")
    monkeypatch.setattr(localizer._locale, "os_languages", lambda: [])
    yield
    localizer.uninstall()


def run(cli, *args, **kw):
    return CliRunner().invoke(cli, list(args), **kw)


def everything(result):
    """stdout and stderr of a run (Click 8.1 mixes them into ``output``)."""
    try:
        return result.output + result.stderr
    except ValueError:
        return result.output


def test_help_is_translated_and_originals_untouched(locales):
    cli = make_cli()
    english = run(cli, "--help").output
    assert "Usage:" in english and "Manage tasks." in english and "Add a task." in english
    assert localizer.localize(cli, locales, language="ja") == "ja"
    out = run(cli, "--help").output
    assert "使い方: cli [OPTIONS] COMMAND [ARGS]..." in out
    assert "タスクを管理します。" in out and "対象の名前。" in out
    assert "オプション" in out and "コマンド" in out
    assert "タスクを追加します。" in out and "タスクを完了にします。" in out  # subcommands, listed through ctx.command
    assert "このメッセージを表示して終了します。" in out
    if hasattr(rich_click, "option_panel"):
        assert "詳細設定" in out and "Advanced" not in out  # an application-defined panel title
    assert cli.help == "Manage tasks." and cli.params[0].help == "Name of the thing."
    sub = run(cli, "add", "--help").output
    assert "使い方: cli add [OPTIONS] TITLE" in sub
    assert "タスクを追加します。" in sub and "タスクの優先度。" in sub
    assert "[環境変数: PRIO]" in sub and "[デフォルト: normal]" in sub
    localizer.uninstall()
    assert run(cli, "--help").output == english


def test_english_run_is_byte_identical(locales, monkeypatch):
    cli = make_cli()
    before = [everything(run(cli, *a)) for a in (("--help",), ("add", "--help"), ("nope",), ("add",))]
    monkeypatch.setenv("LOCALIZER_LANG", "en")
    assert localizer.localize(cli, locales) == ""
    after = [everything(run(cli, *a)) for a in (("--help",), ("add", "--help"), ("nope",), ("add",))]
    assert before == after


def test_errors_prompts_and_abort(locales):
    cli = make_cli()
    localizer.localize(cli, locales, language="ja")
    r = run(cli, "nope")
    assert r.exit_code == 2
    text = everything(r)
    assert "エラー" in text and "コマンド 'nope' はありません。" in text
    assert "ヘルプは 'cli --help' を実行してください。" in text and "for help" not in text
    assert "使い方: cli [OPTIONS]" in text
    r = run(cli, "add")
    assert "引数がありません 'TITLE'." in everything(r)
    r = run(cli, "done", "9", input="y\n")
    assert r.exit_code == 1
    assert "続行しますか?" in r.output and "タスク 9 が見つかりません" in everything(r)
    r = run(cli, "done", "0")
    assert r.exit_code == 1 and "中止しました。" in everything(r) and "Aborted" not in everything(r)
    r = run(cli, "add", "Write docs")
    assert r.output.strip() == "Write docs"


def test_dump(locales, tmp_path, monkeypatch):
    dump = tmp_path / "dump.json"
    monkeypatch.setenv("LOCALIZER_DUMP", str(dump))
    cli = make_cli()
    localizer.localize(cli, locales, language="ja")
    run(cli, "--help")
    doc = json.loads(dump.read_text(encoding="utf-8"))
    entries = {(e["kind"], e["command"], e.get("flag", "")): e for e in doc["entries"]}
    assert doc["language"] == "ja"
    assert entries[("long", "cli", "")]["translated"] is True
    assert entries[("flag", "cli add", "priority")]["translated"] is True
    assert entries[("long", "cli done", "")]["text"] == "Mark a task as done."


def test_failed_translated_render_prints_help_once(locales, monkeypatch):
    """A view rich-click cannot render (a help text that is not a string) falls back to the original, and
    whatever the failed render wrote is discarded first."""
    cli = make_cli()
    localizer.localize(cli, locales, language="ja")
    monkeypatch.setattr(_hooks_click, "_help", lambda s: object() if s == "Manage tasks." else s)
    monkeypatch.setenv("LOCALIZER_DEBUG", "1")
    r = run(cli, "--help")
    assert r.exit_code == 0, everything(r)
    assert r.output.count("使い方:") == 1 and "Manage tasks." in r.output and "タスクを管理します。" not in r.output
    assert "localizer: rich_click help: " in everything(r)


def test_config_strings_are_restored_after_each_render(locales):
    cli = make_cli()
    localizer.localize(cli, locales, language="ja")
    ctx = cli.make_context("cli", [], resilient_parsing=True)
    out = ctx.get_help()
    assert "オプション" in out
    assert ctx.help_config.options_panel_title == "Options"  # translated only while a render runs
    assert rich_click.rich_click.OPTIONS_PANEL_TITLE == "Options"  # the globals are never touched
    assert localizer.localize(cli, locales, language="en") == ""
    out = ctx.get_help()
    assert "Options" in out and "オプション" not in out
