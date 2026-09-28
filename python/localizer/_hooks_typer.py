# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Hooks for Typer's own rendering: its rich help and error panels and the strings it builds itself."""

from __future__ import annotations

import contextlib
import gettext
import importlib
import threading

from . import _api, _hooks, _hooks_click

# Module-level texts typer.rich_utils reads each time it renders (computed once at import from gettext).
# They hold their translations only while one of its renderers runs, so a later switch back to English
# (localize(..., language="en")) never shows a translated panel title, and a language switch takes
# effect at the next render.
_CONSTANTS = (
    "DEPRECATED_STRING",
    "DEFAULT_STRING",
    "ENVVAR_STRING",
    "REQUIRED_LONG_STRING",
    "ARGUMENTS_PANEL_TITLE",
    "OPTIONS_PANEL_TITLE",
    "COMMANDS_PANEL_TITLE",
    "ERRORS_PANEL_TITLE",
    "ABORTED_TEXT",
    "RICH_HELP",
)

_lock = threading.Lock()
_renders = 0  # renderers in flight, in any thread: the constants are translated while it is > 0
_saved: list[tuple[str, str]] = []  # (name, original text) while they are


def _module(name: str):
    try:
        return importlib.import_module(name)
    except Exception:
        return None


@contextlib.contextmanager
def _translated_constants(rich_utils):
    """Swaps the constants for their translations for the duration of a render; the originals come
    back when the last concurrent render finishes."""
    global _renders
    with _lock:
        _renders += 1
        if _renders == 1:
            _swap(rich_utils)
    try:
        yield
    finally:
        with _lock:
            _renders -= 1
            if _renders == 0:
                _restore(rich_utils)


def _swap(rich_utils) -> None:
    for name in _CONSTANTS:
        current = getattr(rich_utils, name, None)
        if not isinstance(current, str):
            continue
        try:
            translated = _hooks_click._lookup(current)
        except Exception:
            _api.debug_exc("rich_utils." + name)
            continue
        if translated != current:
            _saved.append((name, current))
            setattr(rich_utils, name, translated)


def _restore(rich_utils) -> None:
    for name, original in reversed(_saved):
        try:
            setattr(rich_utils, name, original)
        except Exception:
            pass
    _saved.clear()


def install(*, error_hook: bool) -> None:
    if _hooks.installed("typer"):
        return
    _hooks.mark("typer")
    core = _module("typer.core")
    if core is not None and getattr(core, "_", None) is gettext.gettext:
        _hooks.patch(core, "_", lambda s: _hooks_click._lookup(s))
    rich_utils = _module("typer.rich_utils")
    if rich_utils is None:
        return
    # Each renderer runs on translated input with the translated constants; if anything in that
    # fails (a translation that breaks Rich markup, say) the original input renders in English.
    orig_help = getattr(rich_utils, "rich_format_help", None)
    if orig_help is not None:

        def rich_format_help(*args, **kwargs):
            if _api.state() is None:
                return orig_help(*args, **kwargs)
            try:
                from . import _dump

                if "ctx" in kwargs:
                    _dump.maybe_dump_click(kwargs["ctx"])
            except Exception:
                _api.debug_exc("dump")
            try:
                view_args, view_kwargs = args, kwargs
                if "obj" in kwargs:
                    view_kwargs = dict(kwargs, obj=_hooks_click.view(kwargs["obj"]))
                elif args:
                    view_args = (_hooks_click.view(args[0]), *args[1:])
                with _translated_constants(rich_utils):
                    return orig_help(*view_args, **view_kwargs)
            except Exception:
                _api.debug_exc("rich help")
                return orig_help(*args, **kwargs)

        _hooks.patch(rich_utils, "rich_format_help", rich_format_help)
    orig_error = getattr(rich_utils, "rich_format_error", None)
    if orig_error is not None:

        def rich_format_error(exc, *args, **kwargs):
            if _api.state() is None:
                return orig_error(exc, *args, **kwargs)
            try:
                with _translated_constants(rich_utils):
                    if not error_hook:
                        return orig_error(exc, *args, **kwargs)
                    with _hooks_click.translated_message(exc):
                        return orig_error(exc, *args, **kwargs)
            except Exception:
                _api.debug_exc("rich error")
                return orig_error(exc, *args, **kwargs)

        _hooks.patch(rich_utils, "rich_format_error", rich_format_error)
    orig_abort = getattr(rich_utils, "rich_abort_error", None)
    if orig_abort is not None:

        def rich_abort_error(*args, **kwargs):
            if _api.state() is None:
                return orig_abort(*args, **kwargs)
            try:
                with _translated_constants(rich_utils):
                    return orig_abort(*args, **kwargs)
            except Exception:
                _api.debug_exc("rich abort")
                return orig_abort(*args, **kwargs)

        _hooks.patch(rich_utils, "rich_abort_error", rich_abort_error)
