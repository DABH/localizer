# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""The lookup engine: exact catalog hits, reverse-matched templates, and composite text split into
paragraphs, lines and "label: message" parts. A line-by-line port of the Go ``engine`` package; the
shared vectors in testdata/conformance/engine.json pin the two down.
"""

from __future__ import annotations

import enum
import re
import threading
import unicodedata
from collections.abc import Callable, Mapping

from . import _format as fmt

__all__ = ["Mode", "Engine"]


class Mode(enum.IntEnum):
    """How aggressively composite strings are split."""

    OUTPUT = 0  # general program output; never splits "label: rest" (could be data)
    HELP = 1  # help text (all CLI-authored), including "DEPRECATED: ..." prefixes
    ERROR = 2  # error messages: like HELP, plus wrapped-error chains


_MAX_DEPTH = 8
_MAX_MEMO_BYTES = 1 << 20  # total size of memoized inputs and outputs
_MAX_MEMO_ENTRY = 1024  # longer strings are looked up every time rather than kept
_IDX_LEN = 4
_MAX_PREPASS = 64 << 10  # strings longer than this are not split
_MAX_MATCH = 8 << 10  # longer strings never go through a template's regular expression

_SENTENCE_BREAK = re.compile("[.!?。！？][\t\n\f\r ]*")


def _sentence_break(s: str) -> bool:
    """A sentence-ending mark followed by an uppercase letter (Go: ``[.!?。！？]\\s*\\p{Lu}``)."""
    for m in _SENTENCE_BREAK.finditer(s):
        end = m.end()
        if end < len(s) and unicodedata.category(s[end]) == "Lu":
            return True
    return False


def _is_lower(c: str) -> bool:
    return unicodedata.category(c) == "Ll"


def _ends_sentence(s: str) -> bool:
    return s != "" and s[-1] in ".!?。！？"


def _identifier_token(s: str) -> bool:
    """A single token of letters, digits and ``-_./:``: a command name, a flag name, a path; never a message."""
    return s != "" and all(c.isalnum() or c in "-_./:" for c in s)


def _identifier_line(line: str) -> bool:
    """An indented line holding a single identifier-like token, such as the command names Click lists
    under "Did you mean this?"."""
    return bool(line) and line[0] in "\t " and _identifier_token(line.strip(" \t\r\n"))


def _trimmed_keys(c: Mapping[str, str]) -> dict[str, str]:
    """Entries whose key carries surrounding whitespace, under the trimmed key. An exact trimmed key wins
    over any copy; among several keys trimming to the same text the smallest original key wins."""
    fixed: dict[str, str] = {}
    origin: dict[str, str] = {}
    for k, v in c.items():
        _, core, _ = fmt.split_space(k)
        if core == k or core in c:
            continue
        prev = origin.get(core)
        if prev is not None and prev < k:
            continue
        origin[core] = k
        fixed[core] = v
    return fixed


class Engine:
    """Translates strings into one language. Safe for concurrent use."""

    def __init__(self, lang: str, *catalogs: Mapping[str, str]) -> None:
        """Later catalogs override earlier ones: pass the built-in catalog first, the app's last. The last
        catalog is the application's: its templates win ties against the built-in ones, and a lone
        identifier captured by a built-in template (a command or flag name) is never translated again."""
        self.lang = lang
        self._pseudo = False
        self.on_miss: Callable[[str, Mode], None] | None = None
        # Layers are consulted last to first. Keys are normally trimmed; untrimmed ones get a "fixed"
        # layer so the app's catalog never has to be copied.
        self._layers: list[Mapping[str, str]] = []
        self._app_from = 0
        for i, c in enumerate(catalogs):
            if i == len(catalogs) - 1:
                self._app_from = len(self._layers)
            if not c:
                continue
            self._layers.append(c)
            fixed = _trimmed_keys(c)
            if fixed:
                self._layers.append(fixed)
        self._patterns: list[fmt.Pattern] | None = None
        self._pat_layer: list[int] = []
        self._multi: list[int] = []
        self._index: dict[bytes, list[int]] = {}
        self._wild: list[int] = []
        self._valid: dict[str, bool] = {}
        self._memo: list[dict[str, str]] = [{}, {}, {}]
        self._memo_size = 0
        self._translations: set[str] | None = None
        self._lock = threading.Lock()

    @classmethod
    def pseudo(cls, *catalogs: Mapping[str, str]) -> Engine:
        """A pseudo-localizing engine that knows the given source strings."""
        known: dict[str, str] = {}
        for c in catalogs:
            for k in c:
                _, core, _ = fmt.split_space(k)
                if core:
                    known[core] = core
        e = cls("qps")
        e._pseudo = True
        e._layers = [known]
        return e

    # -- exact lookups -------------------------------------------------------------------------

    def _raw(self, core: str) -> tuple[str, bool]:
        for layer in reversed(self._layers):
            if core in layer:
                _, tcore, _ = fmt.split_space(layer[core])
                return tcore, tcore != ""
        return "", False

    def lookup(self, s: str) -> tuple[str, bool]:
        """Translates ``s`` by exact match only; surrounding whitespace is preserved."""
        lead, core, trail = fmt.split_space(s)
        if core == "":
            return s, False
        t, ok = self._exact_hit(core)
        if not ok:
            return s, False
        return lead + t + trail, True

    def _exact_hit(self, core: str) -> tuple[str, bool]:
        t, ok = self._raw(core)
        if not ok:
            return "", False
        if self._pseudo:
            return fmt.pseudo(core), True
        if not self._is_valid(core, t):
            return "", False
        return t, True

    def is_translation(self, s: str) -> bool:
        """Whether ``s`` is one of the catalogs' translations: text that was already translated once and
        reaches the engine again (a framework re-rendering a translated help string) is not a miss."""
        if self._translations is None:
            with self._lock:
                if self._translations is None:
                    values: set[str] = set()
                    for layer in self._layers:
                        for v in layer.values():
                            _, core, _ = fmt.split_space(v)
                            if core:
                                values.add(core)
                    self._translations = values
        _, core, _ = fmt.split_space(s)
        return core in self._translations

    def _is_valid(self, src: str, tr: str) -> bool:
        """A translation must keep the source's placeholders and add no control characters; invalid
        entries behave as if missing, so the CLI falls back to English."""
        v = self._valid.get(src)
        if v is None:
            v = fmt.extract(src) == fmt.extract(tr) and not fmt.new_control_chars(src, tr)
            self._valid[src] = v
        return v

    # -- translation ---------------------------------------------------------------------------

    def translate(self, s: str, mode: Mode) -> str:
        """Returns the translation of ``s``, or ``s`` itself when nothing in the catalog applies."""
        if s == "" or mode not in (Mode.OUTPUT, Mode.HELP, Mode.ERROR):
            return s
        # Help strings are rendered once each; memoizing them would only cost memory.
        cacheable = len(s) <= _MAX_MEMO_ENTRY and mode != Mode.HELP
        if cacheable:
            v = self._memo[mode].get(s)
            if v is not None:
                return v
        out = self._translate(s, mode, 0)
        if out == s and self.on_miss is not None and mode == Mode.HELP:
            _, core, _ = fmt.split_space(s)
            if fmt.has_letter(core) and not self.is_translation(core):
                self.on_miss(s, mode)
        if cacheable:
            with self._lock:
                if self._memo_size < _MAX_MEMO_BYTES and s not in self._memo[mode]:
                    self._memo[mode][s] = out
                    self._memo_size += len(s) + len(out)
        return out

    def _translate(self, s: str, mode: Mode, depth: int) -> str:
        if depth > _MAX_DEPTH:
            return s
        lead, core, trail = fmt.split_space(s)
        if core == "" or not fmt.has_letter(core):
            return s
        t, ok = self._exact_hit(core)
        if ok:
            return lead + t + trail
        multiline = "\n" in core
        # Multi-line help is almost always composed (examples, code blocks): try its parts before any
        # single-line template, which saves regex work. Templates that span lines themselves are tried
        # first, or their first line would match a one-line template and capture the rest as a value.
        if multiline and mode == Mode.HELP and len(core) <= _MAX_PREPASS:
            t, ok = self._multiline_hit(core, mode, depth)
            if ok:
                return lead + t + trail
            t, ok = self._segments(core, mode, depth)
            if ok:
                return lead + t + trail
        t, ok = self._pattern_hit(core, mode, depth)
        if ok:
            return lead + t + trail
        if len(core) > _MAX_PREPASS:
            return s
        if multiline and mode != Mode.HELP:
            t, ok = self._segments(core, mode, depth)
            if ok:
                return lead + t + trail
            return s
        if multiline:
            return s
        if mode != Mode.OUTPUT:
            t, ok = self._label_split(core, mode, depth)
            if ok:
                return lead + t + trail
        return s

    def _segments(self, core: str, mode: Mode, depth: int) -> tuple[str, bool]:
        """Paragraphs, or lines when there is a single paragraph."""
        if "\n\n" in core:
            return self._join_parts(core.split("\n\n"), "\n\n", mode, depth)
        return self._join_parts(core.split("\n"), "\n", mode, depth)

    def _join_parts(self, parts: list[str], sep: str, mode: Mode, depth: int) -> tuple[str, bool]:
        changed = False
        for i, p in enumerate(parts):
            if mode == Mode.HELP and p.strip(" \t\r\n").startswith("$ "):
                continue  # a shell example
            if mode == Mode.ERROR and _identifier_line(p):
                continue  # a suggested command or flag name, never a message
            np = self._translate(p, mode, depth + 1)
            if np != p:
                parts[i] = np
                changed = True
        return sep.join(parts), changed

    def _label_split(self, core: str, mode: Mode, depth: int) -> tuple[str, bool]:
        """"Label: rest": the label (with or without its colon) and the rest translate independently."""
        i = core.find(": ")
        if i <= 0:
            return "", False
        label, rest = core[: i + 1], core[i + 2 :]
        new_label = label
        t, ok = self._exact_hit(label)
        if ok:
            new_label = t
        else:
            t, ok = self._exact_hit(label[:-1])
            if ok:
                new_label = t + ":"
            else:
                t, ok = self._pattern_hit(label[:-1], mode, depth)
                if ok:
                    new_label = t + ":"
        new_rest = self._translate(rest, mode, depth + 1)
        if new_label == label and new_rest == rest:
            return "", False
        sep = "" if new_label.endswith("：") else " "  # a full-width colon carries its own spacing
        return new_label + sep + new_rest, True

    # -- patterns ------------------------------------------------------------------------------

    def _build_patterns(self) -> list[fmt.Pattern]:
        with self._lock:
            if self._patterns is not None:
                return self._patterns
            layer_of: dict[str, int] = {}  # template -> the highest layer holding it
            for i, layer in enumerate(self._layers):
                for k in layer:
                    if "%" in k or "{" in k:
                        _, core, _ = fmt.split_space(k)
                        layer_of[core] = i
            patterns: list[fmt.Pattern] = []
            for k in sorted(layer_of, key=lambda x: x.encode("utf-8")):  # deterministic, as Go sorts
                p = fmt.compile(k)
                if p is None:
                    continue
                i = len(patterns)
                patterns.append(p)
                self._pat_layer.append(layer_of[k])
                if "\n" in k:
                    self._multi.append(i)
                pre = p.prefix
                if pre == "":
                    self._wild.append(i)
                    continue
                pre = pre.encode("utf-8")[:_IDX_LEN]
                self._index.setdefault(pre, []).append(i)
            self._patterns = patterns
            return patterns

    def _pattern_hit(self, core: str, mode: Mode, depth: int) -> tuple[str, bool]:
        patterns = self._patterns if self._patterns is not None else self._build_patterns()
        if not patterns or len(core) > _MAX_MATCH:
            return "", False
        b = core.encode("utf-8")
        candidates: list[int] = []
        for n in range(1, min(_IDX_LEN, len(b)) + 1):
            candidates.extend(self._index.get(b[:n], ()))
        candidates.extend(self._wild)
        return self._best_hit(candidates, core, mode, depth)

    def _multiline_hit(self, core: str, mode: Mode, depth: int) -> tuple[str, bool]:
        """Tries only the templates that span lines."""
        if self._patterns is None:
            self._build_patterns()
        if len(core) > _MAX_MATCH:
            return "", False
        return self._best_hit(self._multi, core, mode, depth)

    def _best_hit(self, candidates: list[int], core: str, mode: Mode, depth: int) -> tuple[str, bool]:
        """The best reverse match among the candidate templates: more literal text wins; at equal
        specificity the application's catalog beats the built-in one, then the first candidate (templates
        are sorted). Text captures are translated again outside output mode, except a lone identifier
        captured by a built-in template (a command or flag name, never a message)."""
        patterns = self._patterns or []
        best: fmt.Pattern | None = None
        best_layer = -1
        best_captures: dict[str, str] = {}
        best_classes: dict[str, str] = {}
        for i in candidates:
            p = patterns[i]
            layer = self._pat_layer[i]
            if best is not None and (p.specificity < best.specificity or (p.specificity == best.specificity and layer <= best_layer)):
                continue
            m = p.match(core)
            if m is not None and _plausible(p, m[0], m[1], mode):
                best, best_layer, best_captures, best_classes = p, layer, m[0], m[1]
        if best is None:
            return "", False
        if self._pseudo:
            translation = fmt.pseudo(best.format)
        else:
            translation, ok = self._exact_hit(best.format)
            if not ok:
                return "", False
        builtin = best_layer < self._app_from

        def transform(name: str, text: str) -> str:
            if best_classes.get(name) == "v" and mode != Mode.OUTPUT and not (builtin and _identifier_token(text)):
                return self._translate(text, mode, depth + 1)
            return text

        out = best.splice(translation, best_captures, transform)
        if out is None:
            return "", False
        return out, True


def _plausible(p: fmt.Pattern, captures: dict[str, str], classes: dict[str, str], mode: Mode) -> bool:
    """Rejects matches that would splice untranslated prose into a translation: a generic template
    (little literal text) must not capture a lowercase phrase, and outside error messages a capture must
    not span sentences or lines, nor, in help text, end a sentence. Captures in error messages may span
    lines and sentences: they legitimately carry wrapped errors and server text."""
    generic = p.letters < 10
    for c in captures.values():
        if generic and " " in c and c and _is_lower(c[0]):
            return False
        if mode == Mode.ERROR:
            continue
        if "\n" in c or _sentence_break(c):
            return False
        if mode == Mode.HELP and " " in c and _ends_sentence(c):
            return False
    return True
