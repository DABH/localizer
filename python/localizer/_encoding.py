# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Whether the standard streams can show a catalog's text.

A stream whose encoding cannot represent the translations (on Windows, redirected output uses the ANSI
code page unless ``PYTHONUTF8=1``) would raise ``UnicodeEncodeError`` on the first translated help line,
or print ``\\uXXXX`` escapes where the stream's error handler allows it. Either is worse than English, so
the language is checked against ``sys.stdout`` and ``sys.stderr`` once, when it is selected.
"""

from __future__ import annotations

import codecs
import sys
from collections.abc import Iterable

__all__ = ["unencodable_stream"]

# Codecs that represent every code point: nothing to check (``codecs.lookup`` normalizes aliases such as
# "UTF8" and "cp65001" to these names).
_UNIVERSAL = ("utf-8", "utf-16", "utf-32", "utf-7")


def _encoding(stream) -> str | None:
    """The codec a text stream encodes with, or None when it doesn't encode (a missing stream, a sink
    such as StringIO, a wrapper without the attribute)."""
    encoding = getattr(stream, "encoding", None)
    return encoding if isinstance(encoding, str) and encoding else None


def _universal(encoding: str) -> bool:
    try:
        return codecs.lookup(encoding).name.startswith(_UNIVERSAL)
    except LookupError:
        return False


def unencodable_stream(texts: Iterable[str], streams=None) -> str:
    """Names the first standard stream whose encoding cannot represent every one of ``texts``, with its
    encoding ("stdout (cp1252)"), or "" when both can. The check is strict: a lossy error handler such as
    ``backslashreplace`` would print escapes, not the translation. Nothing is encoded for UTF streams."""
    if streams is None:
        streams = (("stdout", sys.stdout), ("stderr", sys.stderr))
    joined: str | None = None
    checked: set[str] = set()
    for name, stream in streams:
        encoding = _encoding(stream)
        if encoding is None or encoding in checked or _universal(encoding):
            continue
        if joined is None:
            joined = "".join(texts)
        try:
            joined.encode(encoding, "strict")
        except (UnicodeEncodeError, LookupError):
            return f"{name} ({encoding})"
        checked.add(encoding)
    return ""
