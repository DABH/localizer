# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

import json
import os
import plistlib
import sys
import threading

import pytest
from conftest import CONFORMANCE

from localizer import _locale as loc


PLIST = CONFORMANCE.parent.parent / "internal" / "locale" / "testdata" / "GlobalPreferences.plist"
NO_CORPUS = "conformance corpus not present (run from a repository checkout)"  # an sdist carries the tests only


def _golden():
    if not CONFORMANCE.is_dir():
        return [pytest.param([], [], "", marks=pytest.mark.skip(reason=NO_CORPUS))]
    with open(CONFORMANCE / "locale_match.json", encoding="utf-8") as f:
        data = json.load(f)
    return [
        pytest.param(g["available"], c["prefs"], c["want"], id=f"{'+'.join(g['available'])}: {':'.join(c['prefs'])}")
        for g in data["groups"]
        for c in g["cases"]
    ]


@pytest.mark.parametrize("available,prefs,want", _golden())
def test_match_golden(available, prefs, want):
    assert loc.match(prefs, available) == want


def test_match_edges():
    assert loc.match([], ["ja"]) == ""
    assert loc.match(["ja"], []) == ""
    assert loc.match(["ja"], ["not a tag"]) == ""


@pytest.mark.parametrize(
    "posix,tag,is_c",
    [
        ("pt_BR.UTF-8", "pt-BR", False),
        ("de_US.UTF-8", "de-US", False),
        ("sr_RS@latin", "sr-Latn-RS", False),
        ("sr@cyrillic", "sr-Cyrl", False),
        ("zh_TW.Big5", "zh-TW", False),
        ("ja_JP.UTF-8@mod", "ja-JP", False),
        ("C", "", True),
        ("C.UTF-8", "", True),
        ("POSIX", "", True),
        ("", "", False),
        ("  en_GB  ", "en-GB", False),
    ],
)
def test_normalize(posix, tag, is_c):
    assert loc.normalize(posix) == (tag, is_c)


def _env(**values):
    return lambda name: values.get(name)


def test_detect_order():
    no_os = lambda: []
    r = loc.detect(["APP_LANG", "LOCALIZER_LANG"], _env(APP_LANG=" ja ", LOCALIZER_LANG="de", LANG="fr_FR.UTF-8"), no_os)
    assert (r.tags, r.source, r.off, r.pseudo) == (["ja"], "APP_LANG", False, False)
    r = loc.detect(["LOCALIZER_LANG"], _env(LOCALIZER_LANG="Off", LANG="ja_JP"), no_os)
    assert r.off and r.source == "LOCALIZER_LANG"
    r = loc.detect(["LOCALIZER_LANG"], _env(LOCALIZER_LANG="qps"), no_os)
    assert r.pseudo
    r = loc.detect(["LOCALIZER_LANG"], _env(LOCALIZER_LANG="ja:en,C"), no_os)
    assert r.tags == ["ja", "en", "en"]
    r = loc.detect([], _env(LC_ALL="C", LANG="ja_JP.UTF-8", LANGUAGE="de:fr"), no_os)
    assert r.tags == ["en"] and r.source == "LC_ALL"
    r = loc.detect([], _env(LC_MESSAGES="pt_BR.UTF-8", LANG="ja_JP", LANGUAGE="de:fr"), no_os)
    assert r.tags == ["de", "fr", "pt-BR"] and r.source == "LC_MESSAGES"
    r = loc.detect([], _env(LANG="ja_JP.UTF-8"), no_os)
    assert r.tags == ["ja-JP"] and r.source == "LANG"
    r = loc.detect([], _env(LANGUAGE="de:fr"), no_os)
    assert r.tags == ["de", "fr"] and r.source == "LANGUAGE"
    r = loc.detect([], _env(), lambda: ["ja-JP", "en-US"])
    assert r.tags == ["ja-JP", "en-US"] and r.source == "os"
    r = loc.detect([], _env(), no_os)
    assert r == loc.Result()


def test_apple_languages():
    data = plistlib.dumps({"AppleLanguages": ["ja-JP", "zh-Hans-CN", "en-US", 3], "AppleLocale": "ja_JP"}, fmt=plistlib.FMT_BINARY)
    assert loc.apple_languages(data) == ["ja-JP", "zh-Hans-CN", "en-US"]
    assert loc.apple_languages(b"not a plist") == []
    assert loc.apple_languages(plistlib.dumps({"AppleLocale": "ja_JP"}, fmt=plistlib.FMT_BINARY)) == []
    # The Go implementation's fixture reads the same way.
    if not PLIST.is_file():
        pytest.skip(NO_CORPUS)
    assert loc.apple_languages(PLIST.read_bytes()) == ["ja-JP", "zh-Hans-CN", "en-US", "Français-ünicode"]


def _plist(langs):
    return plistlib.dumps({"AppleLanguages": langs}, fmt=plistlib.FMT_BINARY)


def test_plist_languages_reads_regular_files_only(tmp_path):
    """Like the Go runtime: missing, oversized and irregular files (a directory, a FIFO that would block
    forever) all mean no preference, and nothing ever blocks."""
    path = tmp_path / ".GlobalPreferences.plist"
    path.write_bytes(_plist(["ja-JP", "en-US"]))
    assert loc._plist_languages(str(path)) == ["ja-JP", "en-US"]
    assert loc._plist_languages(str(tmp_path / "missing.plist")) == []
    assert loc._plist_languages(str(tmp_path)) == []
    big = tmp_path / "big.plist"
    with open(big, "wb") as f:
        f.truncate(loc._MAX_PLIST + 1)
    assert loc._plist_languages(str(big)) == []
    if hasattr(os, "mkfifo"):
        fifo = tmp_path / "fifo.plist"
        os.mkfifo(fifo)
        result = []
        t = threading.Thread(target=lambda: result.append(loc._plist_languages(str(fifo))), daemon=True)
        t.start()
        t.join(10)
        assert result == [[]], "opening the FIFO blocked"


@pytest.mark.skipif(sys.platform == "win32", reason="HOME is not the home directory on Windows")
def test_apple_languages_come_from_the_home_directory(tmp_path, monkeypatch):
    prefs = tmp_path / "Library" / "Preferences"
    prefs.mkdir(parents=True)
    (prefs / ".GlobalPreferences.plist").write_bytes(_plist(["de-DE", "en-US"]))
    monkeypatch.setenv("HOME", str(tmp_path))
    assert loc._apple_languages() == ["de-DE", "en-US"]
