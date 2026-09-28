// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Package engine looks up translations for strings a CLI prints: exact catalog hits, reverse-matched
// format strings, and composite text split into paragraphs, lines and "label: message" parts.
//
// CLIs use package localizer instead; engine is exported for tools that need exactly the runtime's matching,
// such as coverage reports.
package engine

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"github.com/DABH/localizer/msgfmt"
)

// Mode selects how aggressively composite strings are split.
type Mode uint8

const (
	// Output is for general program output. It never splits "label: rest", which could hit YAML or other
	// data lines.
	Output Mode = iota
	// Help is for command-tree help text and templates (all CLI-authored), including "DEPRECATED: ..."
	// style prefixes.
	Help
	// Error is for error messages: like Help, plus wrapped-error chains ("failed to X: %w").
	Error
	numModes
)

const (
	maxDepth     = 8
	maxMemoBytes = 1 << 20 // total size of memoized inputs and outputs
	maxMemoEntry = 1024    // longer strings are looked up every time rather than kept
	idxLen       = 4
	maxPrepass   = 64 << 10 // strings longer than this are not split
)

// Engine translates strings into one language. It is safe for concurrent use.
type Engine struct {
	lang   string
	syntax msgfmt.Syntax
	pseudo bool
	// layers are consulted last to first (the app's catalog overrides Localizer's built-ins). Catalog
	// keys are normally already trimmed; the rare untrimmed ones are indexed separately so that the large
	// app catalog never has to be copied at startup.
	layers []map[string]string
	// appFrom is the index of the first layer that belongs to the application's catalog (the last one
	// given to New); earlier layers are the runtime's built-in catalog.
	appFrom int

	OnMiss func(s string, mode Mode) // optional debug hook, called for letter-bearing misses in Help mode

	patOnce  sync.Once
	patterns []*msgfmt.Pattern
	patLayer []int // the layer each pattern's format came from
	multi    []int // patterns whose format spans lines
	index    map[string][]int
	wild     []int

	valid    sync.Map // source -> bool
	memo     [numModes]sync.Map
	memoSize atomic.Int64
}

// New builds an engine for lang from catalogs of Go strings; later catalogs override earlier ones (pass
// Localizer's built-in catalog first and the app's catalog last). When more than one catalog is given,
// the last one is the application's: its templates win ties against the built-in ones, and a lone
// identifier captured by a built-in template (a command or flag name) is never translated again.
func New(lang string, catalogs ...map[string]string) *Engine {
	return NewSyntax(lang, msgfmt.Go, catalogs...)
}

// NewSyntax is New for catalogs whose keys use the given placeholder syntax.
func NewSyntax(lang string, syn msgfmt.Syntax, catalogs ...map[string]string) *Engine {
	e := &Engine{lang: lang, syntax: syn}
	for i, c := range catalogs {
		if i == len(catalogs)-1 {
			e.appFrom = len(e.layers)
		}
		if len(c) == 0 {
			continue
		}
		e.layers = append(e.layers, c)
		if fixed := trimmedKeys(c); len(fixed) > 0 {
			e.layers = append(e.layers, fixed)
		}
	}
	return e
}

// trimmedKeys copies the entries whose key carries surrounding whitespace under the trimmed key, so that
// they are found by the trimmed lookups. An exact trimmed key wins over any copy, and when several keys
// trim to the same text the smallest original key wins, so the choice never depends on map order.
func trimmedKeys(c map[string]string) map[string]string {
	var fixed map[string]string
	origin := map[string]string{}
	for k, v := range c {
		_, core, _ := msgfmt.SplitSpace(k)
		if core == k {
			continue
		}
		if _, exact := c[core]; exact {
			continue
		}
		if prev, ok := origin[core]; ok && prev < k {
			continue
		}
		if fixed == nil {
			fixed = map[string]string{}
		}
		origin[core] = k
		fixed[core] = v
	}
	return fixed
}

// NewPseudo builds a pseudo-localizing engine that knows the given Go source strings.
func NewPseudo(catalogs ...map[string]string) *Engine {
	return NewPseudoSyntax(msgfmt.Go, catalogs...)
}

