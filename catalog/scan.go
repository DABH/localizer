// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package catalog

import (
	"errors"
	"strconv"
	"unicode/utf8"
)

// ParseFast decodes a catalog without encoding/json: a single pass that keeps unescaped strings as
// substrings of one copy of the input, so startup cost stays well under a millisecond for thousands of
// entries. It accepts exactly the catalog shape ({"version": N, "language": "...", "format": "...",
// "messages": {...}}) and falls back to Parse on anything else, so its result is always the one Parse
// would produce. The rules of encoding/json that a hand-written parser gets wrong most easily:
//
//   - A repeated key replaces the earlier value, except that a repeated "messages" object is decoded into
//     the map already built, so its entries override the earlier ones key by key.
//   - An escaped high surrogate must be followed by an escaped low surrogate; otherwise it decodes to
//     U+FFFD and what follows is decoded on its own ("\ud800A" is U+FFFD then "A").
//   - Only whitespace (space, tab, CR, LF) may follow the closing brace, and a UTF-8 byte order mark
//     before the opening brace is a syntax error.
func ParseFast(data []byte) (*File, error) {
	f, ok := scan(data)
	if !ok {
		return Parse(data)
	}
	if f.Version > Version {
		return nil, errors.New("catalog: unsupported version " + strconv.Itoa(f.Version))
	}
	if f.Messages == nil {
		f.Messages = map[string]string{}
	}
	return f, nil
}

// scan is the fast path proper. ok is false for anything it does not handle, malformed or merely
// unexpected, and the caller then defers to Parse.
func scan(data []byte) (*File, bool) {
	if !utf8.Valid(data) {
		return nil, false // encoding/json's replacement-character semantics
	}
	p := scanner{s: string(data)} // one copy; every substring below shares it
	return p.file()
}

type scanner struct {
	s string
	i int
}

func (p *scanner) file() (*File, bool) {
	f := &File{}
	if !p.ws().eat('{') {
		return nil, false
	}
	if !p.ws().eat('}') {
		for {
			key, ok := p.str()
			if !ok || !p.ws().eat(':') {
				return nil, false
			}
			p.ws()
			switch key {
			case "version":
				if f.Version, ok = p.num(); !ok {
					return nil, false
				}
			case "language":
				if f.Language, ok = p.str(); !ok {
					return nil, false
				}
			case "format":
				if f.Format, ok = p.str(); !ok {
					return nil, false
				}
			case "messages":
				if f.Messages, ok = p.messages(f.Messages); !ok {
					return nil, false
				}
			default:
				return nil, false
			}
			if p.ws().eat('}') {
				break
			}
			if !p.eat(',') {
				return nil, false
			}
			p.ws()
		}
	}
	if p.ws().i != len(p.s) {
		return nil, false // something other than whitespace after the top-level value
	}
	return f, true
}

func (p *scanner) ws() *scanner {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\n', '\r', '\t':
			p.i++
		default:
			return p
		}
	}
	return p
}

func (p *scanner) eat(c byte) bool {
	if p.i < len(p.s) && p.s[p.i] == c {
		p.i++
		return true
	}
	return false
}

// num reads a non-negative integer: digits with no leading zero, as JSON's grammar has it. Negative
// numbers, fractions and exponents are left to Parse.
func (p *scanner) num() (int, bool) {
	start := p.i
	for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
		p.i++
	}
	digits := p.s[start:p.i]
	if digits == "" || (digits[0] == '0' && len(digits) > 1) {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	return n, err == nil
}

// str reads a JSON string. Without escapes it returns a substring (no allocation).
func (p *scanner) str() (string, bool) {
	if !p.eat('"') {
		return "", false
	}
	start := p.i
	for p.i < len(p.s) {
		switch c := p.s[p.i]; {
		case c == '"':
			out := p.s[start:p.i]
			p.i++
			return out, true
		case c == '\\':
			return p.strSlow(start)
		case c < 0x20:
			return "", false
		default:
			p.i++
		}
	}
	return "", false
}

// strSlow handles strings with escapes, continuing from the first backslash.
func (p *scanner) strSlow(start int) (string, bool) {
	buf := []byte(p.s[start:p.i])
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == '"':
			p.i++
			return string(buf), true
		case c == '\\':
			if p.i+1 >= len(p.s) {
				return "", false
			}
			e := p.s[p.i+1]
			p.i += 2
			switch e {
			case '"', '\\', '/':
				buf = append(buf, e)
			case 'b':
				buf = append(buf, '\b')
			case 'f':
				buf = append(buf, '\f')
			case 'n':
				buf = append(buf, '\n')
			case 'r':
				buf = append(buf, '\r')
			case 't':
				buf = append(buf, '\t')
			case 'u':
				if p.i+4 > len(p.s) {
					return "", false
				}
				r, ok := hex4(p.s[p.i : p.i+4])
				if !ok {
					return "", false
				}
				p.i += 4
				if r >= 0xD800 && r < 0xDC00 {
					// A high surrogate pairs only with an escaped low surrogate right behind it.
					// Anything else leaves it unpaired: U+FFFD, and the next escape is decoded on its
					// own. (An unpaired low surrogate is not a valid rune, so AppendRune writes U+FFFD
					// for it too.) This is what encoding/json does.
					if r2, ok := p.lowSurrogate(); ok {
						r = (r-0xD800)<<10 + (r2 - 0xDC00) + 0x10000
						p.i += 6
					} else {
						r = utf8.RuneError
					}
				}
				buf = utf8.AppendRune(buf, rune(r))
			default:
				return "", false
			}
		case c < 0x20:
			return "", false
		default:
			buf = append(buf, c)
			p.i++
		}
	}
	return "", false
}

// lowSurrogate reads an escaped low surrogate (\uDC00 to \uDFFF) at the current position without
// consuming it.
func (p *scanner) lowSurrogate() (int, bool) {
	if p.i+6 > len(p.s) || p.s[p.i] != '\\' || p.s[p.i+1] != 'u' {
		return 0, false
	}
	r, ok := hex4(p.s[p.i+2 : p.i+6])
	if !ok || r < 0xDC00 || r >= 0xE000 {
		return 0, false
	}
	return r, true
}

// hex4 decodes four hexadecimal digits.
func hex4(s string) (int, bool) {
	r := 0
	for i := 0; i < 4; i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			c -= '0'
		case c >= 'a' && c <= 'f':
			c -= 'a' - 10
		case c >= 'A' && c <= 'F':
			c -= 'A' - 10
		default:
			return 0, false
		}
		r = r<<4 | int(c)
	}
	return r, true
}

// messages decodes a "messages" object into m, allocating it when nil. Decoding a repeated key into the
// map already built is what encoding/json does with a map field: later entries override earlier ones key
// by key, and an earlier map is never discarded.
func (p *scanner) messages(m map[string]string) (map[string]string, bool) {
	if !p.eat('{') {
		return nil, false
	}
	if m == nil {
		m = make(map[string]string, len(p.s)/120)
	}
	if p.ws().eat('}') {
		return m, true
	}
	for {
		k, ok := p.str()
		if !ok || !p.ws().eat(':') {
			return nil, false
		}
		p.ws()
		v, ok := p.str()
		if !ok {
			return nil, false
		}
		m[k] = v
		if p.ws().eat('}') {
			return m, true
		}
		if !p.eat(',') {
			return nil, false
		}
		p.ws()
	}
}
