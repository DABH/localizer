// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package builtin

import (
	"testing"

	"github.com/DABH/localizer/msgfmt"
)

func TestBuiltinCatalogsAreValid(t *testing.T) {
	langs := Languages()
	if len(langs) < 7 {
		t.Fatalf("expected at least 7 built-in languages, got %v", langs)
	}
	var want map[string]string
	for _, lang := range langs {
		msgs := Messages(lang)
		if len(msgs) == 0 {
			t.Fatalf("%s: no messages", lang)
		}
		if want == nil {
			want = msgs
		}
		if len(msgs) != len(want) {
			t.Errorf("%s has %d entries, %s has %d", lang, len(msgs), langs[0], len(want))
		}
		for src, tr := range msgs {
			if _, ok := want[src]; !ok {
				t.Errorf("%s: unexpected key %q", lang, src)
			}
			if d := msgfmt.Extract(src).Diff(msgfmt.Extract(tr)); d != "" {
				t.Errorf("%s: %q -> %q: %s", lang, src, tr, d)
			}
			if msgfmt.NewControlChars(src, tr) {
				t.Errorf("%s: control characters in %q", lang, tr)
			}
		}
	}
	if Messages("es-MX") == nil || Messages("zh-CN") == nil {
		t.Error("regional variants should fall back to the base built-in catalog")
	}
	if Messages("zh-TW") != nil {
		t.Error("Traditional Chinese must not get Simplified built-ins")
	}
}
