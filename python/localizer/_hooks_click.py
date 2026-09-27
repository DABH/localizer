# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Hooks for Click, both the real package and the copy Typer 0.26+ bundles as ``typer._click``.

Help is rendered from a translated shallow copy ("view") of the command: the copy's help texts and its
parameters' help texts are translated, subcommands are viewed the same way on demand, and listing and
filtering of subcommands is delegated to the real command, so an application that compares its own
objects (for example ``rich_help_panel`` values) keeps working. Usage prefixes and section headings are
translated in the formatter; errors, prompts and the strings Click builds with gettext are translated
where they are produced.
"""

from __future__ import annotations

import contextlib
import copy
import gettext
import importlib
import threading
from typing import Any

from . import _api, _hooks
from ._engine import Mode

_render = threading.local()


def _module(name: str):
    try:
        return importlib.import_module(name)
    except Exception:
        return None


def _engine():
    st = _api.state()
    return st.engine if st is not None else None


def _help(s):
    eng = _engine()
    if eng is None or not isinstance(s, str) or s == "":
        return s
    return eng.translate(s, Mode.HELP)


def _lookup(s: str) -> str:
    eng = _engine()
    if eng is None or not isinstance(s, str):
        return s
    return eng.lookup(s)[0]


# -- views -------------------------------------------------------------------------------------


def _param_view(p: Any) -> Any:
    try:
        q = copy.copy(p)
    except Exception:
        return p
    for attr in ("help", "rich_help_panel"):
        v = getattr(p, attr, None)
        if isinstance(v, str) and v:
            try:
                setattr(q, attr, _help(v))
            except Exception:
                pass
    dep = getattr(p, "deprecated", None)
    if isinstance(dep, str) and dep:
        try:
            q.deprecated = _help(dep)
        except Exception:
            pass
    return q


def view(cmd: Any, cache: dict[int, Any] | None = None) -> Any:
    """A shallow copy of ``cmd`` whose help texts are translated. The original is never changed."""
    if cmd is None:
        return None
    if cache is None:
        cache = {}
    key = id(cmd)
    if key in cache:
        return cache[key]
    try:
        v = copy.copy(cmd)
    except Exception:
        return cmd
    cache[key] = v
    for attr in ("help", "short_help", "epilog", "rich_help_panel"):
        val = getattr(cmd, attr, None)
        if isinstance(val, str) and val:
            try:
                setattr(v, attr, _help(val))
            except Exception:
                pass
    dep = getattr(cmd, "deprecated", None)
    if isinstance(dep, str) and dep:
        try:
            v.deprecated = _help(dep)
        except Exception:
            pass
    get_params = getattr(type(cmd), "get_params", None)
    if get_params is not None:

        def _get_params(ctx, _orig=get_params):
            return [_param_view(p) for p in _orig(cmd, ctx)]

        v.get_params = _get_params
    if isinstance(getattr(cmd, "params", None), list):
        v.params = [_param_view(p) for p in cmd.params]
    if hasattr(cmd, "list_commands") and hasattr(cmd, "get_command"):
        # Listing and filtering see the real subcommands; rendering sees their views.
        v.list_commands = lambda ctx: cmd.list_commands(ctx)
        v.get_command = lambda ctx, name: view(cmd.get_command(ctx, name), cache)
    return v


def _wrap_format_help(command_cls: type) -> None:
    orig = command_cls.__dict__.get("format_help")
    if orig is None:
        return

    def format_help(self, ctx, formatter):
        eng = _engine()
        if eng is None or getattr(_render, "active", False):
            return orig(self, ctx, formatter)
        _render.active = True
        try:
            try:
                from . import _dump

                _dump.maybe_dump_click(ctx)
            except Exception:
                _api.debug_exc("dump")
            return orig(view(self), ctx, formatter)
        except Exception:
            _api.debug_exc("format_help")
            return orig(self, ctx, formatter)
        finally:
            _render.active = False

    _hooks.patch(command_cls, "format_help", format_help)


# -- formatter ---------------------------------------------------------------------------------


def _wrap_formatter(formatter_cls: type) -> None:
    write_usage = formatter_cls.__dict__.get("write_usage")
    if write_usage is not None:

        def _write_usage(self, prog, args="", prefix=None):
            if prefix is None and _engine() is not None:
                prefix = _lookup("Usage:") + " "
            return write_usage(self, prog, args, prefix)

        _hooks.patch(formatter_cls, "write_usage", _write_usage)
    write_heading = formatter_cls.__dict__.get("write_heading")
    if write_heading is not None:

        def _write_heading(self, heading):
            if _engine() is not None and isinstance(heading, str):
                heading = _lookup(heading)
            return write_heading(self, heading)

        _hooks.patch(formatter_cls, "write_heading", _write_heading)


# -- gettext -----------------------------------------------------------------------------------


def _patch_gettext(mod) -> None:
    """Redirects a module's ``_``/``ngettext`` bindings, but only the plain gettext ones: an
    application's own translation function is left alone."""
    if mod is None:
        return
    if getattr(mod, "_", None) is gettext.gettext:
        _hooks.patch(mod, "_", lambda s: _lookup(s))
    if getattr(mod, "ngettext", None) is gettext.ngettext:
        _hooks.patch(mod, "ngettext", lambda s, p, n: _lookup(s if n == 1 else p))


