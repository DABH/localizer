# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Modules a framework imports only when it renders (Typer's rich_utils, Click's editor, pager and
completion helpers) are hooked when that import happens, never imported by localize() itself."""

import json
import os
import subprocess
import sys

import pytest

import localizer
from localizer import _hooks

JA = {
    "Usage:": "使い方:",
    "Options": "オプション",
    "Commands": "コマンド",
    "Error": "エラー",
    "Manage tasks.": "タスクを管理します。",
    "Add a task.": "タスクを追加します。",
    "Name of the thing.": "対象の名前。",
    "No such command {name!r}.": "コマンド {name!r} はありません。",
}


@pytest.fixture(autouse=True)
def clean(monkeypatch):
    for var in ("LOCALIZER_LANG", "LOCALIZER_DEBUG", "LOCALIZER_DUMP", "LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE"):
        monkeypatch.delenv(var, raising=False)
    monkeypatch.setattr(localizer._locale, "os_languages", lambda: [])
    yield
    localizer.uninstall()
    _hooks.uninstall()


def _hook_installed() -> bool:
    return any(type(f).__name__ == "_ImportHook" for f in sys.meta_path)


def test_on_import_runs_when_the_module_is_imported(tmp_path, monkeypatch):
    (tmp_path / "lazy_mod_for_localizer.py").write_text("VALUE = 1\n", encoding="utf-8")
    monkeypatch.syspath_prepend(str(tmp_path))
    monkeypatch.delitem(sys.modules, "lazy_mod_for_localizer", raising=False)
    seen = []
    _hooks.on_import("lazy_mod_for_localizer", lambda m: seen.append(m.VALUE))
    assert seen == [] and "lazy_mod_for_localizer" not in sys.modules and _hook_installed()
    import lazy_mod_for_localizer as mod

    assert seen == [1]
    assert mod.__spec__.loader.get_source("lazy_mod_for_localizer") == "VALUE = 1\n"  # the real loader stays reachable
    _hooks.on_import("lazy_mod_for_localizer", lambda m: seen.append(2))  # imported already: runs at once
    assert seen == [1, 2]
    _hooks.uninstall()
    assert not _hook_installed()
    del sys.modules["lazy_mod_for_localizer"]
    import lazy_mod_for_localizer  # noqa: F401

    assert seen == [1, 2]


def test_failing_callback_does_not_break_the_import(tmp_path, monkeypatch, capsys):
    (tmp_path / "lazy_mod_for_localizer2.py").write_text("VALUE = 2\n", encoding="utf-8")
    monkeypatch.syspath_prepend(str(tmp_path))
    monkeypatch.delitem(sys.modules, "lazy_mod_for_localizer2", raising=False)
    monkeypatch.setenv("LOCALIZER_DEBUG", "1")
    _hooks.on_import("lazy_mod_for_localizer2", lambda m: 1 / 0)
    import lazy_mod_for_localizer2 as mod

    assert mod.VALUE == 2
    assert "localizer: hooking lazy_mod_for_localizer2: ZeroDivisionError" in capsys.readouterr().err


def run(tmp_path, script):
    locales = tmp_path / "locales"
    locales.mkdir(exist_ok=True)
    (locales / "ja.json").write_text(json.dumps({"version": 1, "language": "ja", "format": "python", "messages": JA}, ensure_ascii=False), encoding="utf-8")
    path = tmp_path / "app.py"
    path.write_text(script.replace("LOCALES", repr(str(locales))), encoding="utf-8")
    env = {k: v for k, v in os.environ.items() if not k.startswith(("LOCALIZER_", "LC_")) and k not in ("LANG", "LANGUAGE")}
    env.update(COLUMNS="120", TERM="dumb", NO_COLOR="1", PYTHONIOENCODING="utf-8")
    r = subprocess.run([sys.executable, str(path)], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=120)
    assert r.returncode == 0, r.stderr.decode("utf-8", "replace")
    return json.loads(r.stdout.decode("utf-8"))


TYPER = """
import json, sys
import typer, localizer
from typer.testing import CliRunner
app = typer.Typer(help="Manage tasks.")
@app.command()
def add(name: str = typer.Option("x", help="Name of the thing.")):
    \"\"\"Add a task.\"\"\"
@app.command()
def done(): pass
before = set(sys.modules)
assert localizer.localize(app, LOCALES, language="ja") == "ja"
new = sorted(set(sys.modules) - before)
heavy = [m for m in new if m.split(".")[0] in ("rich", "markdown_it", "pygments") or m == "typer.rich_utils"]
help = CliRunner().invoke(app, ["--help"]).output
error = CliRunner().invoke(app, ["nope"])
try:
    err = error.output + error.stderr
except ValueError:  # Click 8.1 mixes stderr into output
    err = error.output
print(json.dumps({"before": "typer.rich_utils" in before, "heavy": heavy, "help": help, "error": err, "after": "typer.rich_utils" in sys.modules}))
"""


def test_typer_rich_utils_is_hooked_when_typer_imports_it(tmp_path):
    pytest.importorskip("typer")
    d = run(tmp_path, TYPER)
    assert d["before"] is False and d["heavy"] == [], d["heavy"]  # localize() imported none of it
    assert d["after"] is True  # rendering help did, and the hook was applied on the way
    assert "タスクを管理します。" in d["help"] and "オプション" in d["help"] and "コマンド" in d["help"] and "タスクを追加します。" in d["help"]
    assert "エラー" in d["error"] and "コマンド 'nope' はありません。" in d["error"]


CLICK = """
import gettext, json, sys
import click, localizer
@click.group(help="Manage tasks.")
def cli(): pass
before = set(sys.modules)
assert localizer.localize(cli, LOCALES, language="ja") == "ja"
new = sorted(set(sys.modules) - before)
lazy = [m for m in ("click._termui_impl", "click.shell_completion", "click._winconsole", "ctypes") if m in new]
import click._termui_impl as impl
import click.shell_completion as completion
print(json.dumps({"imported_by_localize": lazy, "impl": impl._("Usage:"), "completion": completion._("Usage:"), "plain": impl._ is gettext.gettext}))
"""


def test_click_helpers_are_hooked_when_click_imports_them(tmp_path):
    pytest.importorskip("click")
    d = run(tmp_path, CLICK)
    assert d["imported_by_localize"] == []
    assert d["impl"] == "使い方:" and d["completion"] == "使い方:" and d["plain"] is False
