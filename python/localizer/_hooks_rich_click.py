# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Hooks for rich-click (``rich_click``, 1.9 and later).

rich-click's command classes render help themselves, overriding ``format_help``, so the hook on Click's
``Command`` never sees them: their ``format_help`` is wrapped here the same way, rendering a translated
view of the command. rich-click lists subcommands and parameters through ``ctx.command`` rather than
through the command being rendered, so the context points at the view for the duration. Panel titles and
table labels (``[default: {}]``, ``[required]``, ...) come from a configuration object the formatter
carries; its strings hold their translations while a render runs. Errors are panels drawn by the
formatter: their message, title and "Try ... for help" hint are translated where it writes them.
"""

from __future__ import annotations

import contextlib
import copy
import io
import sys
import threading
from typing import Any

from . import _api, _hooks, _hooks_click

# Strings of a RichHelpConfiguration that reach the screen. The framework's own are looked up exactly;
# the texts an application sets (header, footer, error epilogue) are translated like help.
_FRAMEWORK_STRINGS = (
    "arguments_panel_title",
    "options_panel_title",
    "commands_panel_title",
    "errors_panel_title",
    "aborted_text",
    "deprecated_string",
    "deprecated_with_reason_string",
    "default_string",
    "envvar_string",
    "required_short_string",
    "required_long_string",
    "range_string",
    "append_metavars_help_string",
    "append_range_help_string",
    "helptext_aliases_string",
)
_APP_STRINGS = ("header_text", "footer_text", "errors_suggestion", "errors_epilogue")
_SUGGESTION = "Try '{command} {option}' for help."  # Click's hint; rich-click assembles its own from fragments

_active: dict[int, list] = {}  # id(config) -> [renders in flight, saved (name, original) pairs, config]
_lock = threading.Lock()


def _escape(s: str) -> str:
    """Escapes what Rich would read as markup, as rich-click does for its own bracketed labels."""
    try:
        from rich.markup import escape

        return escape(s)
    except Exception:
        return "\\" + s if s.startswith("[") else s


def _translate_string(name: str, current: str) -> str:
    escaped = current.startswith("\\[")  # "\[default: {}]": the bracket is escaped for Rich markup
    source = current[1:] if escaped else current
    translated = _hooks_click._lookup(source) if name in _FRAMEWORK_STRINGS else _hooks_click._help(source)
    if translated == source:
        return current
    return _escape(translated) if escaped else translated


def _swap(config, extra: dict[str, Any]) -> list[tuple[str, Any]]:
    saved: list[tuple[str, Any]] = []
    for name in _FRAMEWORK_STRINGS + _APP_STRINGS:
        current = getattr(config, name, None)
        if not isinstance(current, str) or not current:
            continue
        try:
            translated = _translate_string(name, current)
        except Exception:
            _api.debug_exc("rich_click config." + name)
            continue
        if translated != current:
            saved.append((name, current))
            setattr(config, name, translated)
    for name, value in extra.items():
        saved.append((name, getattr(config, name, None)))
        setattr(config, name, value)
    return saved


@contextlib.contextmanager
def _translated_config(config, extra: dict[str, Any] | None = None):
    """Swaps the configuration's strings for their translations (plus ``extra`` values) for the duration
    of a render; the originals come back when the last concurrent render on it finishes."""
    if config is None:
        yield
        return
    key = id(config)
    with _lock:
        entry = _active.get(key)
        if entry is None:
            entry = _active[key] = [1, _swap(config, extra or {}), config]
        else:
            entry[0] += 1
    try:
        yield
    finally:
        with _lock:
            entry[0] -= 1
            if entry[0] == 0:
                del _active[key]
                for name, original in reversed(entry[1]):
                    try:
                        setattr(config, name, original)
                    except Exception:
                        pass


# -- help --------------------------------------------------------------------------------------


def _panel_view(panel: Any) -> Any:
    p = copy.copy(panel)
    for attr in ("name", "help"):
        val = getattr(panel, attr, None)
        if isinstance(val, str) and val:
            setattr(p, attr, _hooks_click._help(val))
    return p


def view(cmd: Any) -> Any:
    """A Click view of the command (see :func:`_hooks_click.view`) whose option and command panels have
    translated titles."""
    v = _hooks_click.view(cmd)
    panels = getattr(cmd, "panels", None)
    if v is not cmd and isinstance(panels, list) and panels:
        try:
            v.panels = [_panel_view(p) for p in panels]
        except Exception:
            _api.debug_exc("rich_click panels")
    return v


def _mark(formatter) -> tuple:
    """Where the formatter's console output stands, so a failed translated render can be discarded."""
    console = getattr(formatter, "console", None)
    buffer = getattr(console, "_record_buffer", None)
    file = getattr(console, "file", None)
    pos = None
    if isinstance(file, io.StringIO):
        try:
            pos = file.tell()
        except Exception:
            pos = None
    return (len(buffer) if isinstance(buffer, list) else None, file, pos)


