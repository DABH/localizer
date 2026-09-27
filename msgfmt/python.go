// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package msgfmt

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Syntax selects the placeholder grammar of a CLI's strings. The grammars are specified by the
// conformance corpus in testdata/conformance, which the Python runtime implements too.
type Syntax uint8

const (
	// Go strings use fmt verbs ("%s", "%[2]d") and text/template actions ("{{.CommandPath}}").
	Go Syntax = iota
	// Python strings use str.format fields ("{name}", "{0}", "{}", "{x!r:>10}") or printf-style verbs
	// ("%s", "%(name)s"). Rich markup tags ("[bold]…[/bold]") and "\[" escapes are placeholders too.
	Python
)

func (s Syntax) String() string {
	if s == Python {
		return "python"
	}
	return "go"
}

// ParseSyntax is Parse for the given syntax. For Python, brace fields win: a string with at least one
// is a str.format template ("{{" and "}}" are literal braces and "%" is plain text). Otherwise the
// string is a printf template if the whole of it parses as one with at least one verb; a bare
// space-flag verb, as in "50% of", is plain text. A string that is neither is a single literal token.
// ok is false for templates that cannot be used: '*' widths, unknown conversions, a trailing '%',
// mixed automatic and manual field numbering, or mixed named and positional verbs.
func ParseSyntax(format string, syn Syntax) ([]Token, bool) {
	if syn != Python {
		return Parse(format)
	}
	toks, n, err := parseBraces(format)
	switch {
	case err == errTemplate, err == errBrace && n > 0:
		return toks, false
	case err == 0 && n > 0:
		return numberFields(toks), true
	}
	toks, n, ok := parsePrintf(format)
	if !ok {
		return toks, false
	}
	if n == 0 {
		return []Token{{Lit: format}}, true
	}
	return numberFields(toks), true
}

// HasFields reports whether s is a format string in the given syntax: at least one well-formed
// placeholder, and nothing that prevents reverse matching. It is HasVerbs for Go.
func HasFields(s string, syn Syntax) bool {
	if syn != Python {
		return HasVerbs(s)
	}
	if !strings.ContainsAny(s, "{%") {
		return false
	}
	toks, ok := parsePython(s)
	if !ok {
		return false
	}
	for _, t := range toks {
		if t.IsVerb() {
			return true
		}
	}
	return false
}

func parsePython(s string) ([]Token, bool) { return ParseSyntax(s, Python) }

type braceErr int

const (
	errBrace    braceErr = iota + 1 // an unterminated "{" or a lone "}"
	errTemplate                     // a field that Python's str.format would reject
)

// parseBraces tokenizes s as a str.format template. n is the number of fields found.
func parseBraces(s string) (toks []Token, n int, err braceErr) {
	var lit strings.Builder
	flushLit := func() {
		if lit.Len() > 0 {
			toks = append(toks, Token{Lit: lit.String()})
			lit.Reset()
		}
	}
	auto, manual := 0, false
	for i := 0; i < len(s); {
		switch s[i] {
		case '{':
			if i+1 < len(s) && s[i+1] == '{' {
				lit.WriteByte('{')
				i += 2
				continue
			}
			end := braceEnd(s, i+1)
			if end < 0 {
				return toks, n, errBrace
			}
			t, ok := parseField(s[i+1 : end])
			if !ok {
				return toks, n, errTemplate
			}
			t.Raw = s[i : end+1]
			if t.Name == "" {
				t.Auto = true
				t.Name = strconv.Itoa(auto)
				auto++
			} else if isDigits(argName(t.Name)) {
				manual = true
			}
			if auto > 0 && manual {
				return toks, n, errTemplate // str.format refuses to mix "{}" and "{0}"
			}
			flushLit()
			toks = append(toks, t)
			n++
			i = end + 1
		case '}':
			if i+1 < len(s) && s[i+1] == '}' {
				lit.WriteByte('}')
				i += 2
				continue
			}
			return toks, n, errBrace
		default:
			lit.WriteByte(s[i])
			i++
		}
	}
	flushLit()
	return toks, n, 0
}

