# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""The placeholder grammar of Python CLI strings.

This is the Python side of the grammar specified in testdata/conformance/README.md and implemented in
Go by the ``msgfmt`` package: str.format fields and printf-style verbs, Rich markup tags, backquoted
spans and control characters, plus reverse matching of already formatted text against a template.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field

__all__ = [
    "Token",
    "Placeholders",
    "Pattern",
    "parse",
    "has_fields",
    "extract",
    "compile",
    "new_control_chars",
    "split_space",
    "has_letter",
    "pseudo",
]


@dataclass(frozen=True)
class Token:
    """One piece of a parsed template: literal text, or a field.

    ``verb`` is the field's capture class: ``""`` for literal text, ``"v"`` text, ``"q"`` repr, ``"d"``
    integer, ``"f"`` float, ``"c"`` character. ``name`` is the field's identity (a name, a number, or the
    position of an auto-numbered ``{}``), ``conv`` its conversion (``r``/``s``/``a`` for ``{x!r}`` or the
    printf conversion character) and ``spec`` its format spec (the text after ``:``, or the printf flags,
    width, precision and length). ``arg`` numbers distinct identities from 1 in order of appearance.
    """

    lit: str = ""
    verb: str = ""
    arg: int = 0
    name: str = ""
    conv: str = ""
    spec: str = ""
    raw: str = ""
    auto: bool = False
    flags: str = ""
    width: str = ""
    prec: str = ""

    def is_verb(self) -> bool:
        return self.verb != ""

    def signature(self) -> str:
        """What a translation must reproduce for this field."""
        if self.raw.startswith("%"):
            if len(self.raw) > 1 and self.raw[1] == "(":
                return "%(" + self.name + ")" + self.spec + self.conv
            return "%" + self.name + ":" + self.spec + self.conv
        out = "{" + self.name
        if self.conv:
            out += "!" + self.conv
        if self.spec:
            out += ":" + self.spec
        return out + "}"


class _BraceError(Exception):
    """An unterminated "{" or a lone "}"."""


class _TemplateError(Exception):
    """A field that str.format would reject."""


_PRINTF_CONVERSIONS = "diouxXeEfFgGcrsa"


def parse(s: str) -> tuple[list[Token], bool]:
    """Tokenizes ``s``. Brace fields win: a string with at least one is a str.format template (``{{`` and
    ``}}`` are literal braces, ``%`` is plain text). Otherwise it is a printf template when the whole
    string parses as one with at least one verb; a bare space-flag verb (``50% of``) is plain text.
    Anything else is a single literal token. The flag is False for templates that cannot be used.
    """
    try:
        toks, n = _parse_braces(s)
    except _TemplateError:
        return [], False
    except _BraceError as e:
        if e.args[0] > 0:  # fields were found before the broken brace
            return [], False
    else:
        if n > 0:
            return _number_fields(toks), True
    toks, n, ok = _parse_printf(s)
    if not ok:
        return toks, False
    if n == 0:
        return [Token(lit=s)], True
    return _number_fields(toks), True


def _parse_braces(s: str) -> tuple[list[Token], int]:
    toks: list[Token] = []
    lit: list[str] = []
    n = 0
    auto = 0
    manual = False

    def flush() -> None:
        if lit:
            toks.append(Token(lit="".join(lit)))
            lit.clear()

    i = 0
    length = len(s)
    while i < length:
        c = s[i]
        if c == "{":
            if i + 1 < length and s[i + 1] == "{":
                lit.append("{")
                i += 2
                continue
            end = _brace_end(s, i + 1)
            if end < 0:
                raise _BraceError(n)
            tok = _parse_field(s[i + 1 : end], s[i : end + 1])
            if tok.name == "":
                tok = _replace(tok, auto=True, name=str(auto))
                auto += 1
            elif _arg_name(tok.name).isdigit():
                manual = True
            if auto > 0 and manual:
                raise _TemplateError  # str.format refuses to mix "{}" and "{0}"
            flush()
            toks.append(tok)
            n += 1
            i = end + 1
        elif c == "}":
            if i + 1 < length and s[i + 1] == "}":
                lit.append("}")
                i += 2
                continue
            raise _BraceError(n)
        else:
            lit.append(c)
            i += 1
    flush()
    return toks, n


def _brace_end(s: str, i: int) -> int:
    depth = 1
    for j in range(i, len(s)):
        c = s[j]
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return j
    return -1


