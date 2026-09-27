# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

import argparse
import io
import json
from contextlib import redirect_stderr

import pytest

import localizer

JA = {
    "usage: ": "使い方: ",
    "positional arguments": "位置引数",
    "options": "オプション",
    "show this help message and exit": "このヘルプを表示して終了します",
    "Manage tasks.": "タスクを管理します。",
    "See the docs for more.": "詳しくはドキュメントを参照してください。",
    "Title of the task.": "タスクの件名。",
    "Print more details.": "詳細を表示します。",
    "Add a task.": "タスクを追加します。",
    "the following arguments are required: %s": "次の引数が必要です: %s",
    "%(prog)s: error: %(message)s\n": "%(prog)s: エラー: %(message)s\n",
    "unrecognized arguments: %s": "不明な引数: %s",
    "Available commands": "利用可能なコマンド",
    " (default: %(default)s)": " (デフォルト: %(default)s)",
}


def make_parser():
    parser = argparse.ArgumentParser(prog="taskctl", description="Manage tasks.", epilog="See the docs for more.", formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument("--verbose", action="store_true", help="Print more details.")
    sub = parser.add_subparsers(title="Available commands", dest="command")
    add = sub.add_parser("add", help="Add a task.", description="Add a task.", formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    add.add_argument("title", help="Title of the task.")
    add.add_argument("--priority", default="normal", help="Priority of the task.")
    return parser


@pytest.fixture
def locales(tmp_path):
    (tmp_path / "ja.json").write_text(json.dumps({"version": 1, "language": "ja", "format": "python", "messages": JA}, ensure_ascii=False), encoding="utf-8")
    return tmp_path


@pytest.fixture(autouse=True)
def clean(monkeypatch):
    for var in ("LOCALIZER_LANG", "LOCALIZER_DEBUG", "LOCALIZER_DUMP", "LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE"):
        monkeypatch.delenv(var, raising=False)
    monkeypatch.setenv("COLUMNS", "100")
    monkeypatch.setattr(localizer._locale, "os_languages", lambda: [])
    yield
    localizer.uninstall()


def test_help(locales):
    parser = make_parser()
    english = parser.format_help()
    assert localizer.localize(parser, locales, language="ja") == "ja"
    out = parser.format_help()
    assert out.startswith("使い方: taskctl")
    assert "タスクを管理します。" in out and "詳しくはドキュメントを参照してください。" in out
    assert "詳細を表示します。" in out and "オプション:" in out and "このヘルプを表示して終了します" in out
    assert "利用可能なコマンド:" in out and "タスクを追加します。" in out
    assert parser.description == "Manage tasks."  # restored
    sub = [a for a in parser._actions if isinstance(a, argparse._SubParsersAction)][0].choices["add"]
    sub_out = sub.format_help()
    assert "タスクの件名。" in sub_out and "位置引数:" in sub_out
    assert "(デフォルト: normal)" in sub_out
    localizer.uninstall()
    assert parser.format_help() == english


def test_errors(locales):
    parser = make_parser()
    localizer.localize(parser, locales, language="ja")
    err = io.StringIO()
    with redirect_stderr(err), pytest.raises(SystemExit):
        parser.parse_args(["add"])
    assert "taskctl add: エラー: 次の引数が必要です: title" in err.getvalue()
    err = io.StringIO()
    with redirect_stderr(err), pytest.raises(SystemExit):
        parser.parse_args(["--bogus"])
    assert "不明な引数: --bogus" in err.getvalue()


def test_dump(locales, tmp_path, monkeypatch):
    dump = tmp_path / "dump.json"
    monkeypatch.setenv("LOCALIZER_DUMP", str(dump))
    parser = make_parser()
    localizer.localize(parser, locales, language="ja")
    doc = json.loads(dump.read_text(encoding="utf-8"))
    kinds = {(e["kind"], e["command"], e.get("flag", "")): e["translated"] for e in doc["entries"]}
    assert kinds[("long", "taskctl", "")] is True
    assert kinds[("flag", "taskctl add", "--priority")] is False
    assert kinds[("flag", "taskctl add", "title")] is True
