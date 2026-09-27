# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Hooks for Typer's own rendering: its rich help and error panels and the strings it builds itself."""

from __future__ import annotations

import gettext
import importlib

from . import _api, _hooks, _hooks_click

# Module-level texts typer.rich_utils reads each time it renders (computed once at import from gettext).
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


def _module(name: str):
    try:
        return importlib.import_module(name)
    except Exception:
        return None


def install(*, error_hook: bool) -> None:
    core = _module("typer.core")
    rich_utils = _module("typer.rich_utils")
    if rich_utils is not None:
        _patch_constants(rich_utils)
    if _hooks.installed("typer"):
        return
    _hooks.mark("typer")
    if core is not None:
        if getattr(core, "_", None) is gettext.gettext:
            _hooks.patch(core, "_", lambda s: _hooks_click._lookup(s))
    if rich_utils is None:
        return
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
                if "obj" in kwargs:
                    kwargs = dict(kwargs, obj=_hooks_click.view(kwargs["obj"]))
                elif args:
                    args = (_hooks_click.view(args[0]), *args[1:])
            except Exception:
                _api.debug_exc("rich help view")
                return orig_help(*args, **kwargs)
            return orig_help(*args, **kwargs)

        _hooks.patch(rich_utils, "rich_format_help", rich_format_help)
    if error_hook:
        orig_error = getattr(rich_utils, "rich_format_error", None)
        if orig_error is not None:

            def rich_format_error(exc, *args, **kwargs):
                if _api.state() is None:
                    return orig_error(exc, *args, **kwargs)
                with _hooks_click.translated_message(exc):
                    return orig_error(exc, *args, **kwargs)

            _hooks.patch(rich_utils, "rich_format_error", rich_format_error)


def _patch_constants(rich_utils) -> None:
    """Sets the translated texts. Re-run on every localize() so a language switch takes effect."""
    for name in _CONSTANTS:
        current = getattr(rich_utils, name, None)
        if not isinstance(current, str):
            continue
        original = _original(rich_utils, name, current)
        translated = _hooks_click._lookup(original)
        if translated != current:
            _hooks.patch(rich_utils, name, translated)


def _original(rich_utils, name: str, current: str) -> str:
    for owner, attr, original in _hooks._registry:
        if owner is rich_utils and attr == name:
            return original
    return current
