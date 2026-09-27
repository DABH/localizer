# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Translations of the strings Click, Typer and argparse print themselves (help headings, the help
option, usage errors, prompts), shipped with the runtime. An application's catalog takes precedence."""

from __future__ import annotations

import threading
from importlib import resources

from .. import _catalog, _locale

__all__ = ["languages", "messages"]

_lock = threading.Lock()
_cache: dict[str, dict[str, str]] = {}
_languages: list[str] | None = None


def _root():
    return resources.files(__name__)


def languages() -> list[str]:
    """The languages the built-in catalog covers."""
    global _languages
    if _languages is None:
        _languages = _catalog.languages(_root())
    return _languages


def messages(lang: str) -> dict[str, str]:
    """The built-in catalog that best matches ``lang``, or an empty dict."""
    best = _locale.match([lang], languages())
    if not best:
        return {}
    with _lock:
        m = _cache.get(best)
        if m is None:
            try:
                m = _catalog.load(_root(), best).messages
            except Exception:
                m = {}
            _cache[best] = m
        return m
