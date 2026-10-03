# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""The public API: language selection, the engine, and the helpers a CLI calls at its output points.

Everything here fails open: an internal error leaves the CLI in English and never raises into the
host program.
"""

from __future__ import annotations

import itertools
import os
import string
import sys
import traceback
from collections.abc import Callable, Sequence
from dataclasses import dataclass

from . import _catalog, _encoding, _locale, builtin
from ._catalog import Traversable
from ._engine import Engine, Mode

__all__ = ["ENV_LANG", "Mode", "State", "state", "init", "localize", "t", "tf", "error", "translate", "lang", "uninstall"]

ENV_LANG = "LOCALIZER_LANG"


@dataclass
class State:
    engine: Engine
    lang: str
    debug: bool
    dump: str  # LOCALIZER_DUMP path or ""


_state: State | None = None
_hooks_installed = False


def state() -> State | None:
    """The active state, or None when output stays in English."""
    return _state


def debug_enabled() -> bool:
    return (os.environ.get("LOCALIZER_DEBUG") or "").strip().lower() in ("1", "true", "yes", "on")


def debugf(msg: str) -> None:
    if _state is not None and _state.debug or debug_enabled():
        print("localizer: " + msg, file=sys.stderr)


def debug_exc(where: str) -> None:
    """Reports a swallowed exception when LOCALIZER_DEBUG is on."""
    if debug_enabled():
        print(f"localizer: {where}: {traceback.format_exc().strip().splitlines()[-1]}", file=sys.stderr)


_catalog_cache: dict[tuple[object, str], dict[str, str]] = {}


def _load_messages(root: Traversable, lang: str) -> dict[str, str]:
    key = (str(root), lang)
    msgs = _catalog_cache.get(key)
    if msgs is None:
        msgs = _catalog.load(root, lang).messages
        _catalog_cache[key] = msgs
    return msgs


def _setup(locales, env_var: str | Sequence[str] | None, language: str | None) -> State | None:
    override = [ENV_LANG]
    if isinstance(env_var, str):
        override.insert(0, env_var)
    elif env_var:
        override = [*env_var, ENV_LANG]
    if language is not None and language.strip():
        res = _locale.detect(["_forced"], lambda _name: language)
    else:  # None or "" (an unset --lang option, say): detect
        res = _locale.detect(override)
    if res.off:
        return None
    root = _catalog.resolve(locales)
    available = _catalog.languages(root)
    debug = debug_enabled()
    dump = os.environ.get("LOCALIZER_DUMP") or ""
    if res.pseudo:
        from . import _format

        if _unencodable("qps", [_format.pseudo(string.ascii_letters + string.digits)]):
            return None
        catalogs = [_load_messages(root, l) for l in available]
        catalogs.extend(builtin.messages(l) for l in builtin.languages())
        eng = Engine.pseudo(*catalogs)
        return State(eng, "qps", debug, dump)
    if not available:
        return None
    lang = _locale.match(res.tags, available)
    if not lang:
        return None
    try:
        app = _load_messages(root, lang)
    except Exception:
        debug_exc(f"loading the {lang} catalog")
        return None
    builtin_msgs = builtin.messages(lang)
    if _unencodable(lang, itertools.chain(builtin_msgs.values(), app.values())):
        return None
    eng = Engine(lang, builtin_msgs, app)
    if debug:

        def on_miss(s: str, _mode: Mode) -> None:
            first = s.strip().splitlines()[0] if s.strip() else s
            print(f"localizer: untranslated ({lang}): {first!r}", file=sys.stderr)

        eng.on_miss = on_miss
    return State(eng, lang, debug, dump)


def _unencodable(lang: str, texts) -> bool:
    """Whether a standard stream could not show the language's text, in which case output stays in
    English: a translated help line would otherwise raise UnicodeEncodeError or print escapes (on
    Windows, redirected output uses the ANSI code page unless PYTHONUTF8=1)."""
    where = _encoding.unencodable_stream(texts)
    if where:
        debugf(f"{where} cannot encode the {lang} catalog; output stays in English (set PYTHONUTF8=1 to allow it)")
        return True
    return False


def init(locales, *, env_var: str | Sequence[str] | None = None, language: str | None = None) -> str:
    """Detects the user's language and loads the matching catalog from ``locales`` (a package name such
    as "yourcli.locales", a directory, or a Traversable) for :func:`t`, :func:`tf`, :func:`error` and
    :func:`translate`. Returns the selected language, or "" when output stays in English. Never raises.

    ``env_var`` names an application-specific override variable (or several), consulted before
    ``LOCALIZER_LANG``; ``language`` forces a language ("en" or "off" disables localization).
    """
    global _state
    try:
        _state = _setup(locales, env_var, language)
    except Exception:
        debug_exc("init")
        _state = None
    return _state.lang if _state else ""


def localize(app, locales, *, env_var: str | Sequence[str] | None = None, language: str | None = None,
             without_error_hook: bool = False, without_prompt_hook: bool = False) -> str:
    """Localizes a Typer app, a Click command or an argparse parser: its help text, its framework's
    own messages, the errors it prints and the prompts it shows, plus whatever the program passes
    through :func:`t`. Call it after all commands and options are registered, right before the app
    runs. Returns the selected language ("" for English). Never raises."""
    global _hooks_installed
    lang = init(locales, env_var=env_var, language=language)
    if not _state:
        return ""
    try:
        from . import _hooks

        _hooks.install(app, error_hook=not without_error_hook, prompt_hook=not without_prompt_hook)
        _hooks_installed = True
    except Exception:
        debug_exc("installing hooks")
    return lang


def uninstall() -> None:
    """Removes every hook and forgets the language (for tests)."""
    global _state, _hooks_installed
    _state = None
    try:
        from . import _dump

        _dump.reset()
    except Exception:
        debug_exc("uninstall")
    if _hooks_installed:
        try:
            from . import _hooks

            _hooks.uninstall()
        except Exception:
            debug_exc("uninstall")
        _hooks_installed = False


def lang() -> str:
    """The active language tag ("qps" for pseudo-localization), or "" when output is English."""
    return _state.lang if _state else ""


def t(s: str) -> str:
    """The translation of ``s``, or ``s`` unchanged. A format string is looked up exactly, so call
    ``t`` before formatting: ``t("Created {name}").format(name=n)`` (or use :func:`tf`)."""
    st = _state
    if st is None or not isinstance(s, str) or s == "":
        return s
    try:
        from . import _format

        if _format.has_fields(s):
            return st.engine.lookup(s)[0]
        return st.engine.translate(s, Mode.OUTPUT)
    except Exception:
        debug_exc("t")
        return s


def tf(fmt: str, *args, **kwargs) -> str:
    """``str.format`` with a translated format string; falls back to the English format if the
    translation cannot be formatted with these arguments."""
    tr = t(fmt)
    try:
        return tr.format(*args, **kwargs)
    except Exception:
        return fmt.format(*args, **kwargs)


def translate(s: str, mode: Mode = Mode.OUTPUT) -> str:
    """Translates text the CLI is about to print, splitting composite text as ``mode`` allows. Use it
    at output chokepoints that receive already formatted messages."""
    st = _state
    if st is None or not isinstance(s, str) or s == "":
        return s
    try:
        return st.engine.translate(s, mode)
    except Exception:
        debug_exc("translate")
        return s


def error(exc) -> str:
    """The message of an exception (or a string) translated for display; the CLI's own parts of a
    message are translated, text from servers and libraries stays as it is."""
    try:
        msg = exc.format_message() if hasattr(exc, "format_message") else str(exc)
    except Exception:
        msg = str(exc)
    return translate(msg, Mode.ERROR)
