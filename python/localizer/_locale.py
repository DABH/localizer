# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Which language the user wants: explicit overrides, the POSIX locale environment, then the operating
system's preferred languages; and which catalog serves it best. A port of the Go ``internal/locale``
package; ``testdata/conformance/locale_match.json`` pins the matching results.
"""

from __future__ import annotations

import os
import stat
import sys
from collections.abc import Callable, Sequence
from dataclasses import dataclass, field

__all__ = ["Result", "detect", "normalize", "match", "os_languages"]


@dataclass
class Result:
    """The user's language preference."""

    tags: list[str] = field(default_factory=list)  # BCP 47 tags, most preferred first; empty = English
    source: str = ""  # the setting it came from (an environment variable name or "os")
    off: bool = False  # localization explicitly disabled
    pseudo: bool = False  # pseudo-localization requested (LOCALIZER_LANG=qps)


_OFF = {"off", "none", "0", "false", "no", "disable", "disabled"}
_PSEUDO = {"qps", "qps-ploc", "pseudo"}


def detect(
    override: Sequence[str] = (),
    getenv: Callable[[str], str | None] | None = None,
    os_langs: Callable[[], list[str]] | None = None,
) -> Result:
    """Determines the user's preferred languages.

    Order: the override variables, in order; then POSIX ``LC_ALL``, ``LC_MESSAGES`` and ``LANG`` (the
    first non-empty), with GNU ``LANGUAGE`` honoured unless that locale is C/POSIX; then the OS
    preference. A C/POSIX locale means English, so scripts that set ``LC_ALL=C`` get stable output.
    """
    env = getenv or os.environ.get
    for name in override:
        v = (env(name) or "").strip()
        if v:
            return _parse_override(v, name)
    for name in ("LC_ALL", "LC_MESSAGES", "LANG"):
        v = (env(name) or "").strip()
        if not v:
            continue
        tag, is_c = normalize(v)
        if is_c:
            return Result(tags=["en"], source=name)
        tags = _split_list(env("LANGUAGE") or "")
        if tag:
            tags.append(tag)
        return Result(tags=tags, source=name)
    tags = _split_list(env("LANGUAGE") or "")
    if tags:
        return Result(tags=tags, source="LANGUAGE")
    tags = (os_langs or os_languages)()
    if tags:
        return Result(tags=list(tags), source="os")
    return Result()


def _parse_override(v: str, source: str) -> Result:
    low = v.lower()
    if low in _OFF:
        return Result(off=True, source=source)
    if low in _PSEUDO:
        return Result(pseudo=True, source=source)
    return Result(tags=_split_list(v), source=source)


def _split_list(v: str) -> list[str]:
    out = []
    for f in v.replace(",", ":").split(":"):
        if not f:
            continue
        tag, is_c = normalize(f)
        if is_c:
            out.append("en")
        elif tag:
            out.append(tag)
    return out


def normalize(posix: str) -> tuple[str, bool]:
    """Converts a POSIX locale name ("pt_BR.UTF-8", "sr_RS@latin") to a BCP 47 tag ("pt-BR",
    "sr-Latn-RS"). The flag reports the C/POSIX locale (including C.UTF-8)."""
    s = posix.strip()
    modifier = ""
    if "@" in s:
        s, modifier = s.split("@", 1)
        modifier = modifier.lower()
    if "." in s:
        s = s.split(".", 1)[0]
    if s == "":
        return "", False
    if s in ("C", "POSIX"):
        return "", True
    s = s.replace("_", "-")
    if modifier == "latin":
        s = _insert_script(s, "Latn")
    elif modifier == "cyrillic":
        s = _insert_script(s, "Cyrl")
    return s, False


def _insert_script(tag: str, script: str) -> str:
    lang, sep, rest = tag.partition("-")
    if not sep:
        return lang + "-" + script
    return lang + "-" + script + "-" + rest


# -- operating system preferences -------------------------------------------------------------


def os_languages() -> list[str]:
    """The OS-wide preferred languages: macOS AppleLanguages, Windows display languages, else none."""
    try:
        if sys.platform == "darwin":
            return _apple_languages()
        if sys.platform == "win32":
            return _windows_languages()
    except Exception:  # never let detection break the CLI
        pass
    return []


_MAX_PLIST = 8 << 20  # .GlobalPreferences.plist is a few kilobytes; anything near this is not the file we want


def _apple_languages() -> list[str]:
    return _plist_languages(os.path.join(os.path.expanduser("~"), "Library", "Preferences", ".GlobalPreferences.plist"))


