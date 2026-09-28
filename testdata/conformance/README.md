# Conformance corpus

These vectors define the behaviour every Localizer runtime must share, whatever language it is written
in. The Go package `msgfmt` (and `engine`) and the Python package `localizer` run all of them in their
test suites. Change a rule here first, then both implementations.

| File | What it pins down |
| --- | --- |
| `placeholders.json`, `placeholders_go.json` | Which placeholders a string contains, and whether a translation is acceptable (Python and Go grammars). |
| `roundtrip.json`, `roundtrip_go.json` | Reverse matching: which formats compile to patterns, what they capture, and how captures are spliced into a translation. |
| `pseudo.json` | Pseudo-localization output (`LOCALIZER_LANG=qps`). |
| `engine.json` | The lookup engine end to end: exact hits, patterns, segments, label splits, modes, plausibility, ties. Groups are per grammar; the Python runtime runs the Python groups. |
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

**Signatures** (what a translation must reproduce, as a set: a placeholder that appears twice counts once,
so a translation may repeat a named or numbered field): brace fields as `{identity[!conv][:spec]}`
(`{name!r}`, `{0}`, `{t:.3f}`), positional printf verbs as `%position:flagswidthprecisionconv`
(`%0:-10s`), named ones as `%(name)flagswidthprecisionconv`. Flags, width and precision are part of the
signature so that a translation cannot change how a value is rendered or allocate huge padding. Named and
numbered fields may appear in any order.

**Also preserved:** backquoted spans (`` `snow sql` ``; an unpaired trailing backquote counts as one) — the
**first** span must stay first (a flag description's first span is what the CLI shows as the flag's value
name), the others may move; Rich markup **tags** — every tag `[…]`/`[/…]`/`[/]` (a tag starts with `a-z`,
`#` or `@`; `[text](url)` Markdown links are not tags) when the string contains at least one closing tag,
plus the marker `[/?]` when the closing tags don't balance the open ones the way Rich resolves them (`[/]`
closes the most recent open tag, `[/name]` the most recent open tag with that name), and every `\[` escape
always — and the absence of new **control characters**: U+0000–U+001F except `\n` and `\t` (a `\r` is
allowed only when the source has one), U+007F, U+0080–U+009F, the bidirectional embeddings, overrides and
isolates U+202A–U+202E and U+2066–U+2069, the line and paragraph separators U+2028 and U+2029, U+FEFF, and
the tag characters U+E0000–U+E007F. A character the source itself contains is never new.

## The Go placeholder grammar

Go strings use `fmt` verbs and `text/template` actions, checked by the Go runtime only
(`placeholders_go.json`, `roundtrip_go.json`; the Go groups of `engine.json`):

- A verb is `%` [*flags* from `+-# 0`] [`[n]`] [*width*] [`.` *precision*] *verb*; `%%` is a literal `%`.
  Signatures are `argument:flagswidthprecisionverb` (`1:s`, `2:-10s`), numbered the way `fmt` does.
- **A bare space flag is prose:** `% o` in `50% off` or `% c` in `100% complete` is literal text, exactly as in
  the Python grammar. A space flag with a width, precision or index (`% 5d`, `% [1]d`) is still a verb.
- `*` widths or precisions, unknown verbs, malformed indexes and a trailing `%` make the string invalid.
- Template actions `{{…}}` are preserved as written (whitespace inside normalized); an unterminated `{{`
  counts as the placeholder `{{`, so a translation may only contain one when the source does (an
  unterminated action makes `text/template` refuse the whole template).
- Backquoted spans and control characters follow the rules above.

## The engine

Both runtimes look strings up the same way (`engine.json`): the trimmed core of the string exactly; then,
for multi-line help text, templates that span lines; then paragraphs, or lines when there is a single
paragraph (help text: lines starting with `$ ` stay; error text: indented single-token lines such as
suggested command names stay); then single-line templates; then, outside output mode, `Label: rest`
splits. Templates compete by literal length; at equal length the application's catalog beats the runtime's
built-in one, then the first template in sorted order. A match is rejected when a generic template (fewer
than 10 letters) captures a lowercase phrase, and, outside error messages, when a capture spans lines or
sentences or, in help text, ends a sentence; wrapped errors (`%w`; text captures in error messages) are
exempt. Text captures (`%v`, class `v`) are translated again outside output mode, except a lone identifier
(letters, digits, `-_./:`) captured by a built-in template: a command or flag name, never a message.
Whitespace-variant keys resolve deterministically: an exact trimmed key wins, then the smallest original key.

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

- `placeholders.json`, `placeholders_go.json`: `{"syntax", "cases": [{"name", "source", "translation"?,
  "placeholders": {"fields"?, "actions"? (Go), "backquoted"?, "tags"? (Python)}, "valid"?}]}`. `valid` = the
  placeholders of `source` and `translation` are equal and `translation` adds no control characters.
- `roundtrip.json`: `{"syntax", "cases": [{"name", "format", "compiles"? (default true), "input"?, "match"?
  (default true), "captures"?, "classes"?, "translation"?, "spliced"?, "splices"? (default true)}]}`.
- `pseudo.json`: `{"syntax", "cases": [{"input", "output"}]}`.
- `engine.json`: `{"groups": [{"name", "syntax", "pseudo"?, "builtin"?: {source: translation} (the runtime's
  own catalog, passed first), "catalog": {source: translation} (the application's),
  "cases": [{"input", "mode": "output"|"help"|"error", "want"}]}]}`.
- `locale_match.json`: `{"groups": [{"available": [catalog languages], "cases": [{"prefs": [tags],
  "want": catalog language or ""}]}]}`.
