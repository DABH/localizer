# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

import json
import pathlib

import pytest

from localizer import _catalog as cat


def test_languages_and_load(tmp_path: pathlib.Path):
    (tmp_path / "ja.json").write_text('{"version": 1, "language": "ja", "format": "python", "messages": {"Add a task.": "タスクを追加します。"}}', encoding="utf-8")
    (tmp_path / "de.json").write_text('{"language": "de", "messages": {}}', encoding="utf-8")
    (tmp_path / "_notes.json").write_text("{}", encoding="utf-8")
    (tmp_path / ".hidden.json").write_text("{}", encoding="utf-8")
    (tmp_path / "readme.txt").write_text("x", encoding="utf-8")
    (tmp_path / "sub.json").mkdir()
    root = cat.resolve(tmp_path)
    assert cat.languages(root) == ["de", "ja"]
    f = cat.load(root, "ja")
    assert (f.language, f.version, f.format, f.messages) == ("ja", 1, "python", {"Add a task.": "タスクを追加します。"})
    assert cat.load(root, "de").messages == {}
    assert cat.languages(cat.resolve(str(tmp_path))) == ["de", "ja"]


def test_load_rejects_bad_catalogs(tmp_path: pathlib.Path):
    root = cat.resolve(tmp_path)
    (tmp_path / "v9.json").write_text('{"version": 9, "language": "v9", "messages": {}}', encoding="utf-8")
    with pytest.raises(ValueError):
        cat.load(root, "v9")
    (tmp_path / "bad.json").write_text('{"messages": {"a": 1}}', encoding="utf-8")
    with pytest.raises(ValueError):
        cat.load(root, "bad")
    (tmp_path / "list.json").write_text("[]", encoding="utf-8")
    with pytest.raises(ValueError):
        cat.load(root, "list")


def test_package_resources():
    from localizer import builtin

    root = cat.resolve("localizer.builtin")
    assert cat.languages(root) == builtin.languages()


def test_dumps_is_canonical():
    f = cat.File(language="ja", messages={"b": "2", "a": "1 x", "é": "e", "Z": "z"})
    out = cat.dumps(f)
    assert out == '{\n  "version": 1,\n  "language": "ja",\n  "format": "python",\n  "messages": {\n    "Z": "z",\n    "a": "1\\u2028x",\n    "b": "2",\n    "é": "e"\n  }\n}\n'
    assert json.loads(out)["messages"]["a"] == "1 x"
    f.format = ""
    assert '"format"' not in cat.dumps(f)
