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
	maxDepth   = 8
	maxMemo    = 8192
	idxLen     = 4
	maxPrepass = 64 << 10 // strings longer than this are not memoized or split
)

// Engine translates strings into one language. It is safe for concurrent use.
type Engine struct {
	lang   string
	pseudo bool
	// layers are consulted last to first (the app's catalog overrides Localizer's built-ins). Catalog
	// keys are normally already trimmed; the rare untrimmed ones are indexed separately so that the large
	// app catalog never has to be copied at startup.
	layers []map[string]string

	OnMiss func(s string, mode Mode) // optional debug hook, called for letter-bearing misses in Help mode

	patOnce  sync.Once
	patterns []*msgfmt.Pattern
	index    map[string][]int
	wild     []int

	valid    sync.Map // source -> bool
	memo     [numModes]sync.Map
	memoSize atomic.Int64
}

// New builds an engine for lang from catalogs; later catalogs override earlier ones (pass Localizer's
// built-in catalog first and the app's catalog last).
func New(lang string, catalogs ...map[string]string) *Engine {
	e := &Engine{lang: lang}
	for _, c := range catalogs {
		if len(c) == 0 {
			continue
		}
		var fixed map[string]string
		for k, v := range c {
			if _, core, _ := msgfmt.SplitSpace(k); core != k {
				if fixed == nil {
					fixed = map[string]string{}
				}
				fixed[core] = v
			}
		}
		e.layers = append(e.layers, c)
		if fixed != nil {
			e.layers = append(e.layers, fixed)
		}
	}
	return e
}

// NewPseudo builds a pseudo-localizing engine that knows the given source strings.
func NewPseudo(catalogs ...map[string]string) *Engine {
	e := &Engine{lang: "qps", pseudo: true}
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
	cacheable := len(s) <= maxPrepass && mode != Help
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
	if cacheable && e.memoSize.Load() < maxMemo {
		if _, loaded := e.memo[mode].LoadOrStore(s, out); !loaded {
			e.memoSize.Add(1)
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
	// parts before any pattern, which saves regex work at startup.
	if multiline && mode == Help && len(core) <= maxPrepass {
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
		if np := e.translate(p, mode, depth+1); np != p {
			parts[i] = np
			changed = true
		}
	}
	return strings.Join(parts, sep), changed
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
		return msgfmt.Pseudo(core), true
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
	ok := msgfmt.Extract(src).Equal(msgfmt.Extract(tr)) && !msgfmt.NewControlChars(src, tr)
	e.valid.Store(src, ok)
	return ok
}

var sentenceBreak = regexp.MustCompile(`[.!?。！？]\s*\p{Lu}`)

// plausible rejects reverse matches that would splice untranslated prose into a translation. A capture
// stands for a value (a name, an ID, a number): it must not span sentences or lines, and a generic
// pattern (little literal text, like "Manage %s.") must not capture a lowercase phrase. Error mode is
// exempt for wrapped errors (%w, %v), whose captures legitimately carry long server or library text.
func plausible(p *msgfmt.Pattern, args map[int]string, verbs map[int]rune, mode Mode) bool {
	generic := p.Letters() < 10
	for a, c := range args {
		v := verbs[a]
		if v == 'w' || (mode == Error && v == 'v') {
			continue
		}
		if mode != Error && (strings.Contains(c, "\n") || sentenceBreak.MatchString(c)) {
			return false
		}
		if generic && mode != Error && strings.Contains(c, " ") {
			if r, _ := utf8.DecodeRuneInString(c); unicode.IsLower(r) {
				return false
			}
		}
	}
	return true
}

func (e *Engine) buildPatterns() {
	e.index = map[string][]int{}
	set := map[string]bool{}
	for _, layer := range e.layers {
		for k := range layer {
			if strings.IndexByte(k, '%') >= 0 {
				_, core, _ := msgfmt.SplitSpace(k)
				set[core] = true
			}
		}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic candidate order
	for _, k := range keys {
		p, ok := msgfmt.Compile(k)
		if !ok {
			continue
		}
		i := len(e.patterns)
		e.patterns = append(e.patterns, p)
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

func (e *Engine) patternHit(core string, mode Mode, depth int) (string, bool) {
	e.patOnce.Do(e.buildPatterns)
	if len(e.patterns) == 0 {
		return "", false
	}
	var best *msgfmt.Pattern
	var bestArgs map[int]string
	var bestVerbs map[int]rune
	try := func(i int) {
		p := e.patterns[i]
		if best != nil && p.Specificity() <= best.Specificity() {
			return
		}
		if args, verbs, ok := p.Match(core); ok && plausible(p, args, verbs, mode) {
			best, bestArgs, bestVerbs = p, args, verbs
		}
	}
	for n := 1; n <= idxLen && n <= len(core); n++ {
		for _, i := range e.index[core[:n]] {
			try(i)
		}
	}
	for _, i := range e.wild {
		try(i)
	}
	if best == nil {
		return "", false
	}
	var translation string
	if e.pseudo {
		translation = msgfmt.Pseudo(best.Format)
	} else {
		t, ok := e.exactHit(best.Format)
		if !ok {
			return "", false
		}
		translation = t
	}
	return msgfmt.Splice(translation, bestArgs, func(arg int, text string) string {
		switch bestVerbs[arg] {
		case 'w':
			return e.translate(text, Error, depth+1)
		case 'v':
			if mode != Output {
				return e.translate(text, mode, depth+1)
			}
		}
		return text
	})
}
