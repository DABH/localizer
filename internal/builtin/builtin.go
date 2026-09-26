// Package builtin embeds Localizer's own translations of the strings Cobra and pflag print (help
// headings, the help/completion commands, argument and flag errors). An application's catalog overrides
// these entry by entry.
package builtin

import (
	"embed"
	"sync"

	"github.com/DABH/localizer/catalog"
	"github.com/DABH/localizer/internal/locale"
)

//go:embed *.json
var files embed.FS

var (
	mu    sync.Mutex
	cache = map[string]map[string]string{}
)

// Languages lists the languages with built-in translations.
func Languages() []string { return catalog.Languages(files) }

// Messages returns the built-in messages that best match lang, or nil if none match well.
func Messages(lang string) map[string]string {
	best := locale.Match([]string{lang}, Languages())
	if best == "" {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if m, ok := cache[best]; ok {
		return m
	}
	f, err := catalog.Load(files, best)
	if err != nil {
		return nil
	}
	cache[best] = f.Messages
	return f.Messages
}