def _plist_languages(path: str) -> list[str]:
    """The AppleLanguages of the property list at ``path``, or none when anything is off: the file is
    missing or unreadable, is not a regular file (a FIFO or a device could block forever), is larger than
    ``_MAX_PLIST``, or is malformed. This runs at every CLI startup, so it must neither block nor fail; the
    worst outcome is English. As in the Go runtime, the open is non-blocking (a FIFO without a writer
    returns at once; regular files are unaffected) and the checks run on the open descriptor, so nothing
    can be swapped in between them and the read. On Windows the descriptor must be binary (the C runtime
    would otherwise translate line endings and stop at a Ctrl-Z byte) and not inherited."""
    flags = os.O_RDONLY
    for name in ("O_NONBLOCK", "O_CLOEXEC", "O_BINARY", "O_NOINHERIT"):
        flags |= getattr(os, name, 0)
    try:
        fd = os.open(path, flags)
    except OSError:
        return []
    try:
        st = os.fstat(fd)
        if not stat.S_ISREG(st.st_mode) or st.st_size > _MAX_PLIST:
            return []
        data = _read_up_to(fd, _MAX_PLIST + 1)
    except OSError:
        return []
    finally:
        os.close(fd)
    if len(data) > _MAX_PLIST:
        return []
    return apple_languages(data)


def _read_up_to(fd: int, limit: int) -> bytes:
    parts, size = [], 0
    while size < limit:
        chunk = os.read(fd, min(limit - size, 1 << 16))
        if not chunk:
            break
        parts.append(chunk)
        size += len(chunk)
    return b"".join(parts)


def apple_languages(data: bytes) -> list[str]:
    """The AppleLanguages array of a (binary or XML) property list, or an empty list."""
    import plistlib

    try:
        prefs = plistlib.loads(data)
    except Exception:
        return []
    langs = prefs.get("AppleLanguages") if isinstance(prefs, dict) else None
    if not isinstance(langs, list):
        return []
    return [s for s in langs if isinstance(s, str)]


def _windows_languages() -> list[str]:
    import ctypes
    from ctypes import wintypes

    kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)  # type: ignore[attr-defined]
    fn = kernel32.GetUserPreferredUILanguages
    fn.argtypes = [wintypes.DWORD, ctypes.POINTER(wintypes.ULONG), wintypes.LPWSTR, ctypes.POINTER(wintypes.ULONG)]
    fn.restype = wintypes.BOOL
    mui_language_name = 0x8
    count, size = wintypes.ULONG(), wintypes.ULONG()
    if not fn(mui_language_name, ctypes.byref(count), None, ctypes.byref(size)):
        return []
    buf = ctypes.create_unicode_buffer(size.value)
    if not fn(mui_language_name, ctypes.byref(count), buf, ctypes.byref(size)):
        return []
    return [s for s in buf[: size.value].split("\0") if s]


# -- matching ----------------------------------------------------------------------------------

# Deprecated codes and their replacements (the ones that matter for CLI users).
_ALIASES = {"iw": "he", "in": "id", "ji": "yi", "jw": "jv", "mo": "ro", "tl": "fil", "no": "nb"}
# A tag that means a language and script.
_SCRIPT_ALIASES = {"sh": ("sr", "Latn")}
# Languages close enough that a catalog in one serves readers of the other, ranked below any catalog
# in the reader's own language. The macro tag "no" (an alias of "nb") takes no part in this.
_CLOSE = {"nn": ("nb",), "nb": ("nn",)}
# The script a language is written in unless a tag says otherwise (only languages where an explicit
# script in a preference must be checked against it).
_LIKELY_SCRIPT = {
    "ar": "Arab", "bg": "Cyrl", "cs": "Latn", "da": "Latn", "de": "Latn", "el": "Grek", "en": "Latn",
    "es": "Latn", "fa": "Arab", "fi": "Latn", "fil": "Latn", "fr": "Latn", "he": "Hebr", "hi": "Deva",
    "hr": "Latn", "hu": "Latn", "id": "Latn", "it": "Latn", "ja": "Jpan", "ko": "Kore", "nb": "Latn",
    "nl": "Latn", "nn": "Latn", "pl": "Latn", "pt": "Latn", "ro": "Latn", "ru": "Cyrl", "sk": "Latn",
    "sv": "Latn", "th": "Thai", "tr": "Latn", "uk": "Cyrl", "vi": "Latn", "zh": "Hans",
}
# Regions grouped by the variety of English (and Spanish) they use: a preference for one region matches
# a catalog for another region of the same group better than a catalog with no region.
_REGION_GROUPS = {
    "en": {"GB": {"GB", "AU", "NZ", "IE", "IN", "ZA", "SG", "HK", "MY", "PK", "NG", "KE", "GH", "JM", "TT", "ZW", "150", "001"},
           "US": {"US", "PR", "GU", "VI", "AS", "MP"}},
    "es": {"ES": {"ES", "EA", "IC", "GQ", "PH"},
           "419": {"419", "MX", "AR", "CO", "CL", "PE", "VE", "EC", "GT", "CU", "BO", "DO", "HN", "PY", "SV", "NI", "CR", "PA", "UY", "US", "PR"}},
    "pt": {"BR": {"BR"}, "PT": {"PT", "AO", "MZ", "CV", "GW", "ST", "TL", "MO", "150"}},
    "fr": {"FR": {"FR", "BE", "CH", "LU", "MC", "150"}, "CA": {"CA"}},
}


