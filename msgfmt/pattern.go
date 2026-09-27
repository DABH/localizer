// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package msgfmt

import (
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// Pattern reverse-matches text that was produced by fmt.Sprintf(format, args...) and recovers the
// formatted text of each argument, so the same arguments can be spliced into a translated format.
type Pattern struct {
	Format  string // the format string the pattern was compiled from
	toks    []Token
	expr    string // regular expression, compiled on first use
	once    sync.Once
	re      *regexp.Regexp
	args    []int  // regexp group i+1 captures argument args[i]
	verbs   []rune // verb for group i+1
	prefix  string // literal text before the first verb
	anchor  string // longest literal run, used as a cheap prefilter
	litLen  int    // total literal length; longer means more specific
	letters int    // letters in the literal text
}

// Compile builds a Pattern for format. It returns false when format has no verbs, uses unsupported
// constructs, or has no letters in its literal text (such formats, like "%s: %s", would match almost
// anything and must never be used for reverse matching).
func Compile(format string) (*Pattern, bool) {
	toks, ok := Parse(format)
	if !ok {
		return nil, false
	}
	lit := Literal(toks)
	if !HasLetter(lit) {
		return nil, false
	}
	p := &Pattern{Format: format, toks: toks, litLen: len(lit)}
	for _, r := range lit {
		if unicode.IsLetter(r) {
			p.letters++
		}
	}
	var b strings.Builder
	if strings.Contains(lit, "\n") {
		// Captures may only span lines when the format itself does.
		b.WriteString("(?s)")
	}
	b.WriteString("^")
	seenVerb := false
	for _, t := range toks {
		if !t.IsVerb() {
			b.WriteString(regexp.QuoteMeta(t.Lit))
			if !seenVerb {
				p.prefix += t.Lit
			}
			if len(t.Lit) > len(p.anchor) {
				p.anchor = t.Lit
			}
			continue
		}
		seenVerb = true
		b.WriteString(verbExpr(t))
		p.args = append(p.args, t.Arg)
		p.verbs = append(p.verbs, t.Verb)
	}
	if !seenVerb {
		return nil, false
	}
	b.WriteString("$")
	p.expr = b.String()
	return p, true
}

// regexp compiles the pattern on first use: catalogs hold hundreds of formats, and most are never needed
// in a given run of a CLI.
func (p *Pattern) regexp() *regexp.Regexp {
	p.once.Do(func() { p.re, _ = regexp.Compile(p.expr) })
	return p.re
}

const quotedExpr = `"(?:[^"\\\n]|\\.)*"|` + "`[^`]*`" + `|'(?:[^'\\\n]|\\.)*'|\[[^\n]*?\]`

func verbExpr(t Token) string {
	padded := t.Width != "" || strings.ContainsAny(t.Flags, "- 0+")
	wrap := func(core string) string {
		if padded {
			return `(\s*(?:` + core + `)\s*)`
		}
		return `(` + core + `)`
	}
	switch t.Verb {
	case 'd':
		return wrap(`[-+]?\d+`)
	case 't':
		return wrap(`true|false`)
	case 'q':
		return wrap(quotedExpr)
	case 'e', 'E', 'f', 'F', 'g', 'G':
		return wrap(`[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?|[-+]?Inf|NaN`)
	case 'b', 'c', 'o', 'O', 'x', 'X', 'U', 'p':
		return `(.+?)`
	default: // s, v, w, T
		return `(.*?)`
	}
}

// Prefix returns the literal text before the first verb ("" if the format starts with a verb).
func (p *Pattern) Prefix() string { return p.prefix }

// Anchor returns the longest literal run of the format.
func (p *Pattern) Anchor() string { return p.anchor }

// Specificity orders competing matches: patterns with more literal text win.
func (p *Pattern) Specificity() int { return p.litLen }

// Letters is the number of letters in the format's literal text. Formats with few letters (like
// "Manage %s.") are generic and need extra care before their matches are trusted.
func (p *Pattern) Letters() int { return p.letters }

// Match reverse-matches s. On success it returns the formatted text of each argument and the verb that
// produced it, keyed by argument index. If an argument is formatted more than once (e.g. "%[1]s ... %[1]s")
// every occurrence must be identical.
func (p *Pattern) Match(s string) (args map[int]string, verbs map[int]rune, ok bool) {
	if p.anchor != "" && !strings.Contains(s, p.anchor) {
		return nil, nil, false
	}
	re := p.regexp()
	if re == nil {
		return nil, nil, false
	}
	m := re.FindStringSubmatch(s)
	if m == nil {
		return nil, nil, false
	}
	args = make(map[int]string, len(p.args))
	verbs = make(map[int]rune, len(p.args))
	for i, a := range p.args {
		v := m[i+1]
		if prev, seen := args[a]; seen {
			if prev != v {
				return nil, nil, false
			}
			continue
		}
		args[a] = v
		verbs[a] = p.verbs[i]
	}
	return args, verbs, true
}

// Splice renders a translated format by substituting the already formatted argument text captured by
// Match. transform, if non-nil, may rewrite an argument's text (for example to translate a wrapped
// error). It returns false if the translation refers to an argument that was not captured or cannot be
// parsed.
func Splice(translation string, args map[int]string, transform func(arg int, text string) string) (string, bool) {
	toks, ok := Parse(translation)
	if !ok {
		return "", false
	}
	var b strings.Builder
	for _, t := range toks {
		if !t.IsVerb() {
			b.WriteString(t.Lit)
			continue
		}
		v, found := args[t.Arg]
		if !found {
			return "", false
		}
		if transform != nil {
			v = transform(t.Arg, v)
		}
		b.WriteString(v)
	}
	return b.String(), true
}
