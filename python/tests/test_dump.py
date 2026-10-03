# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""LOCALIZER_DUMP creates the file new and writes nothing in a privileged process, as the Go runtime does."""

import argparse
import json
import os

import pytest

import localizer

JA = {"Manage tasks.": "タスクを管理します。", "Print more details.": "詳細を表示します。"}


def make_parser():
    parser = argparse.ArgumentParser(prog="taskctl", description="Manage tasks.")
    parser.add_argument("--verbose", action="store_true", help="Print more details.")
    parser.add_argument("--quiet", action="store_true", help="Print less.")
    return parser


@pytest.fixture
def locales(tmp_path):
    (tmp_path / "ja.json").write_text(json.dumps({"version": 1, "language": "ja", "format": "python", "messages": JA}, ensure_ascii=False), encoding="utf-8")
    return tmp_path


@pytest.fixture(autouse=True)
def clean(monkeypatch):
    for var in ("LOCALIZER_LANG", "LOCALIZER_DEBUG", "LOCALIZER_DUMP", "LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE"):
        monkeypatch.delenv(var, raising=False)
    monkeypatch.setenv("LOCALIZER_DEBUG", "1")
    monkeypatch.setattr(localizer._locale, "os_languages", lambda: [])
    yield
    localizer.uninstall()


def test_dump_is_written_as_the_go_runtime_writes_it(locales, tmp_path, monkeypatch):
    dump = tmp_path / "dump.json"
    monkeypatch.setenv("LOCALIZER_DUMP", str(dump))
    localizer.localize(make_parser(), locales, language="ja")
    raw = dump.read_bytes()
    assert raw.endswith(b"}\n") and b"\r" not in raw and b"\\u" not in raw  # raw UTF-8, LF only, trailing newline
    doc = json.loads(raw)
    assert doc["language"] == "ja"
    flags = {e["flag"]: e["translated"] for e in doc["entries"] if e["kind"] == "flag"}
    assert flags == {"-h": True, "--verbose": True, "--quiet": False}  # -h: the built-in catalog


def test_dump_never_overwrites(locales, tmp_path, monkeypatch, capsys):
    dump = tmp_path / "dump.json"
    dump.write_text("keep me", encoding="utf-8")
    monkeypatch.setenv("LOCALIZER_DUMP", str(dump))
    localizer.localize(make_parser(), locales, language="ja")
    assert dump.read_text(encoding="utf-8") == "keep me"
    assert f"localizer: LOCALIZER_DUMP: {dump} exists and is left as it is" in capsys.readouterr().err


def test_dump_reports_an_unwritable_path(locales, tmp_path, monkeypatch, capsys):
    monkeypatch.setenv("LOCALIZER_DUMP", str(tmp_path / "no" / "such" / "dir" / "dump.json"))
    localizer.localize(make_parser(), locales, language="ja")
    assert "localizer: dump: " in capsys.readouterr().err


@pytest.mark.skipif(not hasattr(os, "geteuid"), reason="no user ids on this platform")
@pytest.mark.parametrize("euid", ["root", "setuid"])
def test_dump_is_ignored_in_a_privileged_process(locales, tmp_path, monkeypatch, capsys, euid):
    monkeypatch.setattr(os, "geteuid", (lambda: 0) if euid == "root" else (lambda: os.getuid() + 1))
    dump = tmp_path / "dump.json"
    monkeypatch.setenv("LOCALIZER_DUMP", str(dump))
    assert localizer.localize(make_parser(), locales, language="ja") == "ja"
    assert not dump.exists()
    assert "localizer: LOCALIZER_DUMP is ignored in a privileged process" in capsys.readouterr().err
