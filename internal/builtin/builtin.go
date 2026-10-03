// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Package builtin embeds Localizer's own translations of the strings the supported CLI frameworks print
// (help headings, the help and completion commands, argument and flag errors). Each framework has a
// directory of "<language>.json" catalogs: cobra/ for Cobra and pflag, and one per adapter package.
// Messages merges them, so an application sees every framework's strings; an application's catalog
// overrides these entry by entry.
package builtin

import (
	"embed"
	"io/fs"
	"sort"
	"sync"

	"github.com/DABH/localizer/catalog"
	"github.com/DABH/localizer/internal/locale"
)

//go:embed */*.json
var files embed.FS

var (
	mu    sync.Mutex
	cache = map[string]map[string]string{}
)

// Frameworks lists the framework directories, in the order their entries are merged.
func Frameworks() []string {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// Catalogs returns one framework's catalogs, or nil if there is no such directory.
func Catalogs(framework string) fs.FS {
	sub, err := fs.Sub(files, framework)
	if err != nil {
		return nil
	}
	if _, err := fs.ReadDir(sub, "."); err != nil {
		return nil
	}
	return sub
}

// Languages lists the languages with built-in translations for at least one framework.
func Languages() []string {
	seen := map[string]bool{}
	for _, fw := range Frameworks() {
		for _, l := range catalog.Languages(Catalogs(fw)) {
			seen[l] = true
		}
	}
	out := make([]string, 0, len(seen))
	for l := range seen {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// Messages returns the built-in messages of every framework that best match lang, or nil if none match
// well. When two frameworks translate the same string, the first directory in Frameworks order wins.
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
	merged := map[string]string{}
	for _, fw := range Frameworks() {
		f, err := catalog.Load(Catalogs(fw), best)
		if err != nil {
			continue
		}
		for k, v := range f.Messages {
			if _, dup := merged[k]; !dup {
				merged[k] = v
			}
		}
	}
	if len(merged) == 0 {
		return nil
	}
	cache[best] = merged
	return merged
}
