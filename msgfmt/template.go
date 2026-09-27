// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package msgfmt

import (
	"strconv"
	"strings"
)

// FindActions returns the [start, end) byte offsets of every "{{ ... }}" action in s. Quoted strings
// inside an action may contain "}}". An unterminated action ends the scan.
func FindActions(s string) [][2]int {
	var out [][2]int
	for i := 0; i < len(s); {
		j := strings.Index(s[i:], "{{")
		if j < 0 {
			break
		}
		start := i + j
		end := actionEnd(s, start+2)
		if end < 0 {
			break
		}
		out = append(out, [2]int{start, end})
		i = end
	}
	return out
}

// actionEnd returns the offset just past the "}}" that closes an action whose body starts at i.
func actionEnd(s string, i int) int {
	for i < len(s) {
		switch c := s[i]; c {
		case '"', '\'':
			i++
			for i < len(s) && s[i] != c {
				if s[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case '`':
			k := strings.IndexByte(s[i+1:], '`')
			if k < 0 {
				return -1
			}
			i += k + 2
		case '}':
			if strings.HasPrefix(s[i:], "}}") {
				return i + 2
			}
			i++
		default:
			i++
		}
	}
	return -1
}

func actionBody(a string) string {
	body := strings.TrimSuffix(strings.TrimPrefix(a, "{{"), "}}")
	body = strings.TrimPrefix(body, "- ")
	body = strings.TrimSuffix(body, " -")
	return strings.TrimSpace(body)
}

func normalizeAction(a string) string {
	return "{{" + strings.Join(strings.Fields(actionBody(a)), " ") + "}}"
}

var controlKeywords = map[string]bool{
	"if": true, "else": true, "end": true, "range": true, "with": true, "define": true,
	"template": true, "block": true, "break": true, "continue": true,
}

// isControl reports whether an action produces no text of its own (control flow, comments, variable
// declarations). Translatable units never span a control action.
func isControl(a string) bool {
	body := actionBody(a)
	if body == "" || strings.HasPrefix(body, "/*") {
		return true
	}
	first := body
	if k := strings.IndexAny(body, " \t\n("); k >= 0 {
		first = body[:k]
	}
	if controlKeywords[first] {
		return true
	}
	if strings.HasPrefix(body, "$") {
		rest := strings.TrimSpace(strings.TrimLeft(body[1:], "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"))
		if strings.HasPrefix(rest, ":=") || (strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==")) {
			return true
		}
	}
	return false
}

// TemplateUnits returns the translatable units of a text/template: trimmed lines of literal text that
// contain letters outside actions (value actions such as {{.CommandPath}} stay inline as placeholders),
// and letter-bearing string literals of printf/print actions such as {{printf "version %s" .Version}}.
func TemplateUnits(tpl string) []string {
	var units []string
	TranslateTemplate(tpl, func(u string) (string, bool) {
		units = append(units, u)
		return "", false
	})
	return units
}

// TranslateTemplate rewrites tpl, replacing every translatable unit u (see TemplateUnits) with tr(u)
// when tr reports ok. Control actions and everything else are preserved byte for byte.
func TranslateTemplate(tpl string, tr func(unit string) (string, bool)) string {
	acts := FindActions(tpl)
	var out strings.Builder
	segStart := 0
	flush := func(seg string) {
		lines := strings.Split(seg, "\n")
		for i, line := range lines {
			if i > 0 {
				out.WriteByte('\n')
			}
			out.WriteString(translateTemplateLine(line, tr))
		}
	}
	for _, a := range acts {
		action := tpl[a[0]:a[1]]
		if !isControl(action) {
			continue
		}
		flush(tpl[segStart:a[0]])
		out.WriteString(action)
		segStart = a[1]
	}
	flush(tpl[segStart:])
	return out.String()
}

func translateTemplateLine(line string, tr func(string) (string, bool)) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return line
	}
	acts := FindActions(trimmed)
	outside := trimmed
	if len(acts) > 0 {
		var b strings.Builder
		last := 0
		for _, a := range acts {
			b.WriteString(trimmed[last:a[0]])
			last = a[1]
		}
		b.WriteString(trimmed[last:])
		outside = b.String()
	}
	if HasLetter(outside) {
		if t, ok := tr(trimmed); ok {
			lead := line[:strings.Index(line, trimmed)]
			return lead + t + line[len(lead)+len(trimmed):]
		}
		return line
	}
	// No prose outside actions: look for printf string literals inside value actions.
	if len(acts) == 0 {
		return line
	}
	lineActs := FindActions(line)
	var b strings.Builder
	last := 0
	for _, a := range lineActs {
		b.WriteString(line[last:a[0]])
		b.WriteString(translatePrintfLiterals(line[a[0]:a[1]], tr))
		last = a[1]
	}
	b.WriteString(line[last:])
	return b.String()
}

func translatePrintfLiterals(action string, tr func(string) (string, bool)) string {
	body := actionBody(action)
	if !strings.HasPrefix(body, "printf ") && !strings.HasPrefix(body, "print ") {
		return action
	}
	var b strings.Builder
	for i := 0; i < len(action); {
		if action[i] != '"' {
			b.WriteByte(action[i])
			i++
			continue
		}
		j := i + 1
		for j < len(action) && action[j] != '"' {
			if action[j] == '\\' {
				j++
			}
			j++
		}
		if j >= len(action) {
			b.WriteString(action[i:])
			break
		}
		lit := action[i : j+1]
		i = j + 1
		s, err := strconv.Unquote(lit)
		if err == nil && HasLetter(s) {
			if t, ok := tr(s); ok {
				b.WriteString(strconv.Quote(t))
				continue
			}
		}
		b.WriteString(lit)
	}
	return b.String()
}