// NewPseudoSyntax is NewPseudo for source strings in the given placeholder syntax.
func NewPseudoSyntax(syn msgfmt.Syntax, catalogs ...map[string]string) *Engine {
	e := &Engine{lang: "qps", syntax: syn, pseudo: true}
	known := map[string]string{}
	for _, c := range catalogs {
		for k := range c {
			if _, core, _ := msgfmt.SplitSpace(k); core != "" {
				known[core] = core
			}
		}
	}
	e.layers = []map[string]string{known}
	return e
}

// Lang returns the engine's language tag.
func (e *Engine) Lang() string { return e.lang }

// Syntax returns the placeholder syntax of the engine's catalogs.
func (e *Engine) Syntax() msgfmt.Syntax { return e.syntax }

func (e *Engine) raw(core string) (string, bool) {
	for i := len(e.layers) - 1; i >= 0; i-- {
		if t, ok := e.layers[i][core]; ok {
			_, tcore, _ := msgfmt.SplitSpace(t)
			return tcore, tcore != ""
		}
	}
	return "", false
}

// Lookup translates s by exact catalog match only (surrounding whitespace is preserved). Use it for
// format strings that are about to be passed to fmt.Sprintf.
func (e *Engine) Lookup(s string) (string, bool) {
	lead, core, trail := msgfmt.SplitSpace(s)
	if core == "" {
		return s, false
	}
	t, ok := e.exactHit(core)
	if !ok {
		return s, false
	}
	return lead + t + trail, true
}

// Translate returns the translation of s, or s itself when nothing in the catalog applies.
func (e *Engine) Translate(s string, mode Mode) string {
	if s == "" || mode >= numModes {
		return s
	}
	// Help strings are translated once each at startup; memoizing them would only cost allocations.
	cacheable := len(s) <= maxMemoEntry && mode != Help
	if cacheable {
		if v, ok := e.memo[mode].Load(s); ok {
			return v.(string)
		}
	}
	out := e.translate(s, mode, 0)
	if out == s && e.OnMiss != nil && mode == Help {
		if _, core, _ := msgfmt.SplitSpace(s); msgfmt.HasLetter(core) {
			e.OnMiss(s, mode)
		}
	}
	if cacheable && e.memoSize.Load() < maxMemoBytes {
		if _, loaded := e.memo[mode].LoadOrStore(s, out); !loaded {
			e.memoSize.Add(int64(len(s) + len(out)))
		}
	}
	return out
}

func (e *Engine) translate(s string, mode Mode, depth int) string {
	if depth > maxDepth {
		return s
	}
	lead, core, trail := msgfmt.SplitSpace(s)
	if core == "" || !msgfmt.HasLetter(core) {
		return s
	}
	if t, ok := e.exactHit(core); ok {
		return lead + t + trail
	}
	multiline := strings.Contains(core, "\n")
	// Help text with several lines is almost always composed (Examples, Long with code blocks): try its
	// parts before any single-line pattern, which saves regex work at startup. Formats that span lines
	// themselves (Cobra's completion help) are tried first, or their first line would match a one-line
	// format and capture the rest as a value.
	if multiline && mode == Help && len(core) <= maxPrepass {
		if t, ok := e.multilineHit(core, mode, depth); ok {
			return lead + t + trail
		}
		if t, ok := e.segments(core, mode, depth); ok {
			return lead + t + trail
		}
	}
	if t, ok := e.patternHit(core, mode, depth); ok {
		return lead + t + trail
	}
	if len(core) > maxPrepass {
		return s
	}
	if multiline && mode != Help {
		if t, ok := e.segments(core, mode, depth); ok {
			return lead + t + trail
		}
		return s
	}
	if multiline {
		return s
	}
	if mode != Output {
		if t, ok := e.labelSplit(core, mode, depth); ok {
			return lead + t + trail
		}
	}
	return s
}

// segments translates paragraphs, or lines when there is a single paragraph.
func (e *Engine) segments(core string, mode Mode, depth int) (string, bool) {
	if strings.Contains(core, "\n\n") {
		return e.joinParts(strings.Split(core, "\n\n"), "\n\n", mode, depth)
	}
	return e.joinParts(strings.Split(core, "\n"), "\n", mode, depth)
}