// braceEnd returns the index of the "}" closing a field whose body starts at i, allowing one level of
// nesting inside the format spec ("{x:{width}}"), or -1.
func braceEnd(s string, i int) int {
	depth := 1
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// parseField parses the inside of a field: field_name ["!" conversion] [":" format_spec].
func parseField(field string) (Token, bool) {
	i, depth := 0, 0
	for i < len(field) {
		c := field[i]
		if c == '[' {
			depth++
		} else if c == ']' && depth > 0 {
			depth--
		} else if depth == 0 && (c == '!' || c == ':') {
			break
		}
		i++
	}
	t := Token{Name: field[:i]}
	rest := field[i:]
	if strings.HasPrefix(rest, "!") {
		if len(rest) < 2 || strings.IndexByte("rsa", rest[1]) < 0 {
			return Token{}, false
		}
		t.Conv = rest[1:2]
		rest = rest[2:]
		if rest != "" && rest[0] != ':' {
			return Token{}, false
		}
	}
	if strings.HasPrefix(rest, ":") {
		t.Spec = rest[1:]
	}
	t.Verb = braceClass(t.Conv, t.Spec)
	return t, true
}

// argName returns the leading arg_name of a field name ("self" in "self.role", "0" in "0[key]").
func argName(name string) string {
	if i := strings.IndexAny(name, ".["); i >= 0 {
		return name[:i]
	}
	return name
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// braceClass maps a field's conversion and format spec to a capture class: 'q' repr, 'd' integer,
// 'f' float, 'c' character, otherwise 'v' text.
func braceClass(conv, spec string) rune {
	if conv == "r" || conv == "a" {
		return 'q'
	}
	if spec == "" || strings.HasSuffix(spec, "}") {
		return 'v'
	}
	switch spec[len(spec)-1] {
	case 'b', 'd', 'n', 'o', 'x', 'X':
		return 'd'
	case 'e', 'E', 'f', 'F', 'g', 'G', '%':
		return 'f'
	case 'c':
		return 'c'
	}
	return 'v'
}

var specWidthRe = regexp.MustCompile(`^(?:.?[<>=^])?[-+ ]?z?#?0?[0-9]+`)

// specPadded reports whether a format spec pads its value (an explicit or nested width).
func specPadded(spec string) bool {
	return strings.Contains(spec, "{") || specWidthRe.MatchString(spec)
}

// parsePrintf tokenizes s as a printf-style template. n is the number of verbs found.
func parsePrintf(s string) (toks []Token, n int, ok bool) {
	ok = true
	var lit strings.Builder
	flushLit := func() {
		if lit.Len() > 0 {
			toks = append(toks, Token{Lit: lit.String()})
			lit.Reset()
		}
	}
	pos := 0
	named, positional := false, false
	for i := 0; i < len(s); {
		if s[i] != '%' {
			j := strings.IndexByte(s[i:], '%')
			if j < 0 {
				lit.WriteString(s[i:])
				break
			}
			lit.WriteString(s[i : i+j])
			i += j
			continue
		}
		start := i
		i++
		if i >= len(s) {
			lit.WriteByte('%')
			ok = false
			break
		}
		if s[i] == '%' {
			lit.WriteByte('%')
			i++
			continue
		}
		name := ""
		if s[i] == '(' {
			end := strings.IndexByte(s[i:], ')')
			if end < 0 {
				lit.WriteString(s[start:])
				ok = false
				break
			}
			name = s[i+1 : i+end]
			i += end + 1
		}
		fStart := i
		for i < len(s) && strings.IndexByte("#0- +", s[i]) >= 0 {
			i++
		}
		flags := s[fStart:i]
		wStart := i
		if i < len(s) && s[i] == '*' {
			ok = false
			i++
		} else {
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
		}
		width := s[wStart:i]
		prec := ""
		if i < len(s) && s[i] == '.' {
			pStart := i
			i++
			if i < len(s) && s[i] == '*' {
				ok = false
				i++
			} else {
				for i < len(s) && s[i] >= '0' && s[i] <= '9' {
					i++
				}
			}
			prec = s[pStart:i]
		}
		length := ""
		if i < len(s) && strings.IndexByte("hlL", s[i]) >= 0 {
			length = s[i : i+1]
			i++
		}
		if i >= len(s) {
			lit.WriteString(s[start:])
			ok = false
			break
		}
		conv := s[i]
		i++
		if name == "" && flags == " " && width == "" && prec == "" && length == "" {
			lit.WriteString(s[start:i]) // "50% of": prose, not a space-flag verb
			continue
		}
		if strings.IndexByte("diouxXeEfFgGcrsa", conv) < 0 {
			lit.WriteString(s[start:i])
			ok = false
			continue
		}
		flushLit()
		t := Token{Verb: printfClass(conv), Conv: string(conv), Flags: flags, Width: width, Prec: prec, Spec: flags + width + prec + length, Raw: s[start:i]}
		if name != "" {
			t.Name = name
			named = true
		} else {
			t.Name = strconv.Itoa(pos)
			pos++
			positional = true
		}
		toks = append(toks, t)
		n++
	}
	flushLit()
	if named && positional {
		ok = false // "%" formatting takes either a mapping or a tuple, never both
	}
	return toks, n, ok
}

func printfClass(conv byte) rune {
	switch conv {
	case 'r', 'a':
		return 'q'
	case 'd', 'i', 'o', 'u', 'x', 'X':
		return 'd'
	case 'e', 'E', 'f', 'F', 'g', 'G':
		return 'f'
	case 'c':
		return 'c'
	}
	return 'v'
}

// numberFields assigns Arg: one index per distinct field identity, in order of first appearance, so
// that Pattern can key captures the same way for both syntaxes.
func numberFields(toks []Token) []Token {
	idx := map[string]int{}
	for i := range toks {
		if !toks[i].IsVerb() {
			continue
		}
		n, ok := idx[toks[i].Name]
		if !ok {
			n = len(idx) + 1
			idx[toks[i].Name] = n
		}
		toks[i].Arg = n
	}
	return toks
}

// signature is what a translation must reproduce for a Python placeholder: its identity plus
// everything that changes how the value is rendered.
func (t Token) signature() string {
	if strings.HasPrefix(t.Raw, "%") {
		if len(t.Raw) > 1 && t.Raw[1] == '(' {
			return "%(" + t.Name + ")" + t.Spec + t.Conv
		}
		return "%" + t.Name + ":" + t.Spec + t.Conv
	}
	var b strings.Builder
	b.WriteByte('{')
	b.WriteString(t.Name)
	if t.Conv != "" {
		b.WriteByte('!')
		b.WriteString(t.Conv)
	}
	if t.Spec != "" {
		b.WriteByte(':')
		b.WriteString(t.Spec)
	}
	b.WriteByte('}')
	return b.String()
}

// tagRe matches Rich markup tags ("[bold]", "[/bold]", "[/]", "[link=https://…]") and "\[" escapes.
var tagRe = regexp.MustCompile(`\\?\[(?:/|/?[a-z#@][^\[\]\n]*)\]`)

// tags returns the markup a Python translation must keep: every "\[" escape, plus every tag when the
// string contains a closing tag (a closing tag without an open one makes Rich raise). "[text](url)"
// Markdown links are not tags.
func tags(s string) []string {
	if !strings.Contains(s, "[") {
		return nil
	}
	var esc, open []string
	closing := false
	for _, loc := range tagRe.FindAllStringIndex(s, -1) {
		tok := s[loc[0]:loc[1]]
		if tok[0] == '\\' {
			esc = append(esc, `\[`)
			continue
		}
		if loc[1] < len(s) && s[loc[1]] == '(' {
			continue
		}
		if strings.HasPrefix(tok, "[/") {
			closing = true
		}
		open = append(open, tok)
	}
	if closing {
		esc = append(esc, open...)
	}
	sort.Strings(esc)
	return esc
}

// ExtractSyntax is Extract for the given syntax.
func ExtractSyntax(s string, syn Syntax) Placeholders {
	if syn != Python {
		return Extract(s)
	}
	var p Placeholders
	if strings.ContainsAny(s, "{%") {
		if toks, ok := parsePython(s); ok {
			set := map[string]bool{}
			for _, t := range toks {
				if t.IsVerb() {
					set[t.signature()] = true
				}
			}
			for v := range set {
				p.Verbs = append(p.Verbs, v)
			}
			sort.Strings(p.Verbs)
		}
	}
	p.Backquoted = backquoted(s)
	sort.Strings(p.Backquoted)
	p.Tags = tags(s)
	return p
}

// pseudoProtectPython keeps str.format fields, printf verbs (but not the "50% of" prose case), markup
// tags, backquoted spans, quoted values, URLs, flags and <args> out of pseudo-localization.
var pseudoProtectPython = regexp.MustCompile(`\{\{|\}\}|\{[^{}\n]*(?:\{[^{}\n]*\}[^{}\n]*)*\}` +
	`|%\([^)\n]*\)[-+#0 ]*[0-9]*(?:\.[0-9]+)?[hlL]?[a-zA-Z]|%(?:[-+#0][-+#0 ]*)?[0-9]*(?:\.[0-9]+)?[hlL]?[a-zA-Z%]` +
	`|\\?\[(?:/|/?[a-z#@][^\[\]\n]*)\]` + "|`[^`]*`" + `|https?://\S+|--?[a-zA-Z][-a-zA-Z0-9]*|<[^>\s]+>|"[^"\n]*"`)

// PseudoSyntax is Pseudo for the given syntax.
func PseudoSyntax(s string, syn Syntax) string {
	if syn != Python {
		return Pseudo(s)
	}
	return pseudo(s, pseudoProtectPython)
}
