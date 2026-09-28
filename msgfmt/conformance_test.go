// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package msgfmt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The vectors in testdata/conformance are shared with the Python runtime; see the README there.

func loadCorpus(t *testing.T, name string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", "conformance", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func corpusSyntax(t *testing.T, s string) Syntax {
	t.Helper()
	switch s {
	case "python":
		return Python
	case "go":
		return Go
	}
	t.Fatalf("unknown syntax %q", s)
	return Go
}

func TestConformancePlaceholders(t *testing.T) {
	for _, name := range []string{"placeholders.json", "placeholders_go.json"} {
		var file struct {
			Syntax string
			Cases  []struct {
				Name, Source string
				Translation  *string
				Placeholders struct{ Fields, Actions, Backquoted, Tags []string }
				Valid        *bool
			}
		}
		loadCorpus(t, name, &file)
		syn := corpusSyntax(t, file.Syntax)
		for _, c := range file.Cases {
			got := ExtractSyntax(c.Source, syn)
			want := Placeholders{Verbs: c.Placeholders.Fields, Actions: c.Placeholders.Actions, Backquoted: c.Placeholders.Backquoted, Tags: c.Placeholders.Tags}
			if !got.Equal(want) {
				t.Errorf("%s: %s: placeholders of %q = %+v, want %+v", name, c.Name, c.Source, got, want)
			}
			if c.Translation != nil && c.Valid != nil {
				tr := *c.Translation
				valid := got.Equal(ExtractSyntax(tr, syn)) && !NewControlChars(c.Source, tr)
				if valid != *c.Valid {
					t.Errorf("%s: %s: valid(%q -> %q) = %v, want %v (diff: %s)", name, c.Name, c.Source, tr, valid, *c.Valid, got.Diff(ExtractSyntax(tr, syn)))
				}
			}
		}
	}
}

func TestConformanceRoundtrip(t *testing.T) {
	var file struct {
		Syntax string
		Cases  []struct {
			Name, Format string
			Compiles     *bool
			Input        *string
			Match        *bool
			Captures     map[string]string
			Classes      map[string]string
			Translation  *string
			Spliced      *string
			Splices      *bool
		}
	}
	for _, name := range []string{"roundtrip.json", "roundtrip_go.json"} {
		file.Cases = nil
		loadCorpus(t, name, &file)
		syn := corpusSyntax(t, file.Syntax)
		for _, c := range file.Cases {
			p, ok := CompileSyntax(c.Format, syn)
			if compiles := c.Compiles == nil || *c.Compiles; ok != compiles {
				t.Errorf("%s: compile(%q) = %v, want %v", c.Name, c.Format, ok, compiles)
				continue
			}
			if !ok || c.Input == nil {
				continue
			}
			args, verbs, matched := p.Match(*c.Input)
			if wantMatch := c.Match == nil || *c.Match; matched != wantMatch {
				t.Errorf("%s: match(%q) = %v, want %v", c.Name, *c.Input, matched, wantMatch)
				continue
			}
			if !matched {
				continue
			}
			byName := map[string]int{}
			for name, idx := range p.names {
				byName[name] = idx
			}
			if syn == Go {
				for _, tok := range p.toks {
					if tok.IsVerb() {
						byName[itoa(tok.Arg-1)] = tok.Arg
					}
				}
			}
			for name, want := range c.Captures {
				if got := args[byName[name]]; got != want {
					t.Errorf("%s: capture %s = %q, want %q", c.Name, name, got, want)
				}
			}
			if len(args) != len(c.Captures) {
				t.Errorf("%s: %d captures, want %d", c.Name, len(args), len(c.Captures))
			}
			for name, want := range c.Classes {
				if got := string(verbs[byName[name]]); got != want {
					t.Errorf("%s: class of %s = %q, want %q", c.Name, name, got, want)
				}
			}
			if c.Translation == nil {
				continue
			}
			out, spliced := p.Splice(*c.Translation, args, nil)
			if wantSplice := c.Splices == nil || *c.Splices; spliced != wantSplice {
				t.Errorf("%s: splice ok = %v, want %v", c.Name, spliced, wantSplice)
				continue
			}
			if spliced && c.Spliced != nil && out != *c.Spliced {
				t.Errorf("%s: spliced = %q, want %q", c.Name, out, *c.Spliced)
			}
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestConformancePseudo(t *testing.T) {
	var file struct {
		Syntax string
		Cases  []struct{ Input, Output string }
	}
	loadCorpus(t, "pseudo.json", &file)
	syn := corpusSyntax(t, file.Syntax)
	for _, c := range file.Cases {
		if got := PseudoSyntax(c.Input, syn); got != c.Output {
			t.Errorf("pseudo(%q) = %q, want %q", c.Input, got, c.Output)
		}
	}
}