# -- errors ------------------------------------------------------------------------------------


@contextlib.contextmanager
def translated_message(exc: Any):
    """Makes ``exc.format_message()`` return its translation for the duration of the block."""
    eng = _engine()
    if eng is None or type(exc).__name__ == "NoArgsIsHelpError":
        yield
        return
    try:
        msg = exc.format_message()
    except Exception:
        yield
        return
    if not isinstance(msg, str):
        yield
        return
    tr = eng.translate(msg, Mode.ERROR)
    if tr == msg:
        yield
        return
    exc.format_message = lambda: tr
    try:
        yield
    finally:
        try:
            del exc.format_message
        except AttributeError:
            pass


def _wrap_show(exc_cls: type, exceptions_mod) -> None:
    orig = exc_cls.__dict__.get("show")
    if orig is None:
        return

    def show(self, file=None):
        eng = _engine()
        if eng is None:
            return orig(self, file)
        # Everything show() prints (the usage line, the "Try ... for help." hint, "Error: ...") goes
        # through the module's echo; translating there covers Click's gettext strings and the copy of
        # Click bundled with Typer, which hard-codes them.
        echo = getattr(exceptions_mod, "echo", None)
        if echo is None:
            with translated_message(self):
                return orig(self, file)

        def translating_echo(message=None, *args, **kwargs):
            if isinstance(message, str):
                message = eng.translate(message, Mode.ERROR)
            return echo(message, *args, **kwargs)

        exceptions_mod.echo = translating_echo
        try:
            return orig(self, file)
        finally:
            exceptions_mod.echo = echo

    _hooks.patch(exc_cls, "show", show)


# -- prompts -----------------------------------------------------------------------------------


def _wrap_prompts(prefix: str, modules: list) -> None:
    termui = _module(prefix + ".termui")
    if termui is None:
        return
    for name in ("prompt", "confirm"):
        orig = getattr(termui, name, None)
        if orig is None:
            continue

        def make(orig=orig):
            def wrapper(text, *args, **kwargs):
                eng = _engine()
                if eng is not None and isinstance(text, str):
                    text = eng.translate(text, Mode.OUTPUT)
                return orig(text, *args, **kwargs)

            wrapper.__wrapped__ = orig
            return wrapper

        wrapper = make()
        _hooks.patch(termui, name, wrapper)
        for mod in modules:
            if mod is not termui:
                _hooks.patch(mod, name, wrapper, only_if=orig)


# -- install -----------------------------------------------------------------------------------


def install(prefix: str, *, error_hook: bool, prompt_hook: bool) -> None:
    """Hooks one Click implementation: ``"click"`` or ``"typer._click"``."""
    if _hooks.installed(prefix):
        return
    root = _module(prefix)
    core = _module(prefix + ".core")
    formatting = _module(prefix + ".formatting")
    exceptions = _module(prefix + ".exceptions")
    if root is None or core is None or formatting is None or exceptions is None:
        return
    _hooks.mark(prefix)
    command_cls = getattr(core, "Command", None)
    if isinstance(command_cls, type):
        _wrap_format_help(command_cls)
    formatter_cls = getattr(formatting, "HelpFormatter", None)
    if isinstance(formatter_cls, type):
        _wrap_formatter(formatter_cls)
    modules = [root, core, formatting, exceptions]
    for name in ("parser", "decorators", "types", "termui", "_termui_impl", "utils", "shell_completion", "_winconsole"):
        mod = _module(prefix + "." + name)
        if mod is not None:
            modules.append(mod)
    for mod in modules:
        _patch_gettext(mod)
    if error_hook:
        for name in ("ClickException", "UsageError"):
            cls = getattr(exceptions, name, None)
            if isinstance(cls, type):
                _wrap_show(cls, exceptions)
    if prompt_hook:
        typer = _module("typer") if prefix != "click" or "typer" in __import__("sys").modules else None
        _wrap_prompts(prefix, modules + ([typer] if typer is not None else []))