def _parse_field(body: str, raw: str) -> Token:
    i = 0
    depth = 0
    while i < len(body):
        c = body[i]
        if c == "[":
            depth += 1
        elif c == "]" and depth > 0:
            depth -= 1
        elif depth == 0 and c in "!:":
            break
        i += 1
    name, rest = body[:i], body[i:]
    conv = ""
    if rest.startswith("!"):
        if len(rest) < 2 or rest[1] not in "rsa":
            raise _TemplateError
        conv = rest[1]
        rest = rest[2:]
        if rest and rest[0] != ":":
            raise _TemplateError
    spec = rest[1:] if rest.startswith(":") else ""
    return Token(verb=_brace_class(conv, spec), name=name, conv=conv, spec=spec, raw=raw)


def _arg_name(name: str) -> str:
    for i, c in enumerate(name):
        if c in ".[":
            return name[:i]
    return name


def _brace_class(conv: str, spec: str) -> str:
    if conv in ("r", "a"):
        return "q"
    if not spec or spec.endswith("}"):
        return "v"
    last = spec[-1]
    if last in "bdnoxX":
        return "d"
    if last in "eEfFgG%":
        return "f"
    if last == "c":
        return "c"
    return "v"


_SPEC_WIDTH = re.compile(r"^(?:.?[<>=^])?[-+ ]?z?#?0?[0-9]+")


def _spec_padded(spec: str) -> bool:
    return "{" in spec or _SPEC_WIDTH.match(spec) is not None


def _parse_printf(s: str) -> tuple[list[Token], int, bool]:
    toks: list[Token] = []
    lit: list[str] = []
    n = 0
    ok = True
    pos = 0
    named = positional = False

    def flush() -> None:
        if lit:
            toks.append(Token(lit="".join(lit)))
            lit.clear()

    i = 0
    length = len(s)
    while i < length:
        if s[i] != "%":
            j = s.find("%", i)
            if j < 0:
                lit.append(s[i:])
                break
            lit.append(s[i:j])
            i = j
            continue
        start = i
        i += 1
        if i >= length:
            lit.append("%")
            ok = False
            break
        if s[i] == "%":
            lit.append("%")
            i += 1
            continue
        name = ""
        if s[i] == "(":
            end = s.find(")", i)
            if end < 0:
                lit.append(s[start:])
                ok = False
                break
            name = s[i + 1 : end]
            i = end + 1
        f_start = i
        while i < length and s[i] in "#0- +":
            i += 1
        flags = s[f_start:i]
        w_start = i
        if i < length and s[i] == "*":
            ok = False
            i += 1
        else:
            while i < length and s[i].isdigit() and s[i].isascii():
                i += 1
        width = s[w_start:i]
        prec = ""
        if i < length and s[i] == ".":
            p_start = i
            i += 1
            if i < length and s[i] == "*":
                ok = False
                i += 1
            else:
                while i < length and s[i].isdigit() and s[i].isascii():
                    i += 1
            prec = s[p_start:i]
        mod = ""
        if i < length and s[i] in "hlL":
            mod = s[i]
            i += 1
        if i >= length:
            lit.append(s[start:])
            ok = False
            break
        conv = s[i]
        i += 1
        if name == "" and flags == " " and width == "" and prec == "" and mod == "":
            lit.append(s[start:i])  # "50% of": prose, not a space-flag verb
            continue
        if conv not in _PRINTF_CONVERSIONS:
            lit.append(s[start:i])
            ok = False
            continue
        flush()
        tok = Token(
            verb=_printf_class(conv),
            conv=conv,
            flags=flags,
            width=width,
            prec=prec,
            spec=flags + width + prec + mod,
            raw=s[start:i],
        )
        if name:
            tok = _replace(tok, name=name)
            named = True
        else:
            tok = _replace(tok, name=str(pos))
            pos += 1
            positional = True
        toks.append(tok)
        n += 1
    flush()
    if named and positional:
        ok = False  # "%" formatting takes either a mapping or a tuple, never both
    return toks, n, ok


def _printf_class(conv: str) -> str:
    if conv in "ra":
        return "q"
    if conv in "diouxX":
        return "d"
    if conv in "eEfFgG":
        return "f"
    if conv == "c":
        return "c"
    return "v"


def _replace(tok: Token, **changes) -> Token:
    values = {k: getattr(tok, k) for k in tok.__dataclass_fields__}
    values.update(changes)
    return Token(**values)


def _number_fields(toks: list[Token]) -> list[Token]:
    idx: dict[str, int] = {}
    out = []
    for t in toks:
        if t.is_verb():
            n = idx.get(t.name)
            if n is None:
                n = len(idx) + 1
                idx[t.name] = n
            t = _replace(t, arg=n)
        out.append(t)
    return out


