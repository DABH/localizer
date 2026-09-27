// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/DABH/localizer/msgfmt"
)

// TestConformanceEngine runs testdata/conformance/engine.json, which the Python runtime runs too.
func TestConformanceEngine(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "conformance", "engine.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Groups []struct {
			Name, Syntax string
			Pseudo       bool
			Catalog      map[string]string
			Cases        []struct{ Input, Mode, Want string }
		}
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	modes := map[string]Mode{"output": Output, "help": Help, "error": Error}
	for _, g := range file.Groups {
		syn := msgfmt.Go
		if g.Syntax == "python" {
			syn = msgfmt.Python
		}
		var e *Engine
		if g.Pseudo {
			e = NewPseudoSyntax(syn, g.Catalog)
		} else {
			e = NewSyntax("ja", syn, g.Catalog)
		}
		for _, c := range g.Cases {
			mode, ok := modes[c.Mode]
			if !ok {
				t.Fatalf("%s: unknown mode %q", g.Name, c.Mode)
			}
			if got := e.Translate(c.Input, mode); got != c.Want {
				t.Errorf("%s: translate(%q, %s)\n got %q\nwant %q", g.Name, c.Input, c.Mode, got, c.Want)
			}
		}
	}
}
