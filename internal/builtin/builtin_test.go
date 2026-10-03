// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package builtin

import (
	"testing"

	"github.com/DABH/localizer/catalog"
	"github.com/DABH/localizer/msgfmt"
)

// Every framework directory translates the same set of strings into the same languages, with intact
// placeholders and no control characters.
func TestBuiltinCatalogsAreValid(t *testing.T) {
	fws := Frameworks()
	if len(fws) == 0 || fws[0] != "cobra" {
		t.Fatalf("frameworks %v: expected cobra first", fws)
	}
	all := Languages()
	if len(all) < 7 {
		t.Fatalf("expected at least 7 built-in languages, got %v", all)
	}
	for _, fw := range fws {
		fsys := Catalogs(fw)
		langs := catalog.Languages(fsys)
		if len(langs) != len(all) {
			t.Errorf("%s: languages %v, want %v", fw, langs, all)
		}
		var want map[string]string
		var first string
		for _, lang := range langs {
			f, err := catalog.Load(fsys, lang)
			if err != nil {
				t.Fatalf("%s/%s: %v", fw, lang, err)
			}
			msgs := f.Messages
			if len(msgs) == 0 {
				t.Fatalf("%s/%s: no messages", fw, lang)
			}
			if want == nil {
				want, first = msgs, lang
			}
			if len(msgs) != len(want) {
				t.Errorf("%s/%s has %d entries, %s has %d", fw, lang, len(msgs), first, len(want))
			}
			for src, tr := range msgs {
				if _, ok := want[src]; !ok {
					t.Errorf("%s/%s: unexpected key %q", fw, lang, src)
				}
				if d := msgfmt.Extract(src).Diff(msgfmt.Extract(tr)); d != "" {
					t.Errorf("%s/%s: %q -> %q: %s", fw, lang, src, tr, d)
				}
				if msgfmt.NewControlChars(src, tr) {
					t.Errorf("%s/%s: control characters in %q", fw, lang, tr)
				}
			}
		}
	}
	for _, lang := range all {
		if len(Messages(lang)) == 0 {
			t.Errorf("%s: no merged messages", lang)
		}
	}
	if Messages("ja")["Usage:"] == "" {
		t.Error("the merged catalog lacks Cobra's strings")
	}
	if Messages("es-MX") == nil || Messages("zh-CN") == nil {
		t.Error("regional variants should fall back to the base built-in catalog")
	}
	if Messages("zh-TW") != nil {
		t.Error("Traditional Chinese must not get Simplified built-ins")
	}
	if Catalogs("no-such-framework") != nil {
		t.Error("an unknown framework should have no catalogs")
	}
}