def has_fields(s: str) -> bool:
    """Whether ``s`` is a usable template: at least one field, nothing that prevents reverse matching."""
    if "{" not in s and "%" not in s:
        return False
    toks, ok = parse(s)
    return ok and any(t.is_verb() for t in toks)


def literal(toks: list[Token]) -> str:
    return "".join(t.lit for t in toks if not t.is_verb())


def has_letter(s: str) -> bool:
    return any(c.isalpha() for c in s)


def split_space(s: str) -> tuple[str, str, str]:
    """Splits ``s`` into leading whitespace, the trimmed core and trailing whitespace (space, tab, CR, LF)."""
    core = s.lstrip(" \t\r\n")
    lead = s[: len(s) - len(core)]
    trimmed = core.rstrip(" \t\r\n")
    return lead, trimmed, core[len(trimmed) :]


_TAG = re.compile(r"\\?\[(?:/|/?[a-z#@][^\[\]\n]*)\]")


def _tags(s: str) -> list[str]:
    if "[" not in s:
        return []
    esc: list[str] = []
    open_: list[str] = []
    closing = False
    for m in _TAG.finditer(s):
        tok = m.group(0)
        if tok[0] == "\\":
            esc.append("\\[")
            continue
        if m.end() < len(s) and s[m.end()] == "(":
            continue  # a Markdown link
        if tok.startswith("[/"):
            closing = True
        open_.append(tok)
    if closing:
        esc.extend(open_)
    return sorted(esc)


def _backquoted(s: str) -> list[str]:
    out: list[str] = []
    while True:
        i = s.find("`")
        if i < 0:
            return out
        j = s.find("`", i + 1)
        if j < 0:
            out.append("`")
            return out
        out.append(s[i : j + 1])
        s = s[j + 1 :]


@dataclass(frozen=True)
class Placeholders:
    """What a translation must carry over unchanged: field signatures, backquoted spans and markup."""

    fields: list[str] = field(default_factory=list)
    backquoted: list[str] = field(default_factory=list)
    tags: list[str] = field(default_factory=list)

    def diff(self, other: Placeholders) -> str:
        probs = []
        if self.fields != other.fields:
            probs.append(f"format fields differ: source has {self.fields}, translation has {other.fields}")
        if self.backquoted != other.backquoted:
            probs.append(f"backquoted spans differ: source has {self.backquoted}, translation has {other.backquoted}")
        if self.tags != other.tags:
            probs.append(f"markup tags differ: source has {self.tags}, translation has {other.tags}")
        return "; ".join(probs)


def extract(s: str) -> Placeholders:
    """Returns the placeholders in ``s``. Fields count only when ``s`` parses as a template."""
    fields: list[str] = []
    if "{" in s or "%" in s:
        toks, ok = parse(s)
        if ok:
            fields = sorted({t.signature() for t in toks if t.is_verb()})
    return Placeholders(fields=fields, backquoted=sorted(_backquoted(s)), tags=_tags(s))


def new_control_chars(src: str, dst: str) -> bool:
    """Control characters (other than newline, tab, CR) or escape sequences in ``dst`` but not ``src``."""
    for c in dst:
        o = ord(c)
        if (o < 0x20 and c not in "\n\t\r") or o == 0x7F or 0x80 <= o < 0xA0 or o in (0x202E, 0x202D):
            if c not in src:
                return True
    return False


_QUOTED = r"""'(?:[^'\\\n]|\\.)*'|"(?:[^"\\\n]|\\.)*"|[^\n]+?"""


def _verb_expr(t: Token) -> str:
    padded = t.width != "" or any(f in t.flags for f in "- 0+") or (not t.raw.startswith("%") and _spec_padded(t.spec))

    def wrap(core: str) -> str:
        return r"([ \t]*(?:" + core + r")[ \t]*)" if padded else "(" + core + ")"

    if t.verb == "d":
        return wrap(r"[-+]?(?:0[bBoOxX])?[0-9a-fA-F][0-9a-fA-F,_]*")
    if t.verb == "f":
        return wrap(r"[-+]?(?:[0-9][0-9,_]*\.?[0-9]*|\.[0-9]+)(?:[eE][-+]?[0-9]+)?%?|[-+]?inf|nan")
    if t.verb == "q":
        return wrap(_QUOTED)
    if t.verb == "c":
        return "(.)"
    return "(.*?)"


