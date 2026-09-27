# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""LOCALIZER_DUMP: every help string of the command tree, with whether it was translated, written once
per process as JSON in the same shape the Go runtime writes (for coverage reports)."""

from __future__ import annotations

import json

from . import _api
from ._engine import Mode

_done: set[str] = set()


def _write(entries: list[dict]) -> None:
    st = _api.state()
    if st is None or not st.dump:
        return
    doc = {"language": st.lang, "entries": entries}
    with open(st.dump, "w", encoding="utf-8") as f:
        json.dump(doc, f, ensure_ascii=False, indent=2)
        f.write("\n")


def _entry(entries: list[dict], kind: str, command: str, text, flag: str = "") -> None:
    st = _api.state()
    if not isinstance(text, str) or not text.strip():
        return
    e = {"kind": kind, "command": command, "text": text}
    if flag:
        e["flag"] = flag
    if st is not None:
        e["translated"] = st.engine.translate(text, Mode.HELP) != text
    entries.append(e)


def maybe_dump_click(ctx) -> None:
    """Dumps the tree reachable from the root of a Click context, the first time help renders."""
    st = _api.state()
    if st is None or not st.dump or "click" in _done:
        return
    _done.add("click")
    root_ctx = ctx.find_root()
    entries: list[dict] = []
    _walk_click(root_ctx.command, root_ctx, entries)
    _write(entries)


def _walk_click(cmd, ctx, entries: list[dict]) -> None:
    path = ctx.command_path
    _entry(entries, "short", path, getattr(cmd, "short_help", None) or _first_paragraph(getattr(cmd, "help", None)))
    _entry(entries, "long", path, getattr(cmd, "help", None))
    _entry(entries, "epilog", path, getattr(cmd, "epilog", None))
    if isinstance(getattr(cmd, "deprecated", None), str):
        _entry(entries, "deprecated", path, cmd.deprecated)
    if isinstance(getattr(cmd, "rich_help_panel", None), str):
        _entry(entries, "group", path, cmd.rich_help_panel)
    seen = set()
    try:
        params = cmd.get_params(ctx)
    except Exception:
        params = getattr(cmd, "params", [])
    for p in params:
        name = getattr(p, "name", "") or ""
        if name in seen:
            continue
        seen.add(name)
        _entry(entries, "flag", path, getattr(p, "help", None), name)
        if isinstance(getattr(p, "rich_help_panel", None), str):
            _entry(entries, "group", path, p.rich_help_panel, name)
    if hasattr(cmd, "list_commands") and hasattr(cmd, "get_command"):
        for name in cmd.list_commands(ctx):
            sub = cmd.get_command(ctx, name)
            if sub is None:
                continue
            try:
                sub_ctx = type(ctx)(sub, info_name=name, parent=ctx)
            except Exception:
                continue
            _walk_click(sub, sub_ctx, entries)


def _first_paragraph(text):
    if not isinstance(text, str):
        return None
    return text.strip().split("\n\n", 1)[0]


def maybe_dump_argparse(parser) -> None:
    st = _api.state()
    if st is None or not st.dump or "argparse" in _done:
        return
    _done.add("argparse")
    entries: list[dict] = []
    _walk_argparse(parser, parser.prog, entries)
    _write(entries)


def _walk_argparse(parser, path: str, entries: list[dict]) -> None:
    import argparse

    _entry(entries, "long", path, parser.description)
    _entry(entries, "epilog", path, parser.epilog)
    for group in getattr(parser, "_action_groups", ()):
        _entry(entries, "group", path, group.title)
    for action in getattr(parser, "_actions", ()):
        if action.help is argparse.SUPPRESS:
            continue
        flag = action.option_strings[0] if action.option_strings else action.dest
        _entry(entries, "flag", path, action.help, flag)
        choices = getattr(action, "choices", None)
        if isinstance(action, argparse._SubParsersAction) and isinstance(choices, dict):
            for name, sub in choices.items():
                _walk_argparse(sub, f"{path} {name}", entries)
