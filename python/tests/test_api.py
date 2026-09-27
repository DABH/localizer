# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

import json

import pytest

import localizer
from localizer import _api

JA = {
    "Add a task.": "タスクを追加します。",
    "Hello {name}": "こんにちは {name}",
    "Missing argument": "引数がありません",
    "Error": "エラー",
    "Deleted {n} tasks.": "{n} 件のタスクを削除しました。",
}
DE = {"Add a task.": "Eine Aufgabe hinzufügen."}


@pytest.fixture
def locales(tmp_path):
    for lang, msgs in (("ja", JA), ("de", DE)):
        (tmp_path / f"{lang}.json").write_text(json.dumps({"version": 1, "language": lang, "format": "python", "messages": msgs}, ensure_ascii=False), encoding="utf-8")
    return tmp_path


@pytest.fixture(autouse=True)
def clean(monkeypatch):
    for var in ("LOCALIZER_LANG", "LOCALIZER_DEBUG", "LOCALIZER_DUMP", "LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE", "APP_LANG"):
        monkeypatch.delenv(var, raising=False)
    monkeypatch.setattr(localizer._locale, "os_languages", lambda: [])
    yield
    localizer.uninstall()


def test_helpers_pass_through_before_init():
    assert localizer.lang() == ""
    assert localizer.t("Add a task.") == "Add a task."
    assert localizer.tf("Hello {name}", name="x") == "Hello x"
    assert localizer.error(ValueError("boom")) == "boom"
    assert localizer.translate("x", localizer.Mode.HELP) == "x"


def test_init_and_helpers(locales):
    assert localizer.init(locales, language="ja") == "ja"
    assert localizer.lang() == "ja"
    assert localizer.t("Add a task.") == "タスクを追加します。"
    assert localizer.t("  Add a task.\n") == "  タスクを追加します。\n"
    assert localizer.t("Hello {name}") == "こんにちは {name}"  # a format string: exact lookup only
    assert localizer.tf("Hello {name}", name="Ann") == "こんにちは Ann"
    assert localizer.tf("Bye {name}", name="Ann") == "Bye Ann"
    assert localizer.t("Deleted 3 tasks.") == "3 件のタスクを削除しました。"
    assert localizer.error("Missing argument") == "引数がありません"
    assert localizer.error(RuntimeError("Error: Missing argument")) == "エラー: 引数がありません"
    assert localizer.translate("Add a task.\n\nnot in catalog", localizer.Mode.HELP) == "タスクを追加します。\n\nnot in catalog"
    assert localizer.t("unknown text") == "unknown text"
    assert localizer.t("") == ""


def test_language_selection(locales, monkeypatch):
    monkeypatch.setenv("APP_LANG", "de")
    monkeypatch.setenv("LOCALIZER_LANG", "ja")
    assert localizer.init(locales, env_var="APP_LANG") == "de"
    assert localizer.t("Add a task.") == "Eine Aufgabe hinzufügen."
    assert localizer.init(locales) == "ja"
    monkeypatch.setenv("LOCALIZER_LANG", "en")
    assert localizer.init(locales) == ""
    assert localizer.t("Add a task.") == "Add a task."
    monkeypatch.setenv("LOCALIZER_LANG", "off")
    assert localizer.init(locales) == ""
    monkeypatch.setenv("LOCALIZER_LANG", "it")
    assert localizer.init(locales) == ""
    monkeypatch.delenv("LOCALIZER_LANG")
    monkeypatch.setenv("LANG", "ja_JP.UTF-8")
    assert localizer.init(locales) == "ja"
    monkeypatch.setenv("LC_ALL", "C")
    assert localizer.init(locales) == ""
    assert localizer.init(locales, language="de") == "de"
    assert localizer.init(locales, language="off") == ""


def test_pseudo(locales, monkeypatch):
    monkeypatch.setenv("LOCALIZER_LANG", "qps")
    assert localizer.init(locales) == "qps"
    assert localizer.t("Add a task.") == "⟦Åďď á ţášķ.⟧"
    assert localizer.t("Deleted 3 tasks.") == "⟦Ďéļéţéď 3 ţášķš.⟧"
    assert localizer.t("not in catalog") == "not in catalog"


def test_missing_catalog_dir(tmp_path):
    assert localizer.init(tmp_path / "nowhere", language="ja") == ""
    assert localizer.t("Add a task.") == "Add a task."


def test_init_fails_open(locales, monkeypatch):
    def boom(*_a, **_k):
        raise RuntimeError("boom")

    monkeypatch.setattr(_api, "_setup", boom)
    assert localizer.init(locales, language="ja") == ""
    assert localizer.t("Add a task.") == "Add a task."


def test_debug_reports_misses(locales, monkeypatch, capsys):
    monkeypatch.setenv("LOCALIZER_DEBUG", "1")
    localizer.init(locales, language="ja")
    localizer.translate("Not translated yet.\nSecond line", localizer.Mode.HELP)
    assert "localizer: untranslated (ja): 'Not translated yet.'" in capsys.readouterr().err