def _rewind(formatter, mark: tuple) -> None:
    n, file, pos = mark
    buffer = getattr(getattr(formatter, "console", None), "_record_buffer", None)
    if n is not None and isinstance(buffer, list):
        del buffer[n:]
    if pos is not None:
        try:
            file.seek(pos)
            file.truncate()
        except Exception:
            pass


def _wrap_format_help(command_cls: type) -> None:
    orig = command_cls.__dict__.get("format_help")
    if orig is None:
        return

    def format_help(self, ctx, formatter):
        render = _hooks_click._render
        if _hooks_click._engine() is None or getattr(render, "active", False):
            return orig(self, ctx, formatter)
        render.active = True
        try:
            try:
                from . import _dump

                _dump.maybe_dump_click(ctx)
            except Exception:
                _api.debug_exc("dump")
            mark = _mark(formatter)
            real = ctx.command
            try:
                v = view(self)
                if real is self:
                    ctx.command = v
                with _translated_config(getattr(formatter, "config", None)):
                    return orig(v, ctx, formatter)
            except Exception:
                _api.debug_exc("rich_click help")
                ctx.command = real
                _rewind(formatter, mark)
                return orig(self, ctx, formatter)
            finally:
                ctx.command = real
        finally:
            render.active = False

    _hooks.patch(command_cls, "format_help", format_help)


# -- formatter: usage, errors, abort -----------------------------------------------------------


def _suggestion(exc, config) -> dict[str, Any]:
    """The translated "Try ... for help." hint as rich-click's ``errors_suggestion``, when the catalog
    has Click's sentence; rich-click would otherwise build the hint from fragments."""
    if config is None or getattr(config, "errors_suggestion", None) is not None:
        return {}
    ctx = getattr(exc, "ctx", None)
    if ctx is None:
        return {}
    try:
        if ctx.command.get_help_option(ctx) is None:
            return {}
        command, option = ctx.command_path, ctx.help_option_names[0]
    except Exception:
        return {}
    translated = _hooks_click._lookup(_SUGGESTION)
    if translated == _SUGGESTION:
        return {}
    try:
        return {"errors_suggestion": _escape(translated.format(command=command, option=option))}
    except Exception:
        return {}


def _wrap_formatter(formatter_cls: type, error_hook: bool) -> None:
    write_usage = formatter_cls.__dict__.get("write_usage")
    if write_usage is not None:

        def _write_usage(self, prog, args="", prefix=None):
            if prefix is None and _hooks_click._engine() is not None:
                prefix = _hooks_click._lookup("Usage:")
            return write_usage(self, prog, args, prefix)

        _hooks.patch(formatter_cls, "write_usage", _write_usage)
    write_error = formatter_cls.__dict__.get("write_error")
    if write_error is not None:

        def _write_error(self, exc):
            if _hooks_click._engine() is None:
                return write_error(self, exc)
            mark = _mark(self)
            try:
                config = getattr(self, "config", None)
                with _translated_config(config, _suggestion(exc, config)):
                    if not error_hook:
                        return write_error(self, exc)
                    with _hooks_click.translated_message(exc):
                        return write_error(self, exc)
            except Exception:
                _api.debug_exc("rich_click error")
                _rewind(self, mark)
                return write_error(self, exc)

        _hooks.patch(formatter_cls, "write_error", _write_error)
    write_abort = formatter_cls.__dict__.get("write_abort")
    if write_abort is not None:

        def _write_abort(self):
            if _hooks_click._engine() is None:
                return write_abort(self)
            try:
                with _translated_config(getattr(self, "config", None)):
                    return write_abort(self)
            except Exception:
                _api.debug_exc("rich_click abort")
                return write_abort(self)

        _hooks.patch(formatter_cls, "write_abort", _write_abort)


# -- install -----------------------------------------------------------------------------------


def install(*, error_hook: bool) -> None:
    """Hooks rich-click when the application has imported it."""
    if _hooks.installed("rich_click"):
        return
    commands = sys.modules.get("rich_click.rich_command")
    formatting = sys.modules.get("rich_click.rich_help_formatter")
    if commands is None or formatting is None:
        return
    _hooks.mark("rich_click")
    for name in ("RichCommand", "RichGroup", "RichCommandCollection"):
        cls = getattr(commands, name, None)
        if isinstance(cls, type):
            _wrap_format_help(cls)
    formatter_cls = getattr(formatting, "RichHelpFormatter", None)
    if isinstance(formatter_cls, type):
        _wrap_formatter(formatter_cls, error_hook)
    # "Show this message and exit." and "(dynamic)" go through gettext in these modules.
    for name in ("rich_click.decorators", "rich_click.rich_help_rendering", "rich_click.cli"):
        _hooks.on_import(name, _hooks_click._patch_gettext)
    # rich_click re-exports Click's prompt and confirm under its own name; the Click hook (installed
    # first) wrapped the originals in click.termui, so the re-exports get the same wrappers.
    root, termui = sys.modules.get("rich_click"), sys.modules.get("click.termui")
    for name in ("prompt", "confirm"):
        wrapper = getattr(termui, name, None)
        original = getattr(wrapper, "__wrapped__", None)
        if original is not None:
            _hooks.patch(root, name, wrapper, only_if=original)
