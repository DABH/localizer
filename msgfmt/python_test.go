// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package msgfmt

import "testing"

func TestParsePython(t *testing.T) {
	toks, ok := ParseSyntax("Created {name!r} with {n} items.", Python)
	if !ok || len(toks) != 5 {
		t.Fatalf("toks = %+v, ok = %v", toks, ok)
	}
	name, n := toks[1], toks[3]
	if name.Name != "name" || name.Conv != "r" || name.Verb != 'q' || name.Arg != 1 || name.Raw != "{name!r}" {
		t.Errorf("name token = %+v", name)
	}
	if n.Name != "n" || n.Conv != "" || n.Verb != 'v' || n.Arg != 2 || n.Auto {
		t.Errorf("n token = %+v", n)
	}
	if toks[0].Lit != "Created " || toks[2].Lit != " with " || toks[4].Lit != " items." {
		t.Errorf("literals = %+v", toks)
	}

	toks, ok = ParseSyntax("{0} and {1} and {0}", Python)
	if !ok || toks[0].Arg != 1 || toks[2].Arg != 2 || toks[4].Arg != 1 {
		t.Errorf("repeated field numbering: %+v, ok = %v", toks, ok)
	}
	toks, ok = ParseSyntax("{} and {}", Python)
	if !ok || !toks[0].Auto || toks[0].Name != "0" || toks[2].Name != "1" {
		t.Errorf("auto numbering: %+v, ok = %v", toks, ok)
	}
	toks, ok = ParseSyntax("%d items (%s)", Python)
	if !ok || toks[0].Conv != "d" || toks[0].Verb != 'd' || toks[0].Name != "0" || toks[2].Name != "1" || toks[2].Verb != 'v' {
		t.Errorf("printf: %+v, ok = %v", toks, ok)
	}
	toks, ok = ParseSyntax("%(age)03d", Python)
	if !ok || toks[0].Name != "age" || toks[0].Flags != "0" || toks[0].Width != "3" || toks[0].Spec != "03" {
		t.Errorf("named printf: %+v, ok = %v", toks, ok)
	}
	for _, bad := range []string{"{} and {0}", "%(name)s and %s", "50%", "%*d", "{x!q} y", "%Y-%m-%d", "{a} and {"} {
		if _, ok := ParseSyntax(bad, Python); ok {
			t.Errorf("ParseSyntax(%q) ok, want invalid", bad)
		}
	}
	for _, prose := range []string{"100% done", "lone } brace", "unterminated { brace", "{x:{", "plain", ""} {
		toks, ok := ParseSyntax(prose, Python)
		if !ok || HasFields(prose, Python) {
			t.Errorf("ParseSyntax(%q) = %+v, %v: want prose", prose, toks, ok)
		}
	}
	if !HasFields("Progress: {pct}%", Python) || !HasFields("%s items", Python) || HasFields("{{x}}", Python) {
		t.Error("HasFields")
	}
	if Python.String() != "python" || Go.String() != "go" {
		t.Error("Syntax.String")
	}
}

func TestGoSyntaxUnchanged(t *testing.T) {
	if _, ok := ParseSyntax("{name} and %s", Go); !ok {
		t.Error("Go syntax must ignore braces")
	}
	p := Extract("Created %s {x}")
	if len(p.Verbs) != 1 || p.Verbs[0] != "1:s" || p.Tags != nil {
		t.Errorf("Go extract = %+v", p)
	}
	if got := Pseudo("Hello {x}"); got != "⟦Ĥéļļö {ẋ}⟧" {
		t.Errorf("Go pseudo = %q", got)
	}
	pat, ok := Compile("Created %s.")
	if !ok || pat.Syntax() != Go {
		t.Fatal("Compile")
	}
	args, _, _ := pat.Match("Created x.")
	if out, ok := pat.Splice("%s を作成しました。", args, nil); !ok || out != "x を作成しました。" {
		t.Errorf("Splice method on a Go pattern = %q, %v", out, ok)
	}
}
