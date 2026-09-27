# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Hooks for argparse: help and usage are rendered with the parser's texts translated for the duration
of the call and restored afterwards; errors are translated where argparse reports them."""

from __future__ import annotations

import contextlib
import gettext

from . import _api, _hooks
from ._engine import Mode


def _engine():
    st = _api.state()
    return st.engine if st is not None else None


def _lookup(s: str) -> str:
    eng = _engine()
    return eng.lookup(s)[0] if eng is not None and isinstance(s, str) else s


@contextlib.contextmanager
def translated_parser(parser, eng):
    import argparse

    saved = []

    def swap(obj, attr):
        val = getattr(obj, attr, None)
        if isinstance(val, str) and val and val is not argparse.SUPPRESS:
            saved.append((obj, attr, val))
            try:
                setattr(obj, attr, eng.translate(val, Mode.HELP))
            except Exception:
                saved.pop()

    for attr in ("description", "epilog", "usage"):
        swap(parser, attr)
    for group in getattr(parser, "_action_groups", ()):
        swap(group, "title")
        swap(group, "description")
    for action in getattr(parser, "_actions", ()):
        swap(action, "help")
        for sub in getattr(action, "_choices_actions", ()):
            swap(sub, "help")
    try:
        yield
    finally:
        for obj, attr, val in reversed(saved):
            setattr(obj, attr, val)


def install(app, *, error_hook: bool) -> None:
    import argparse

    from . import _dump

    if app is not None and isinstance(app, argparse.ArgumentParser):
        try:
            _dump.maybe_dump_argparse(app)
        except Exception:
            _api.debug_exc("dump")
    if _hooks.installed("argparse"):
        return
    _hooks.mark("argparse")
    if getattr(argparse, "_", None) is gettext.gettext:  # getattr: 3.15 binds it lazily
        _hooks.patch(argparse, "_", lambda s: _lookup(s))
    if getattr(argparse, "ngettext", None) is gettext.ngettext:
        _hooks.patch(argparse, "ngettext", lambda s, p, n: _lookup(s if n == 1 else p))
    parser_cls = argparse.ArgumentParser
    for name in ("format_help", "format_usage"):
        orig = parser_cls.__dict__.get(name)
        if orig is None:
            continue

        def make(orig=orig):
            def wrapped(self):
                eng = _engine()
                if eng is None:
                    return orig(self)
                try:
                    with translated_parser(self, eng):
                        return orig(self)
                except Exception:
                    _api.debug_exc("argparse help")
                    return orig(self)

            return wrapped

        _hooks.patch(parser_cls, name, make())
    if error_hook:
        orig_error = parser_cls.__dict__.get("error")
        if orig_error is not None:

            def error(self, message):
                eng = _engine()
                if eng is not None and isinstance(message, str):
                    try:
                        message = eng.translate(message, Mode.ERROR)
                    except Exception:
                        _api.debug_exc("argparse error")
                return orig_error(self, message)

            _hooks.patch(parser_cls, "error", error)
    # Python 3.10-3.12 hard-code the defaults suffix; 3.13+ pass it through gettext.
    defaults_cls = getattr(argparse, "ArgumentDefaultsHelpFormatter", None)
    orig_help_string = defaults_cls.__dict__.get("_get_help_string") if defaults_cls is not None else None
    if orig_help_string is not None:
        suffix = " (default: %(default)s)"

        def _get_help_string(self, action):
            out = orig_help_string(self, action)
            if isinstance(out, str) and out.endswith(suffix) and _engine() is not None:
                tr = _lookup(suffix)
                if tr != suffix:
                    out = out[: -len(suffix)] + tr
            return out

        _hooks.patch(defaults_cls, "_get_help_string", _get_help_string)
