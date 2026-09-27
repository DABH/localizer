// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Package msgfmt understands the two kinds of placeholders that appear in CLI strings: Go fmt verbs
// ("Created %s \"%s\".") and text/template actions ("{{.CommandPath}}").
//
// It is shared by the runtime (reverse-matching already formatted output against catalog keys) and by
// the tooling (extracting template units, validating translations).
package msgfmt

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token is one piece of a parsed format string: literal text or a single verb.
type Token struct {
	Lit   string // literal text; "%%" is already unescaped to "%"
	Verb  rune   // verb character ('s', 'd', ...); 0 for literal tokens
	Arg   int    // 1-based index of the argument the verb consumes
	Flags string // any of "+-# 0"
	Width string // decimal width, or "" (a '*' width makes the format unsupported)
	Prec  string // precision including the leading '.', or ""
	Raw   string // the verb exactly as written, e.g. "%-10s" or "%[2]q"

	// Python syntax only (see Syntax): Name is the field's identity ("name", "self.role", "0"; an
	// auto-numbered "{}" gets its position), Conv its conversion ("r", "s" or "a" for "{x!r}", or the
	// printf conversion character) and Spec its format spec (the text after ":" in "{x:>10}", or the
	// printf flags, width, precision and length). Verb then holds a capture class rather than a Go verb:
	// 'v' text, 'q' repr, 'd' integer, 'f' float, 'c' character.
	Name string
	Conv string
	Spec string
	Auto bool
}

// IsVerb reports whether t is a verb (as opposed to literal text).
func (t Token) IsVerb() bool { return t.Verb != 0 }

const knownVerbs = "bcdeEfFgGoOpqstTUvxXw"

// Parse splits a Go format string into literal and verb tokens, numbering verb arguments the way fmt does
// (including explicit indexes such as %[2]s). ok is false if the string uses constructs that cannot be
// reverse-matched reliably: '*' widths or precisions, unknown verbs, malformed indexes or a trailing '%'.
func Parse(format string) (toks []Token, ok bool) {
	ok = true
	var lit strings.Builder
	flushLit := func() {
		if lit.Len() > 0 {
			toks = append(toks, Token{Lit: lit.String()})
			lit.Reset()
		}
	}
	argNum := 1
	n := len(format)
	for i := 0; i < n; {
		if format[i] != '%' {
			j := strings.IndexByte(format[i:], '%')
			if j < 0 {
				lit.WriteString(format[i:])
				break
			}
			lit.WriteString(format[i : i+j])
			i += j
			continue
		}
		start := i
		i++
		if i >= n {
			lit.WriteByte('%')
			ok = false
			break
		}
		fStart := i
		for i < n && strings.IndexByte("+-# 0", format[i]) >= 0 {
			i++
		}
		flags := format[fStart:i]
		var good bool
		if argNum, i, good = argIndex(format, i, argNum); !good {
			ok = false
		}
		wStart := i
		if i < n && format[i] == '*' {
			ok = false
			argNum++
			i++
		} else {
			for i < n && format[i] >= '0' && format[i] <= '9' {
				i++
			}
		}
		width := format[wStart:i]
		prec := ""
		if i < n && format[i] == '.' {
			pStart := i
			i++
			if argNum, i, good = argIndex(format, i, argNum); !good {
				ok = false
			}
			if i < n && format[i] == '*' {
				ok = false
				argNum++
				i++
			} else {
				for i < n && format[i] >= '0' && format[i] <= '9' {
					i++
				}
			}
			prec = format[pStart:i]
		}
		if argNum, i, good = argIndex(format, i, argNum); !good {
			ok = false
		}
		if i >= n {
			lit.WriteString(format[start:])
			ok = false
			break
		}
		verb, size := utf8.DecodeRuneInString(format[i:])
		i += size
		if verb == '%' {
			lit.WriteByte('%')
			continue
		}
		if !strings.ContainsRune(knownVerbs, verb) {
			ok = false
		}
		flushLit()
		toks = append(toks, Token{Verb: verb, Arg: argNum, Flags: flags, Width: width, Prec: prec, Raw: format[start:i]})
		argNum++
	}
	flushLit()
	return toks, ok
}

// argIndex parses an optional "[n]" at format[i:]. It returns the argument number to use next, the new
// position, and false if an index is present but malformed.
func argIndex(format string, i, argNum int) (int, int, bool) {
	if i >= len(format) || format[i] != '[' {
		return argNum, i, true
	}
	end := strings.IndexByte(format[i:], ']')
	if end < 0 {
		return argNum, i, false
	}
	v, err := strconv.Atoi(format[i+1 : i+end])
	if err != nil || v < 1 {
		return argNum, i + end + 1, false
	}
	return v, i + end + 1, true
}

// HasVerbs reports whether s contains at least one well-formed fmt verb, i.e. whether it looks like a
// format string rather than plain text.
func HasVerbs(s string) bool {
	if !strings.Contains(s, "%") {
		return false
	}
	toks, ok := Parse(s)
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

// Literal returns the concatenated literal text of a parsed format.
func Literal(toks []Token) string {
	var b strings.Builder
	for _, t := range toks {
		if !t.IsVerb() {
			b.WriteString(t.Lit)
		}
	}
	return b.String()
}

// HasLetter reports whether s contains a Unicode letter.
func HasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
