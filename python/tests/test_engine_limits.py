# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Bounds that keep the engine fast and small on hostile or huge input."""

import time

from localizer import _engine
from localizer._engine import Engine, Mode
from localizer.builtin import messages


def test_long_text_never_reaches_a_template_regex():
    e = Engine("ja", messages("ja"), {"{a} to {b} from {c} failed": "{a} から {b} へ {c} 失敗"})
    text = "word " * 40_000 + "does not exist. trailing"
    start = time.perf_counter()
    assert e.translate(text, Mode.ERROR) == text  # over the bound: no template is tried
    # Under the bound the template still matches, in bounded time.
    assert e.translate("x " * 3_000 + "to y from z failed", Mode.ERROR).endswith("から y へ z 失敗")
    assert time.perf_counter() - start < 1.0


def test_memo_is_bounded_by_bytes():
    e = Engine("ja", {"List Kafka clusters.": "Kafka クラスターを一覧表示します。"})
    long = "x" * (_engine._MAX_MEMO_ENTRY + 1)
    e.translate(long, Mode.OUTPUT)
    assert long not in e._memo[Mode.OUTPUT]
    e.translate("y" * 100, Mode.OUTPUT)
    assert e._memo_size == 200
    e._memo_size = _engine._MAX_MEMO_BYTES
    assert e.translate("List Kafka clusters.", Mode.OUTPUT) == "Kafka クラスターを一覧表示します。"
    assert "List Kafka clusters." not in e._memo[Mode.OUTPUT]


def test_trimmed_keys_are_deterministic():
    e = Engine("ja", {" Done": "A", "Done\n": "B", "Fine ": "C", "Fine": "D"})
    assert e.translate("Done", Mode.OUTPUT) == "A"
    assert e.translate("Fine", Mode.OUTPUT) == "D"
