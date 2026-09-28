// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package msgfmt

import (
	"fmt"
	"sort"
	"strings"
)

// Placeholders is the set of things a translation must carry over unchanged from its source string.
type Placeholders struct {
	Verbs      []string // "argIndex:flags width .prec verb", sorted and de-duplicated
	Actions    []string // text/template actions, sorted and de-duplicated; "{{" stands for an unterminated action
	Backquoted []string // `...` spans: the first one first (pflag uses it as the flag's value name), then the rest sorted
	Tags       []string // Python: Rich markup tags and "\[" escapes, sorted, with duplicates; "[/?]" stands for unbalanced markup
}

// Extract returns the placeholders in a Go string. Verbs are only considered when s parses as a format
// string. See ExtractSyntax for other syntaxes.
func Extract(s string) Placeholders {
	var p Placeholders
	masked := s
	actions := FindActions(s)
	if len(actions) > 0 {
		set := map[string]bool{}
		var b strings.Builder
		last := 0
		for _, a := range actions {
			set[normalizeAction(s[a[0]:a[1]])] = true
			b.WriteString(s[last:a[0]])
			b.WriteString("\x00")
			last = a[1]
		}
		b.WriteString(s[last:])
		masked = b.String()
		for a := range set {
			p.Actions = append(p.Actions, a)
		}
	}
	if strings.Contains(masked, "{{") {
		// An unterminated action would make text/template refuse the whole template: a translation may
		// only contain one if the source does.
		p.Actions = append(p.Actions, "{{")
	}
	sort.Strings(p.Actions)
	if strings.Contains(masked, "%") {
		if toks, ok := Parse(masked); ok {
			set := map[string]bool{}
			for _, t := range toks {
				if t.IsVerb() {
					// Flags, width and precision are part of the signature: a translation must not be able to turn
					// "%s" into "%999999999s" (a memory bomb) or otherwise change how an argument is rendered.
					set[fmt.Sprintf("%d:%s%s%s%c", t.Arg, t.Flags, t.Width, t.Prec, t.Verb)] = true
				}
			}
			for v := range set {
				p.Verbs = append(p.Verbs, v)
			}
			sort.Strings(p.Verbs)
		}
	}
	p.Backquoted = orderedSpans(backquoted(masked))
	return p
}

// Equal reports whether two placeholder sets are identical.
func (p Placeholders) Equal(o Placeholders) bool {
	return equalStrings(p.Verbs, o.Verbs) && equalStrings(p.Actions, o.Actions) && equalStrings(p.Backquoted, o.Backquoted) && equalStrings(p.Tags, o.Tags)
}

// Diff describes how dst's placeholders differ from p (the source's). It returns "" when they match.
func (p Placeholders) Diff(dst Placeholders) string {
	var probs []string
	if !equalStrings(p.Verbs, dst.Verbs) {
		probs = append(probs, fmt.Sprintf("format verbs differ: source has %v, translation has %v", p.Verbs, dst.Verbs))
	}
	if !equalStrings(p.Actions, dst.Actions) {
		probs = append(probs, fmt.Sprintf("template actions differ: source has %q, translation has %q", p.Actions, dst.Actions))
	}
	if !equalStrings(p.Backquoted, dst.Backquoted) {
		probs = append(probs, fmt.Sprintf("backquoted spans differ: source has %q, translation has %q", p.Backquoted, dst.Backquoted))
	}
	if !equalStrings(p.Tags, dst.Tags) {
		probs = append(probs, fmt.Sprintf("markup tags differ: source has %q, translation has %q", p.Tags, dst.Tags))
	}
	return strings.Join(probs, "; ")
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// backquoted returns the `...` spans of s in order. An unpaired trailing backquote is reported as a lone
// "`" so that adding or removing a stray backquote is still detected.
func backquoted(s string) []string {
	var out []string
	for {
		i := strings.IndexByte(s, '`')
		if i < 0 {
			return out
		}
		j := strings.IndexByte(s[i+1:], '`')
		if j < 0 {
			return append(out, "`")
		}
		out = append(out, s[i:i+j+2])
		s = s[i+j+2:]
	}
}

// orderedSpans keeps the first span in place and sorts the rest: the first backquoted span of a flag
// description is what pflag shows as the flag's value name, so a translation must keep it first, while
// the others may move with the sentence.
func orderedSpans(spans []string) []string {
	if len(spans) > 1 {
		sort.Strings(spans[1:])
	}
	return spans
}

// NewControlChars reports control characters (other than \n and \t), a carriage return the source
// doesn't have, terminal escape sequences, bidirectional embeddings, overrides and isolates, line and
// paragraph separators, a byte order mark or tag characters present in dst but not in src. Translations
// must never smuggle terminal escapes into a CLI's output, nor reorder or hide what the user sees.
func NewControlChars(src, dst string) bool {
	for _, r := range dst {
		if controlRune(r) && !strings.ContainsRune(src, r) {
			return true
		}
	}
	return false
}

func controlRune(r rune) bool {
	switch {
	case r < 0x20:
		return r != '\n' && r != '\t'
	case r == 0x7f, r >= 0x80 && r < 0xa0:
		return true
	case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069: // bidi embeddings, overrides, isolates
		return true
	case r == 0x2028, r == 0x2029, r == 0xfeff:
		return true
	case r >= 0xe0000 && r <= 0xe007f: // tag characters
		return true
	}
	return false
}
