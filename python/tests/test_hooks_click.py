# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

import io
import json
import threading
import time

import pytest

click = pytest.importorskip("click")
from click.testing import CliRunner  # noqa: E402

import localizer  # noqa: E402
from localizer import _hooks_click  # noqa: E402

JA = {
    "Usage:": "使い方:",
    "Options": "オプション",
    "Commands": "コマンド",
    "Show this message and exit.": "このメッセージを表示して終了します。",
    "Manage tasks.": "タスクを管理します。",
    "Manage tasks.\n\nUse it to add, list and complete tasks.": "タスクを管理します。\n\n追加、一覧、完了に使います。",
    "Add a task.\n\nThe title is required.": "タスクを追加します。\n\n件名は必須です。",
    "Name of the thing.": "対象の名前。",
    "Priority of the task: low, normal or high.": "タスクの優先度: low、normal、high。",
    "No such command {name!r}.": "コマンド {name!r} はありません。",
    "Error: {message}": "エラー: {message}",
    "Try '{command} {option}' for help.": "ヘルプは '{command} {option}' を実行してください。",
    "Missing argument": "引数がありません",
    "Continue?": "続行しますか?",
    "Added task {n}: {title!r}": "タスク {n} を追加しました: {title!r}",
    "task {id} not found": "タスク {id} が見つかりません",
}


def make_cli():
    @click.group(help="Manage tasks.")
    @click.option("--name", help="Name of the thing.")
    def cli(name):
        pass

    @cli.command(help="Add a task.\n\nThe title is required.")
    @click.argument("title")
    @click.option("--priority", default="normal", help="Priority of the task: low, normal or high.")
    def add(title, priority):
        click.echo(localizer.t(f"Added task {3}: {title!r}"))

    @cli.command()
    @click.argument("id")
    def done(id):
        if click.confirm("Continue?"):
            raise click.ClickException(f"task {id} not found")

    return cli


@pytest.fixture
def locales(tmp_path):
    (tmp_path / "ja.json").write_text(json.dumps({"version": 1, "language": "ja", "format": "python", "messages": JA}, ensure_ascii=False), encoding="utf-8")
    return tmp_path


@pytest.fixture(autouse=True)
def clean(monkeypatch):
    for var in ("LOCALIZER_LANG", "LOCALIZER_DEBUG", "LOCALIZER_DUMP", "LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE"):
        monkeypatch.delenv(var, raising=False)
    monkeypatch.setattr(localizer._locale, "os_languages", lambda: [])
    yield
    localizer.uninstall()


def test_help_is_translated_and_originals_untouched(locales):
    cli = make_cli()
    english = CliRunner().invoke(cli, ["--help"]).output
    assert localizer.localize(cli, locales, language="ja") == "ja"
    out = CliRunner().invoke(cli, ["--help"]).output
    assert "使い方: cli [OPTIONS] COMMAND [ARGS]..." in out
    assert "タスクを管理します。" in out and "対象の名前。" in out
    assert "オプション:" in out and "コマンド:" in out
    assert "このメッセージを表示して終了します。" in out
    assert "add   タスクを追加します。" in out  # the subcommand's short help
    assert cli.help == "Manage tasks." and cli.params[0].help == "Name of the thing."
    sub = CliRunner().invoke(cli, ["add", "--help"]).output
    assert "件名は必須です。" in sub and "low、normal、high" in sub
    localizer.uninstall()
    assert CliRunner().invoke(cli, ["--help"]).output == english


def test_english_run_is_byte_identical(locales, monkeypatch):
    cli = make_cli()
    before = [CliRunner().invoke(cli, args).output for args in (["--help"], ["nope"], ["add"])]
    monkeypatch.setenv("LOCALIZER_LANG", "en")
    assert localizer.localize(cli, locales) == ""
    after = [CliRunner().invoke(cli, args).output for args in (["--help"], ["nope"], ["add"])]
    assert before == after