class Pattern:
    """Reverse-matches text produced by formatting ``format`` and recovers each field's text."""

    __slots__ = ("format", "_toks", "_expr", "_re", "_args", "_classes", "_names", "prefix", "anchor", "specificity", "letters")

    def __init__(self, format: str, toks: list[Token]) -> None:
        self.format = format
        self._toks = toks
        lit = literal(toks)
        self.specificity = len(lit.encode("utf-8"))
        self.letters = sum(1 for c in lit if c.isalpha())
        self._args: list[int] = []
        self._classes: list[str] = []
        self._names: dict[str, int] = {}
        self.prefix = ""
        self.anchor = ""
        parts = ["^"]
        seen = False
        for t in toks:
            if not t.is_verb():
                parts.append(re.escape(t.lit))
                if not seen:
                    self.prefix += t.lit
                if len(t.lit) > len(self.anchor):
                    self.anchor = t.lit
                continue
            seen = True
            parts.append(_verb_expr(t))
            self._names[t.name] = t.arg
            self._args.append(t.arg)
            self._classes.append(t.verb)
        parts.append(r"\Z")
        self._expr = "".join(parts)
        self._re: re.Pattern[str] | None = None

    def _regex(self) -> re.Pattern[str]:
        if self._re is None:
            flags = re.DOTALL if "\n" in literal(self._toks) else 0
            self._re = re.compile(self._expr, flags)
        return self._re

    def match(self, s: str) -> tuple[dict[str, str], dict[str, str]] | None:
        """Returns each field's captured text and class, keyed by identity, or None."""
        if self.anchor and self.anchor not in s:
            return None
        m = self._regex().match(s)
        if m is None:
            return None
        by_arg: dict[int, str] = {}
        classes: dict[int, str] = {}
        for i, arg in enumerate(self._args):
            v = m.group(i + 1)
            if arg in by_arg:
                if by_arg[arg] != v:
                    return None
                continue
            by_arg[arg] = v
            classes[arg] = self._classes[i]
        return (
            {name: by_arg[arg] for name, arg in self._names.items()},
            {name: classes[arg] for name, arg in self._names.items()},
        )

    def splice(self, translation: str, captures: dict[str, str], transform=None) -> str | None:
        """Renders a translation of the format with captured field text, matched by identity."""
        toks, ok = parse(translation)
        if not ok:
            return None
        out = []
        for t in toks:
            if not t.is_verb():
                out.append(t.lit)
                continue
            if t.name not in captures:
                return None
            v = captures[t.name]
            if transform is not None:
                v = transform(t.name, v)
            out.append(v)
        return "".join(out)


def compile(format: str) -> Pattern | None:
    """Builds a pattern, or None when the format is invalid, has no fields, or has no letters."""
    toks, ok = parse(format)
    if not ok:
        return None
    if not has_letter(literal(toks)):
        return None
    if not any(t.is_verb() for t in toks):
        return None
    return Pattern(format, toks)


_PSEUDO_PROTECT = re.compile(
    r"\{\{|\}\}|\{[^{}\n]*(?:\{[^{}\n]*\}[^{}\n]*)*\}"
    r"|%\([^)\n]*\)[-+#0 ]*[0-9]*(?:\.[0-9]+)?[hlL]?[a-zA-Z]|%(?:[-+#0][-+#0 ]*)?[0-9]*(?:\.[0-9]+)?[hlL]?[a-zA-Z%]"
    r"|\\?\[(?:/|/?[a-z#@][^\[\]\n]*)\]|`[^`]*`|https?://[^\t\n\f\r ]+|--?[a-zA-Z][-a-zA-Z0-9]*|<[^>\t\n\f\r ]+>|\"[^\"\n]*\""
)
_ACCENT = str.maketrans(
    "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
    "áƀçďéƒĝĥîĵķļɱñöþǫŕšţûṽŵẋýžÅƁÇĎÉƑĜĤÎĴĶĻṀÑÖÞǪŔŠŢÛṼŴẊÝŽ",
)


def pseudo(s: str) -> str:
    """Pseudo-localizes ``s``: accented look-alikes for letters, wrapped in ⟦ ⟧, placeholders untouched."""
    lead, core, trail = split_space(s)
    if core == "":
        return s
    out = [lead, "⟦"]
    last = 0
    for m in _PSEUDO_PROTECT.finditer(core):
        out.append(core[last : m.start()].translate(_ACCENT))
        out.append(m.group(0))
        last = m.end()
    out.append(core[last:].translate(_ACCENT))
    out.append("⟧")
    out.append(trail)
    return "".join(out)
