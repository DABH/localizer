// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package localizer

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/DABH/localizer/engine"
)

// DumpEntry is one help string found in a command tree (LOCALIZER_DUMP output).
type DumpEntry struct {
	Kind       string `json:"kind"` // short, long, example, deprecated, group, flag, flag_deprecated
	Command    string `json:"command"`
	Flag       string `json:"flag,omitempty"`
	Text       string `json:"text"`
	Translated *bool  `json:"translated,omitempty"` // nil when no language is active
}

// Dump is the LOCALIZER_DUMP file format.
type Dump struct {
	Language string      `json:"language,omitempty"`
	Entries  []DumpEntry `json:"entries"`
}

// dumpTree records every help string in the tree (before translation) for coverage analysis.
func dumpTree(root *cobra.Command, path string, st *state) error {
	d := Dump{Entries: []DumpEntry{}}
	var eng *engine.Engine
	if st != nil {
		d.Language, eng = st.lang, st.eng
	}
	add := func(kind string, c *cobra.Command, flag, text string) {
		if text == "" {
			return
		}
		e := DumpEntry{Kind: kind, Command: c.CommandPath(), Flag: flag, Text: text}
		if eng != nil {
			hit := eng.Translate(text, engine.Help) != text
			e.Translated = &hit
		}
		d.Entries = append(d.Entries, e)
	}
	seen := map[*pflag.Flag]bool{}
	walk(root, func(c *cobra.Command) {
		add("short", c, "", c.Short)
		add("long", c, "", c.Long)
		add("example", c, "", c.Example)
		add("deprecated", c, "", c.Deprecated)
		for _, g := range c.Groups() {
			add("group", c, "", g.Title)
		}
		visit := func(f *pflag.Flag) {
			if seen[f] {
				return
			}
			seen[f] = true
			add("flag", c, f.Name, f.Usage)
			add("flag_deprecated", c, f.Name, f.Deprecated)
		}
		c.Flags().VisitAll(visit)
		c.PersistentFlags().VisitAll(visit)
	})
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
