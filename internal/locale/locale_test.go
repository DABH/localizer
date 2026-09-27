// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package locale

import (
	"os"
	"reflect"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDetect(t *testing.T) {
	osLangs := func() []string { return []string{"ko-KR", "en-US"} }
	tests := []struct {
		name string
		env  map[string]string
		want Result
	}{
		{"override wins", map[string]string{"APP_LANG": "fr", "LOCALIZER_LANG": "de", "LANG": "ja_JP.UTF-8"}, Result{Tags: []string{"fr"}, Source: "APP_LANG"}},
		{"localizer override", map[string]string{"LOCALIZER_LANG": "pt_BR", "LANG": "ja_JP.UTF-8"}, Result{Tags: []string{"pt-BR"}, Source: "LOCALIZER_LANG"}},
		{"off", map[string]string{"LOCALIZER_LANG": "off", "LANG": "ja_JP.UTF-8"}, Result{Off: true, Source: "LOCALIZER_LANG"}},
		{"pseudo", map[string]string{"LOCALIZER_LANG": "qps"}, Result{Pseudo: true, Source: "LOCALIZER_LANG"}},
		{"LC_ALL beats LANG", map[string]string{"LC_ALL": "de_DE.UTF-8", "LANG": "ja_JP.UTF-8"}, Result{Tags: []string{"de-DE"}, Source: "LC_ALL"}},
		{"LC_MESSAGES beats LANG", map[string]string{"LC_MESSAGES": "es_MX", "LANG": "ja_JP.UTF-8"}, Result{Tags: []string{"es-MX"}, Source: "LC_MESSAGES"}},
		{"C means English", map[string]string{"LC_ALL": "C", "LANGUAGE": "ja", "LANG": "ja_JP.UTF-8"}, Result{Tags: []string{"en"}, Source: "LC_ALL"}},
		{"C.UTF-8 means English", map[string]string{"LANG": "C.UTF-8"}, Result{Tags: []string{"en"}, Source: "LANG"}},
		{"LANGUAGE list first", map[string]string{"LANGUAGE": "zh_TW:ja", "LANG": "en_US.UTF-8"}, Result{Tags: []string{"zh-TW", "ja", "en-US"}, Source: "LANG"}},
		{"LANGUAGE alone", map[string]string{"LANGUAGE": "ko"}, Result{Tags: []string{"ko"}, Source: "LANGUAGE"}},
		{"only LC_CTYPE falls through to OS", map[string]string{"LC_CTYPE": "UTF-8"}, Result{Tags: []string{"ko-KR", "en-US"}, Source: "os"}},
		{"nothing", map[string]string{}, Result{Tags: []string{"ko-KR", "en-US"}, Source: "os"}},
	}
	for _, tt := range tests {
		got := Detect(Options{Override: []string{"APP_LANG", "LOCALIZER_LANG"}, Getenv: env(tt.env), OSLanguages: osLangs})
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: Detect = %+v, want %+v", tt.name, got, tt.want)
		}
	}
	if got := Detect(Options{Getenv: env(nil), OSLanguages: func() []string { return nil }}); len(got.Tags) != 0 {
		t.Errorf("no preference should yield no tags, got %+v", got)
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"ja_JP.UTF-8":      "ja-JP",
		"de_US.UTF-8":      "de-US",
		"sr_RS@latin":      "sr-Latn-RS",
		"sr@cyrillic":      "sr-Cyrl",
		"zh_TW.Big5":       "zh-TW",
		"en":               "en",
		"ca_ES.UTF-8@euro": "ca-ES",
	} {
		if got, isC := Normalize(in); got != want || isC {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, isC, want)
		}
	}
	for _, c := range []string{"C", "POSIX", "C.UTF-8"} {
		if _, isC := Normalize(c); !isC {
			t.Errorf("Normalize(%q) should be C", c)
		}
	}
}

func TestMatch(t *testing.T) {
	avail := []string{"ja", "zh-Hans", "ko", "es", "fr", "de", "pt-BR"}
	tests := []struct {
		prefs []string
		want  string
	}{
		{[]string{"ja-JP"}, "ja"},
		{[]string{"en-US", "ja-JP"}, ""},   // English preferred first
		{[]string{"it-IT"}, ""},            // no Italian catalog
		{[]string{"it-IT", "fr-CA"}, "fr"}, // second preference
		{[]string{"es-MX"}, "es"},          // regional variant
		{[]string{"zh-CN"}, "zh-Hans"},     // Simplified
		{[]string{"zh-TW"}, ""},            // Traditional readers don't get Simplified
		{[]string{"zh-Hans-CN", "en"}, "zh-Hans"},
		{[]string{"pt-BR"}, "pt-BR"},
		{[]string{"not a tag"}, ""},
		{nil, ""},
	}
	for _, tt := range tests {
		if got := Match(tt.prefs, avail); got != tt.want {
			t.Errorf("Match(%v) = %q, want %q", tt.prefs, got, tt.want)
		}
	}
}

func TestAppleLanguages(t *testing.T) {
	data, err := os.ReadFile("testdata/GlobalPreferences.plist")
	if err != nil {
		t.Fatal(err)
	}
	got, err := appleLanguages(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ja-JP", "zh-Hans-CN", "en-US", "Français-ünicode"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("appleLanguages = %q, want %q", got, want)
	}
	// Corrupt inputs must fail cleanly, never panic.
	for i := 0; i < len(data); i++ {
		for _, b := range []byte{0x00, 0xff, 0x0f, 0xdf} {
			c := append([]byte(nil), data...)
			c[i] = b
			_, _ = appleLanguages(c)
		}
	}
	for _, c := range [][]byte{nil, []byte("bplist00"), data[:40], []byte("<?xml version=\"1.0\"?>")} {
		if _, err := appleLanguages(c); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func FuzzAppleLanguages(f *testing.F) {
	if data, err := os.ReadFile("testdata/GlobalPreferences.plist"); err == nil {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = appleLanguages(data) })
}
