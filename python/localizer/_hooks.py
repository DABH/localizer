# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Installs the framework hooks (Click, Typer, argparse) and keeps a registry so they can be removed.

Hooks translate at render time on copies of the framework's objects; nothing the application built is
mutated. They read the active state on every call, so installing them once per process is enough. A
hook that meets an unexpected framework shape skips itself; one that fails at render time falls back
to the original behaviour.
"""

from __future__ import annotations

import sys
from typing import Any

from . import _api

__all__ = ["install", "uninstall", "patch", "installed"]

_MISSING = object()
_registry: list[tuple[Any, str, Any]] = []  # (owner, attribute, original or _MISSING)
_installed: set[str] = set()


def patch(owner: Any, attr: str, value: Any, *, only_if=_MISSING) -> bool:
    """Replaces ``owner.attr`` (a module attribute or a method defined on that class), remembering the
    original. Returns False when the attribute doesn't exist there or ``only_if`` doesn't match."""
    if owner is None:
        return False
    if isinstance(owner, type):
        if attr not in owner.__dict__:
            return False
        original = owner.__dict__[attr]
    else:
        if not hasattr(owner, attr):
            return False
        original = getattr(owner, attr)
    if only_if is not _MISSING and original is not only_if:
        return False
    _registry.append((owner, attr, original))
    setattr(owner, attr, value)
    return True


def installed(kind: str) -> bool:
    return kind in _installed


def mark(kind: str) -> None:
    _installed.add(kind)


def install(app: Any, *, error_hook: bool = True, prompt_hook: bool = True) -> None:
    """Installs hooks for the frameworks the application uses. ``app`` may be a Typer app, a Click
    command, an argparse parser or None (then every imported framework is hooked)."""
    from . import _hooks_argparse, _hooks_click, _hooks_typer

    want_click = want_typer = want_argparse = app is None
    kind = type(app).__module__.split(".")[0] if app is not None else ""
    if kind == "typer":
        want_typer = want_click = True
    elif kind == "click":
        want_click = True
    elif kind == "argparse":
        want_argparse = True
    elif app is not None:
        want_click = want_typer = want_argparse = True
    if want_click or want_typer:
        for prefix in ("click", "typer._click"):
            if prefix.split(".")[0] in sys.modules or want_typer and prefix != "click":
                try:
                    _hooks_click.install(prefix, error_hook=error_hook, prompt_hook=prompt_hook)
                except Exception:
                    _api.debug_exc(f"hooking {prefix}")
    if want_typer or (app is None and "typer" in sys.modules):
        try:
            _hooks_typer.install(error_hook=error_hook)
        except Exception:
            _api.debug_exc("hooking typer")
    if want_argparse or (app is None and "argparse" in sys.modules):
        try:
            _hooks_argparse.install(app, error_hook=error_hook)
        except Exception:
            _api.debug_exc("hooking argparse")


def uninstall() -> None:
    for owner, attr, original in reversed(_registry):
        try:
            if original is _MISSING:
                delattr(owner, attr)
            else:
                setattr(owner, attr, original)
        except Exception:
            pass
    _registry.clear()
    _installed.clear()