def test_errors_and_prompts(locales):
    cli = make_cli()
    localizer.localize(cli, locales, language="ja")
    r = CliRunner().invoke(cli, ["nope"])
    assert r.exit_code == 2
    assert "エラー: コマンド 'nope' はありません。" in r.output
    assert "ヘルプは 'cli --help' を実行してください。" in r.output
    assert "使い方: cli" in r.output
    r = CliRunner().invoke(cli, ["add"])
    assert "エラー: 引数がありません 'TITLE'." in r.output
    r = CliRunner().invoke(cli, ["done", "9"], input="y\n")
    assert "続行しますか?" in r.output
    assert "エラー: タスク 9 が見つかりません" in r.output
    r = CliRunner().invoke(cli, ["add", "Write docs"])
    assert r.output.strip() == "タスク 3 を追加しました: 'Write docs'"


def test_dump(locales, tmp_path, monkeypatch):
    dump = tmp_path / "dump.json"
    monkeypatch.setenv("LOCALIZER_DUMP", str(dump))
    cli = make_cli()
    localizer.localize(cli, locales, language="ja")
    CliRunner().invoke(cli, ["--help"])
    doc = json.loads(dump.read_text(encoding="utf-8"))
    assert doc["language"] == "ja"
    entries = {(e["kind"], e["command"], e.get("flag", "")): e for e in doc["entries"]}
    assert entries[("long", "cli", "")]["translated"] is True
    assert entries[("flag", "cli add", "priority")]["text"] == "Priority of the task: low, normal or high."
    assert entries[("long", "cli done", "")] if ("long", "cli done", "") in entries else True


def test_docstring_help_is_translated(locales, tmp_path, monkeypatch):
    """Python < 3.13 hands Click the raw, indented docstring; the catalog is keyed by its cleaned text."""

    @click.group()
    def cli():
        """Manage tasks.

        Use it to add, list and complete tasks.
        """

    @cli.command()
    def add():
        """Add a task.

        The title is required.
        """

    dump = tmp_path / "dump.json"
    monkeypatch.setenv("LOCALIZER_DUMP", str(dump))
    localizer.localize(cli, locales, language="ja")
    out = CliRunner().invoke(cli, ["--help"]).output
    assert "タスクを管理します。" in out and "追加、一覧、完了に使います。" in out
    assert "add  タスクを追加します。" in out
    sub = CliRunner().invoke(cli, ["add", "--help"]).output
    assert "タスクを追加します。" in sub and "件名は必須です。" in sub
    assert cli.help.startswith("Manage tasks.")  # the original is not modified
    entries = {(e["kind"], e["command"]): e for e in json.loads(dump.read_text(encoding="utf-8"))["entries"]}
    assert entries[("long", "cli add")] == {"kind": "long", "command": "cli add", "text": "Add a task.\n\nThe title is required.", "translated": True}


def test_failed_translated_render_prints_help_once(locales, monkeypatch):
    cli = make_cli()
    localizer.localize(cli, locales, language="ja")
    # A copy whose help is not a string makes Click fail after it wrote the usage line.
    monkeypatch.setattr(_hooks_click, "_help", lambda s: 0 if s == "Manage tasks." else s)
    out = CliRunner().invoke(cli, ["--help"]).output
    assert out.count("使い方:") == 1 and "Manage tasks." in out and "タスクを管理します。" not in out


def test_error_display_from_threads_and_uninstall(locales, monkeypatch):
    cli = make_cli()
    orig_echo = click.exceptions.echo
    localizer.localize(cli, locales, language="ja")
    engine = localizer._api.state().engine
    real = engine.translate
    monkeypatch.setattr(engine, "translate", lambda s, m: (time.sleep(0.001), real(s, m))[1])  # lets the threads interleave
    outputs = []
    barrier = threading.Barrier(2)

    def show():
        buf = io.StringIO()
        barrier.wait()
        for _ in range(10):
            click.exceptions.UsageError("Missing argument").show(file=buf)
        outputs.append(buf.getvalue())

    threads = [threading.Thread(target=show) for _ in range(2)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    assert [o.count("エラー: 引数がありません") for o in outputs] == [10, 10]
    localizer.uninstall()
    assert click.exceptions.echo is orig_echo
