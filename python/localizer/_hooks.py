# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Installs the framework hooks (Click, rich-click, Typer, argparse) and keeps a registry so they can be
removed.

Hooks translate at render time on copies of the framework's objects; nothing the application built is
mutated. They read the active state on every call, so installing them once per process is enough. A
hook that meets an unexpected framework shape skips itself; one that fails at render time falls back
to the original behaviour.

Modules a framework imports only when it renders (Typer's ``rich_utils`` and, with it, Rich's Markdown
renderer and Pygments; Click's editor, pager and completion helpers) are never imported here: that would
cost every invocation tens of milliseconds for a help screen that is rarely shown. :func:`on_import`
patches such a module the moment the framework imports it.
"""

from __future__ import annotations

import sys
from collections.abc import Callable
from types import ModuleType
from typing import Any

from . import _api

__all__ = ["install", "uninstall", "patch", "installed", "mark", "on_import"]

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


# -- deferred patching -------------------------------------------------------------------------


class _ImportHook:
    """A ``sys.meta_path`` finder that runs a callback on a module right after its first execution. It
    finds nothing itself: it hands the lookup to the finders behind it and wraps the loader they return,
    so the module is loaded exactly as it would be otherwise."""

    def __init__(self) -> None:
        self.callbacks: dict[str, Callable[[ModuleType], None]] = {}

    def find_spec(self, fullname, path=None, target=None):
        if fullname not in self.callbacks:
            return None
        for finder in sys.meta_path:
            if finder is self:
                continue
            find_spec = getattr(finder, "find_spec", None)
            if find_spec is None:
                continue
            spec = find_spec(fullname, path, target)
            if spec is None:
                continue
            if spec.loader is not None and hasattr(spec.loader, "exec_module"):
                spec.loader = _Loader(spec.loader, self, fullname)
            return spec
        return None

    def invalidate_caches(self) -> None:
        pass

    def run(self, name: str, module: ModuleType) -> None:
        callback = self.callbacks.get(name)  # gone after uninstall()
        if callback is not None:
            _call(name, callback, module)


class _Loader:
    """Executes a module as its loader would, then runs the hook's callback. Everything else (``get_source``
    for tracebacks, ``get_data``, ``is_package``, ...) is the wrapped loader's."""

    def __init__(self, loader: Any, hook: _ImportHook, name: str) -> None:
        self._loader = loader
        self._hook = hook
        self._name = name

    def create_module(self, spec):
        create = getattr(self._loader, "create_module", None)
        return create(spec) if create is not None else None

    def exec_module(self, module: ModuleType) -> None:
        self._loader.exec_module(module)
        self._hook.run(self._name, module)

    def __getattr__(self, name: str):
        loader = self.__dict__.get("_loader")
        if loader is None:
            raise AttributeError(name)
        return getattr(loader, name)


_import_hook: _ImportHook | None = None


def _call(name: str, callback: Callable[[ModuleType], None], module: ModuleType) -> None:
    try:
        callback(module)
    except Exception:
        _api.debug_exc(f"hooking {name}")


def on_import(name: str, callback: Callable[[ModuleType], None]) -> None:
    """Runs ``callback(module)`` on the module ``name``: right away when it is imported already, otherwise
    when the framework first imports it. One callback per module name; :func:`uninstall` forgets them."""
    global _import_hook
    module = sys.modules.get(name)
    if module is not None:
        _call(name, callback, module)
        return
    if _import_hook is None:
        _import_hook = _ImportHook()
    if _import_hook not in sys.meta_path:
        sys.meta_path.insert(0, _import_hook)
    _import_hook.callbacks[name] = callback


# -- install -----------------------------------------------------------------------------------


def install(app: Any, *, error_hook: bool = True, prompt_hook: bool = True) -> None:
    """Installs hooks for the frameworks the application uses. ``app`` may be a Typer app, a Click or
    rich-click command, an argparse parser or None (then every imported framework is hooked)."""
    from . import _hooks_argparse, _hooks_click, _hooks_typer

    want_click = want_typer = want_argparse = app is None
    want_rich_click = False
    kind = type(app).__module__.split(".")[0] if app is not None else ""
    if kind == "typer":
        want_typer = want_click = True
    elif kind == "click":
        want_click = True
    elif kind == "rich_click":
        want_click = want_rich_click = True
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
    if want_rich_click or (want_click and "rich_click" in sys.modules):
        try:
            from . import _hooks_rich_click

            _hooks_rich_click.install(error_hook=error_hook)
        except Exception:
            _api.debug_exc("hooking rich_click")
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
    global _import_hook
    if _import_hook is not None:
        _import_hook.callbacks.clear()
        try:
            sys.meta_path.remove(_import_hook)
        except ValueError:
            pass
        _import_hook = None
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
