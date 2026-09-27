// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Package catalog reads and writes Localizer translation catalogs: one JSON file per language, named
// "<BCP 47 tag>.json", mapping each English source string to its translation.
//
// CLIs don't use this package directly (package localizer loads their catalogs); it is exported for tools
// that read, write or check catalogs.
package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Version is the catalog schema version written by this package.
const Version = 1

// File is one language's catalog.
type File struct {
	Version  int    `json:"version"`
	Language string `json:"language"`
	// Format names the placeholder syntax of the keys: "" for Go strings, "python" for str.format fields
	// and printf-style verbs.
	Format   string            `json:"format,omitempty"`
	Messages map[string]string `json:"messages"`
}

// Languages lists the catalog languages in fsys: "<tag>.json" files at the root. Names starting with "_"
// or "." are reserved for tooling and ignored.
func Languages(fsys fs.FS) []string {
	if fsys == nil {
		return nil
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		out = append(out, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(out)
	return out
}

// Load reads the catalog for lang from fsys.
func Load(fsys fs.FS, lang string) (*File, error) {
	data, err := fs.ReadFile(fsys, path.Clean(lang)+".json")
	if err != nil {
		return nil, err
	}
	return ParseFast(data)
}

// Parse decodes a catalog.
func Parse(data []byte) (*File, error) {
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	if f.Version > Version {
		return nil, fmt.Errorf("catalog: unsupported version %d (this build understands up to %d)", f.Version, Version)
	}
	if f.Messages == nil {
		f.Messages = map[string]string{}
	}
	return &f, nil
}

// Marshal renders f canonically (sorted keys, 2-space indent, no HTML escaping, trailing newline) so that
// catalogs diff cleanly in pull requests.
func Marshal(f *File) ([]byte, error) {
	out := *f
	if out.Version == 0 {
		out.Version = Version
	}
	if out.Messages == nil {
		out.Messages = map[string]string{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
