# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

from localizer._engine import Engine, Mode


def test_on_miss_skips_already_translated_text():
    eng = Engine("ja", {"Show this message and exit.": "このメッセージを表示して終了します。"})
    misses = []
    eng.on_miss = lambda s, mode: misses.append(s)
    assert eng.translate("Show this message and exit.", Mode.HELP) == "このメッセージを表示して終了します。"
    # Frameworks sometimes render a string that was translated once more; that is not a coverage gap.
    assert eng.translate("このメッセージを表示して終了します。", Mode.HELP) == "このメッセージを表示して終了します。"
    assert eng.translate("Print more details.", Mode.HELP) == "Print more details."
    assert misses == ["Print more details."]
    assert eng.is_translation("  このメッセージを表示して終了します。\n") and not eng.is_translation("Print more details.")
