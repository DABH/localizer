# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Runs the vectors shared with the Go implementation (testdata/conformance)."""

import pytest

from localizer import _format as fmt


def _cases(name):
    import json
    from conftest import CONFORMANCE

    with open(CONFORMANCE / name, encoding="utf-8") as f:
        data = json.load(f)
    assert data["syntax"] == "python"
    return [pytest.param(c, id=c.get("name") or c.get("input", "")) for c in data["cases"]]


@pytest.mark.parametrize("case", _cases("placeholders.json"))
def test_placeholders(case):
    got = fmt.extract(case["source"])
    want = case["placeholders"]
    assert got.fields == sorted(want.get("fields", []))
    assert got.backquoted == sorted(want.get("backquoted", []))
    assert got.tags == sorted(want.get("tags", []))
    if "translation" in case and "valid" in case:
        tr = case["translation"]
        valid = got == fmt.extract(tr) and not fmt.new_control_chars(case["source"], tr)
        assert valid == case["valid"], got.diff(fmt.extract(tr))


@pytest.mark.parametrize("case", _cases("roundtrip.json"))
def test_roundtrip(case):
    p = fmt.compile(case["format"])
    assert (p is not None) == case.get("compiles", True)
    if p is None or "input" not in case:
        return
    m = p.match(case["input"])
    assert (m is not None) == case.get("match", True)
    if m is None:
        return
    captures, classes = m
    assert captures == case.get("captures", {})
    for name, cls in case.get("classes", {}).items():
        assert classes[name] == cls, name
    if "translation" not in case:
        return
    out = p.splice(case["translation"], captures)
    assert (out is not None) == case.get("splices", True)
    if out is not None and "spliced" in case:
        assert out == case["spliced"]


@pytest.mark.parametrize("case", _cases("pseudo.json"))
def test_pseudo(case):
    assert fmt.pseudo(case["input"]) == case["output"]


def _engine_cases():
    import json
    from conftest import CONFORMANCE

    from localizer._engine import Engine, Mode

    with open(CONFORMANCE / "engine.json", encoding="utf-8") as f:
        data = json.load(f)
    modes = {"output": Mode.OUTPUT, "help": Mode.HELP, "error": Mode.ERROR}
    out = []
    for g in data["groups"]:
        if g["syntax"] != "python":
            continue
        engine = Engine.pseudo(g["catalog"]) if g.get("pseudo") else Engine("ja", g["catalog"])
        for c in g["cases"]:
            out.append(pytest.param(engine, c["input"], modes[c["mode"]], c["want"], id=f"{g['name']}: {c['input'][:40]}"))
    return out


@pytest.mark.parametrize("engine,text,mode,want", _engine_cases())
def test_engine(engine, text, mode, want):
    assert engine.translate(text, mode) == want