func (e *Engine) joinParts(parts []string, sep string, mode Mode, depth int) (string, bool) {
	changed := false
	for i, p := range parts {
		if mode == Help && strings.HasPrefix(strings.TrimSpace(p), "$ ") {
			continue // shell example
		}
		if mode == Error && identifierLine(p) {
			continue // a suggested command or flag name, never a message
		}
		if np := e.translate(p, mode, depth+1); np != p {
			parts[i] = np
			changed = true
		}
	}
	return strings.Join(parts, sep), changed
}

// identifierLine reports an indented line that holds a single identifier-like token, such as the
// command names Cobra lists under "Did you mean this?".
func identifierLine(line string) bool {
	return line != "" && (line[0] == '\t' || line[0] == ' ') && identifierToken(strings.TrimSpace(line))
}

// identifierToken reports a single token made of letters, digits and "-_./:": a command name, a flag
// name, a path. Never a message.
func identifierToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("-_./:", r) {
			return false
		}
	}
	return true
}

// labelSplit handles "Label: rest" by translating the label (with or without its colon) and the rest
// independently, e.g. "DEPRECATED: Manage clusters." or "failed to create topic: <wrapped error>".
func (e *Engine) labelSplit(core string, mode Mode, depth int) (string, bool) {
	i := strings.Index(core, ": ")
	if i <= 0 {
		return "", false
	}
	label, rest := core[:i+1], core[i+2:]
	newLabel := label
	if t, ok := e.exactHit(label); ok {
		newLabel = t
	} else if t, ok := e.exactHit(label[:len(label)-1]); ok {
		newLabel = t + ":"
	} else if t, ok := e.patternHit(label[:len(label)-1], mode, depth); ok {
		newLabel = t + ":"
	}
	newRest := e.translate(rest, mode, depth+1)
	if newLabel == label && newRest == rest {
		return "", false
	}
	sep := " "
	if strings.HasSuffix(newLabel, "：") { // full-width colon already carries its spacing
		sep = ""
	}
	return newLabel + sep + newRest, true
}

func (e *Engine) exactHit(core string) (string, bool) {
	t, ok := e.raw(core)
	if !ok {
		return "", false
	}
	if e.pseudo {
		return msgfmt.PseudoSyntax(core, e.syntax), true
	}
	if !e.isValid(core, t) {
		return "", false
	}
	return t, true
}

// isValid checks (once per entry) that a translation keeps the source's placeholders and adds no control
// characters. Invalid entries behave as if they were missing, so the CLI falls back to English.
func (e *Engine) isValid(src, tr string) bool {
	if v, ok := e.valid.Load(src); ok {
		return v.(bool)
	}
	ok := msgfmt.ExtractSyntax(src, e.syntax).Equal(msgfmt.ExtractSyntax(tr, e.syntax)) && !msgfmt.NewControlChars(src, tr)
	e.valid.Store(src, ok)
	return ok
}

var sentenceBreak = regexp.MustCompile(`[.!?。！？]\s*\p{Lu}`)

// plausible rejects reverse matches that would splice untranslated prose into a translation. A capture
// stands for a value (a name, an ID, a number): a generic pattern (little literal text, like "Manage %s.")
// must not capture a lowercase phrase, and outside error messages a capture must not span sentences or
// lines, nor, in help text, end a sentence. Wrapped errors (%w) are exempt from everything, and captures
// in error messages may span lines and sentences: they legitimately carry server or library text, and
// Cobra's "unknown command" error carries its suggestions in a %s.
func plausible(p *msgfmt.Pattern, args map[int]string, verbs map[int]rune, mode Mode) bool {
	generic := p.Letters() < 10
	for a, c := range args {
		if verbs[a] == 'w' {
			continue
		}
		if generic && strings.Contains(c, " ") {
			if r, _ := utf8.DecodeRuneInString(c); unicode.IsLower(r) {
				return false
			}
		}
		if mode == Error {
			continue
		}
		if strings.Contains(c, "\n") || sentenceBreak.MatchString(c) {
			return false
		}
		if mode == Help && strings.Contains(c, " ") && endsSentence(c) {
			return false
		}
	}
	return true
}

