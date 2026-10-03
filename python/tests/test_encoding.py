# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Output stays in English when a standard stream cannot encode the catalog: on Windows, redirected output
uses the ANSI code page unless PYTHONUTF8=1, and a translated help line would raise UnicodeEncodeError (or
print escapes where the error handler allows it). pytest's capture and the CliRunner are UTF-8, so the
end-to-end checks run the CLI in a subprocess with PYTHONIOENCODING set and stdout on a pipe."""

import io
import json
import os
import subprocess
import sys

import pytest

import localizer
from localizer import _encoding

JA = {"Manage tasks.": "タスクを管理します。", "Print more details.": "詳細を表示します。", "Usage:": "使い方:", "usage: ": "使い方: "}

ARGPARSE = """
import argparse, localizer
p = argparse.ArgumentParser(prog="taskctl", description="Manage tasks.")
p.add_argument("--verbose", action="store_true", help="Print more details.")
localizer.localize(p, LOCALES)
p.parse_args()
"""
CLICK = """
import click, localizer
@click.group(help="Manage tasks.")
def cli(): pass
@cli.command(help="Print more details.")
def add(): pass
localizer.localize(cli, LOCALES)
cli()
"""
TYPER = """
import typer, localizer
app = typer.Typer(help="Manage tasks.")
@app.command()
def add():
    \"\"\"Print more details.\"\"\"
@app.command()
def done(): pass
localizer.localize(app, LOCALES)
app()
"""


def run(tmp_path, script, args, **env_overrides):
    locales = tmp_path / "locales"
    locales.mkdir(exist_ok=True)
    (locales / "ja.json").write_text(json.dumps({"version": 1, "language": "ja", "format": "python", "messages": JA}, ensure_ascii=False), encoding="utf-8")
    path = tmp_path / "app.py"
    path.write_text(script.replace("LOCALES", repr(str(locales))), encoding="utf-8")
    env = {k: v for k, v in os.environ.items() if not k.startswith(("LOCALIZER_", "LC_")) and k not in ("LANG", "LANGUAGE", "PYTHONIOENCODING", "PYTHONUTF8", "PYTHONLEGACYWINDOWSSTDIO")}
    env.update(LANG="ja_JP.UTF-8", COLUMNS="100", TERM="dumb", NO_COLOR="1", **env_overrides)
    r = subprocess.run([sys.executable, str(path), *args], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=120)
    return r.returncode, r.stdout.decode("utf-8", "replace"), r.stderr.decode("utf-8", "replace")


@pytest.mark.parametrize("framework,script", [("argparse", ARGPARSE), ("click", CLICK), ("typer", TYPER)])
def test_unencodable_stdout_stays_english(tmp_path, framework, script):
    if framework != "argparse":
        pytest.importorskip(framework)
    code, out, err = run(tmp_path, script, ["--help"], PYTHONIOENCODING="cp1252", LOCALIZER_DEBUG="1")
    assert code == 0, err
    assert "Manage tasks." in out and "タスク" not in out
    assert "UnicodeEncodeError" not in err
    assert "localizer: stdout (cp1252) cannot encode the ja catalog; output stays in English" in err


def test_utf8_stdout_is_translated(tmp_path):
    """The control: the same program with a UTF-8 stream shows the catalog."""
    code, out, err = run(tmp_path, ARGPARSE, ["--help"], PYTHONIOENCODING="utf-8")
    assert code == 0, err
    assert out.startswith("使い方: taskctl") and "タスクを管理します。" in out


def test_lossy_error_handler_stays_english_too(tmp_path):
    """stderr's default handler is backslashreplace: with it, a cp1252 stream prints \\uXXXX escapes rather
    than raising, which is no better than English."""
    code, out, err = run(tmp_path, ARGPARSE, ["--bogus"], PYTHONIOENCODING="cp1252:backslashreplace")
    assert code == 2
    assert "taskctl: error: unrecognized arguments: --bogus" in err and "\\u" not in err


def test_pseudo_localization_needs_an_encodable_stream(tmp_path):
    code, out, err = run(tmp_path, ARGPARSE, ["--help"], PYTHONIOENCODING="cp1252", LOCALIZER_LANG="qps", LOCALIZER_DEBUG="1")
    assert code == 0, err
    assert out.startswith("usage: taskctl") and "⟦" not in out
    assert "stdout (cp1252) cannot encode the qps catalog" in err


class Stream(io.StringIO):
    def __init__(self, encoding):
        super().__init__()
        self._encoding = encoding

    @property
    def encoding(self):
        return self._encoding


def test_unencodable_stream():
    ja = ["使い方:", "Options"]
    assert _encoding.unencodable_stream(ja, [("stdout", Stream("cp1252"))]) == "stdout (cp1252)"
    assert _encoding.unencodable_stream(ja, [("stdout", Stream("utf-8")), ("stderr", Stream("UTF8"))]) == ""
    assert _encoding.unencodable_stream(ja, [("stdout", Stream("utf-16")), ("stderr", Stream("utf-32-le"))]) == ""
    assert _encoding.unencodable_stream(ja, [("stdout", Stream("euc_jp"))]) == ""  # a legacy encoding that has the text
    assert _encoding.unencodable_stream(["Usage:"], [("stdout", Stream("ascii"))]) == ""
    assert _encoding.unencodable_stream(["한국어"], [("stdout", Stream("utf-8")), ("stderr", Stream("euc_jp"))]) == "stderr (euc_jp)"
    assert _encoding.unencodable_stream(ja, [("stdout", None), ("stderr", io.StringIO())]) == ""  # no stream; a sink without an encoding
    assert _encoding.unencodable_stream(ja, [("stdout", Stream("no-such-codec"))]) == "stdout (no-such-codec)"
    assert _encoding.unencodable_stream(iter(ja), [("stdout", Stream("cp1252")), ("stderr", Stream("cp1252"))]) == "stdout (cp1252)"


@pytest.fixture
def locales(tmp_path):
    (tmp_path / "ja.json").write_text(json.dumps({"version": 1, "language": "ja", "format": "python", "messages": JA}, ensure_ascii=False), encoding="utf-8")
    return tmp_path


def test_init_checks_the_streams_once(locales, monkeypatch, capsys):
    monkeypatch.setattr(localizer._locale, "os_languages", lambda: [])
    monkeypatch.setenv("LOCALIZER_DEBUG", "1")
    monkeypatch.setattr(sys, "stdout", Stream("cp1252"))
    try:
        assert localizer.init(locales, language="ja") == ""
        assert localizer.t("Manage tasks.") == "Manage tasks."
        assert "stdout (cp1252) cannot encode the ja catalog" in capsys.readouterr().err
        monkeypatch.setattr(sys, "stdout", Stream("utf-8"))
        assert localizer.init(locales, language="ja") == "ja"
        assert localizer.t("Manage tasks.") == "タスクを管理します。"
    finally:
        localizer.uninstall()