@dataclass(frozen=True)
class _Tag:
    lang: str
    script: str  # "" when unspecified
    region: str  # "" when unspecified
    close_ok: bool = True  # may match a close language (not for the macro tag "no")


def _parse_tag(s: str) -> _Tag | None:
    parts = s.strip().replace("_", "-").split("-")
    lang = parts[0].lower()
    if not (2 <= len(lang) <= 8) or not lang.isalpha() or not lang.isascii():
        return None
    script = region = ""
    rest = parts[1:]
    if rest and len(rest[0]) == 4 and rest[0].isalpha():
        script = rest[0].title()
        rest = rest[1:]
    if rest and ((len(rest[0]) == 2 and rest[0].isalpha()) or (len(rest[0]) == 3 and rest[0].isdigit())):
        region = rest[0].upper()
    if lang in _SCRIPT_ALIASES:
        lang, alias_script = _SCRIPT_ALIASES[lang]
        script = script or alias_script
    close_ok = lang != "no"
    lang = _ALIASES.get(lang, lang)
    if not script:
        script = _likely_script(lang, region)
    return _Tag(lang, script, region, close_ok)


def _likely_script(lang: str, region: str) -> str:
    if lang == "zh":
        return "Hant" if region in ("TW", "HK", "MO") else "Hans"
    if lang == "sr":
        return "Latn" if region == "ME" else "Cyrl"
    if lang == "pa" and region == "PK":
        return "Arab"
    if lang in ("uz", "az") and region in ("AF", "IR"):
        return "Arab"
    return _LIKELY_SCRIPT.get(lang, "")


_NONE, _CLOSE_LANGUAGE, _LANGUAGE, _REGION_GROUP, _EXACT = 0, 1, 2, 3, 4


def _score(pref: _Tag, cand: _Tag) -> int:
    lang = pref.lang
    if cand.lang != lang:
        if not (pref.close_ok and cand.close_ok) or cand.lang not in _CLOSE.get(lang, ()):
            return _NONE
        if pref.script and cand.script and pref.script != cand.script:
            return _NONE
        return _CLOSE_LANGUAGE  # below every catalog in the reader's own language
    if pref.script and cand.script and pref.script != cand.script:
        return _NONE
    if pref.region == cand.region or (not cand.region and not pref.region):
        return _EXACT
    groups = _REGION_GROUPS.get(lang, {})
    for members in groups.values():
        if pref.region in members and cand.region in members:
            return _REGION_GROUP
    return _LANGUAGE


def match(prefs: Sequence[str], available: Sequence[str]) -> str:
    """Picks the best catalog language in ``available`` for the user's preferred tags, or "" when
    English is at least as good as any catalog or nothing matches confidently (a Traditional Chinese
    reader is never shown Simplified Chinese)."""
    if not prefs or not available:
        return ""
    supported: list[tuple[str, _Tag]] = [("", _Tag("en", "Latn", ""))]
    for a in available:
        t = _parse_tag(a)
        if t is not None:
            supported.append((a, t))
    if len(supported) == 1:
        return ""
    for p in prefs:
        pref = _parse_tag(p)
        if pref is None:
            continue
        best_name, best_score = "", _NONE
        for name, cand in supported:
            score = _score(pref, cand)
            if score > best_score:
                best_name, best_score = name, score
        if best_score >= _CLOSE_LANGUAGE:
            return best_name
    return ""