// endsSentence reports whether s ends with a sentence-ending mark.
func endsSentence(s string) bool {
	r, size := utf8.DecodeLastRuneInString(s)
	return size > 0 && strings.ContainsRune(".!?。！？", r)
}

func (e *Engine) buildPatterns() {
	e.index = map[string][]int{}
	marks := "%"
	if e.syntax == msgfmt.Python {
		marks = "%{"
	}
	layerOf := map[string]int{} // format -> the highest layer holding it
	for i, layer := range e.layers {
		for k := range layer {
			if strings.ContainsAny(k, marks) {
				_, core, _ := msgfmt.SplitSpace(k)
				layerOf[core] = i
			}
		}
	}
	keys := make([]string, 0, len(layerOf))
	for k := range layerOf {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic candidate order
	for _, k := range keys {
		p, ok := msgfmt.CompileSyntax(k, e.syntax)
		if !ok {
			continue
		}
		i := len(e.patterns)
		e.patterns = append(e.patterns, p)
		e.patLayer = append(e.patLayer, layerOf[k])
		if strings.Contains(k, "\n") {
			e.multi = append(e.multi, i)
		}
		pre := p.Prefix()
		if pre == "" {
			e.wild = append(e.wild, i)
			continue
		}
		if len(pre) > idxLen {
			pre = pre[:idxLen]
		}
		e.index[pre] = append(e.index[pre], i)
	}
}

// match is the best reverse match found so far.
type match struct {
	p     *msgfmt.Pattern
	layer int
	args  map[int]string
	verbs map[int]rune
}

// consider tries pattern i against core and keeps it when it beats the current best: more literal text
// wins; at equal specificity the application's catalog beats the built-in one, and within a layer the
// first candidate (formats are sorted) wins.
func (e *Engine) consider(m *match, i int, core string, mode Mode) {
	p := e.patterns[i]
	if m.p != nil {
		if s, best := p.Specificity(), m.p.Specificity(); s < best || (s == best && e.patLayer[i] <= m.layer) {
			return
		}
	}
	if args, verbs, ok := p.Match(core); ok && plausible(p, args, verbs, mode) {
		m.p, m.layer, m.args, m.verbs = p, e.patLayer[i], args, verbs
	}
}

func (e *Engine) patternHit(core string, mode Mode, depth int) (string, bool) {
	e.patOnce.Do(e.buildPatterns)
	if len(e.patterns) == 0 {
		return "", false
	}
	var m match
	for n := 1; n <= idxLen && n <= len(core); n++ {
		for _, i := range e.index[core[:n]] {
			e.consider(&m, i, core, mode)
		}
	}
	for _, i := range e.wild {
		e.consider(&m, i, core, mode)
	}
	return e.render(&m, mode, depth)
}

// multilineHit tries only the formats that span lines.
func (e *Engine) multilineHit(core string, mode Mode, depth int) (string, bool) {
	e.patOnce.Do(e.buildPatterns)
	var m match
	for _, i := range e.multi {
		e.consider(&m, i, core, mode)
	}
	return e.render(&m, mode, depth)
}

// render splices the captured values into the best match's translation. Wrapped errors are translated
// again; so are %v captures outside output mode, except a lone identifier captured by a built-in format
// (Cobra's "Run '%v --help'" carries a command name there, never a message).
func (e *Engine) render(m *match, mode Mode, depth int) (string, bool) {
	if m.p == nil {
		return "", false
	}
	var translation string
	if e.pseudo {
		translation = msgfmt.PseudoSyntax(m.p.Format, e.syntax)
	} else {
		t, ok := e.exactHit(m.p.Format)
		if !ok {
			return "", false
		}
		translation = t
	}
	builtin := m.layer < e.appFrom
	return m.p.Splice(translation, m.args, func(arg int, text string) string {
		switch m.verbs[arg] {
		case 'w':
			return e.translate(text, Error, depth+1)
		case 'v':
			if mode != Output && !(builtin && identifierToken(text)) {
				return e.translate(text, mode, depth+1)
			}
		}
		return text
	})
}
