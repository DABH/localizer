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
// entries. It accepts exactly the catalog shape ({"version": N, "language": "...", "messages": {...}})
// and falls back to Parse on anything else.
func ParseFast(data []byte) (*File, error) {
	if !utf8.Valid(data) {
		return Parse(data) // encoding/json's replacement-character semantics
	}
	s := string(data) // one copy; every substring below shares it
	p := scanner{s: s}
	f := &File{}
	if !p.ws().eat('{') {
		return Parse(data)
	}
	for {
		p.ws()
		if p.eat('}') {
			break
		}
		key, ok := p.str()
		if !ok || !p.ws().eat(':') {
			return Parse(data)
		}
		p.ws()
		switch key {
		case "version":
			n, ok := p.num()
			if !ok {
				return Parse(data)
			}
			f.Version = n
		case "language":
			if f.Language, ok = p.str(); !ok {
				return Parse(data)
			}
		case "messages":
			m, ok := p.messages(len(s) / 120)
			if !ok {
				return Parse(data)
			}
			f.Messages = m
		default:
			return Parse(data)
		}
		p.ws()
		if p.eat(',') {
			continue
		}
		if p.ws().eat('}') {
			break
		}
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

type scanner struct {
	s string
	i int
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

func (p *scanner) num() (int, bool) {
	start := p.i
	for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
		p.i++
	}
	n, err := strconv.Atoi(p.s[start:p.i])
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
				r, ok := p.hex4()
				if !ok {
					return "", false
				}
				if r >= 0xD800 && r < 0xDC00 { // surrogate pair
					if p.i+1 < len(p.s) && p.s[p.i] == '\\' && p.s[p.i+1] == 'u' {
						p.i += 2
						r2, ok := p.hex4()
						if !ok {
							return "", false
						}
						r = (r-0xD800)<<10 + (r2 - 0xDC00) + 0x10000
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

func (p *scanner) hex4() (int, bool) {
	if p.i+4 > len(p.s) {
		return 0, false
	}
	n, err := strconv.ParseUint(p.s[p.i:p.i+4], 16, 32)
	p.i += 4
	return int(n), err == nil
}

func (p *scanner) messages(sizeHint int) (map[string]string, bool) {
	if !p.eat('{') {
		return nil, false
	}
	m := make(map[string]string, sizeHint)
	for {
		p.ws()
		if p.eat('}') {
			return m, true
		}
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
		p.ws()
		if p.eat(',') {
			continue
		}
		if p.ws().eat('}') {
			return m, true
		}
		return nil, false
	}
}
