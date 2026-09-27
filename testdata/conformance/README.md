# Conformance corpus

These vectors define the behaviour every Localizer runtime must share, whatever language it is written
in. The Go package `msgfmt` (and `engine`) and the Python package `localizer` run all of them in their
test suites. Change a rule here first, then both implementations.

| File | What it pins down |
| --- | --- |
| `placeholders.json` | Which placeholders a string contains, and whether a translation is acceptable. |
| `roundtrip.json` | Reverse matching: which formats compile to patterns, what they capture, and how captures are spliced into a translation. |
| `pseudo.json` | Pseudo-localization output (`LOCALIZER_LANG=qps`). |
| `engine.json` | The lookup engine end to end: exact hits, patterns, segments, label splits, modes. |
| `locale_match.json` | Language matching: which catalog a list of preferred languages selects. Generated from the Go implementation (`go test ./internal/locale -run Golden -update`). |

## The Python placeholder grammar

A string is one of three things, decided in this order:

1. **A `str.format` template** when it contains at least one field. `{{` and `}}` are literal braces;
   `%` is plain text. A field is `{` *name* [`!` *conversion*] [`:` *spec*] `}`:
   - *name* is empty (auto-numbered: the field's identity is its position, `0`, `1`, …), a number, or a
     name with optional `.attr` and `[index]` parts (`self.role`, `d[a:b]`; a colon inside brackets is
     part of the index). Mixing empty and numbered names makes the string invalid.
   - *conversion* is `r`, `s` or `a`. Anything else makes the string invalid.
   - *spec* is kept verbatim, including one level of nested fields (`{x:{width}}`).
   - Capture class: `q` for `!r`/`!a`; from the spec's last character, `d` for `b d n o x X`, `f` for
     `e E f F g G %`, `c` for `c`; otherwise `v` (text).
   - A lone `{` or `}` means the string is not a template (it is prose), unless fields were already found,
     in which case the string is invalid.
2. **A printf template** when the whole string parses as one with at least one verb:
   `%` [`(` *name* `)`] *flags* [*width*] [`.` *precision*] [`h`|`l`|`L`] *conversion*, with *flags* from
   `#0- +` and *conversion* from `d i o u x X e E f F g G c r s a`; `%%` is a literal `%`.
   - A `*` width or precision, an unknown conversion, a trailing `%`, or a mix of named and positional
     verbs makes the string invalid (no placeholders are recognised).
   - **A bare space flag is prose:** `% o` in `50% of` or `% d` in `100% done` is literal text.
   - Identity: the name for `%(name)s`, otherwise the verb's position (`0`, `1`, …).
   - Capture class: `q` for `r a`, `d` for `d i o u x X`, `f` for `e E f F g G`, `c` for `c`, `v` for `s`.
3. **Plain text** otherwise.

**Signatures** (what a translation must reproduce, as a multiset): brace fields as `{identity[!conv][:spec]}`
(`{name!r}`, `{0}`, `{t:.3f}`), positional printf verbs as `%position:flagswidthprecisionconv`
(`%0:-10s`), named ones as `%(name)flagswidthprecisionconv`. Flags, width and precision are part of the
signature so that a translation cannot change how a value is rendered or allocate huge padding. Named and
numbered fields may appear in any order.

**Also preserved:** backquoted spans (`` `snow sql` ``; an unpaired trailing backquote counts as one),
Rich markup **tags** — every tag `[…]`/`[/…]`/`[/]` (a tag starts with `a-z`, `#` or `@`; `[text](url)`
Markdown links are not tags) when the string contains at least one closing tag, and every `\[` escape
always — and the absence of new control characters (U+0000–U+001F except `\n`, `\t`, `\r`; U+007F;
U+0080–U+009F; U+202E; U+202D).

**Reverse matching:** a format compiles to a pattern only if it is valid, has at least one field, and has
a letter in its literal text. Captures are anchored regular expressions built from ASCII classes: `v`
`(.*?)`, `q` a single- or double-quoted string or a run of non-newline text, `d` `[-+]?(?:0[bBoOxX])?[0-9a-fA-F][0-9a-fA-F,_]*`,
`f` a decimal with optional exponent, `%`, `inf` or `nan`, `c` one character. A padded field (width, fill or a
nested spec; printf width or `- 0+` flags) may carry surrounding spaces and tabs. Captures never cross a
line unless the format itself contains a newline. A repeated identity must capture identical text.
Splicing renders a translation by identity, so `{1} y {0}` reverses `{} and {}`.

**Pseudo-localization:** letters are replaced by accented look-alikes (`a→á … z→ž`, `A→Å … Z→Ž`) and the
trimmed core is wrapped in `⟦ ⟧`; fields, printf verbs (not the prose `% o` case), markup tags, `\[`
escapes, backquoted spans, `"quoted"` values, URLs, `-f`/`--flags` and `<args>` stay untouched.

## File formats

- `placeholders.json`: `{"syntax", "cases": [{"name", "source", "translation"?, "placeholders": {"fields"?,
  "backquoted"?, "tags"?}, "valid"?}]}`. `valid` = the placeholders of `source` and `translation` are equal
  and `translation` adds no control characters.
- `roundtrip.json`: `{"syntax", "cases": [{"name", "format", "compiles"? (default true), "input"?, "match"?
  (default true), "captures"?, "classes"?, "translation"?, "spliced"?, "splices"? (default true)}]}`.
- `pseudo.json`: `{"syntax", "cases": [{"input", "output"}]}`.
- `engine.json`: `{"groups": [{"name", "syntax", "pseudo"?, "catalog": {source: translation},
  "cases": [{"input", "mode": "output"|"help"|"error", "want"}]}]}`.
- `locale_match.json`: `{"groups": [{"available": [catalog languages], "cases": [{"prefs": [tags],
  "want": catalog language or ""}]}]}`.
