// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package locale

import (
	"encoding/binary"
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

// testdataLanguages is the AppleLanguages array in testdata/GlobalPreferences.plist.
var testdataLanguages = []string{"ja-JP", "zh-Hans-CN", "en-US", "Français-ünicode"}

func TestAppleLanguages(t *testing.T) {
	data, err := os.ReadFile("testdata/GlobalPreferences.plist")
	if err != nil {
		t.Fatal(err)
	}
	got, err := appleLanguages(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, testdataLanguages) {
		t.Errorf("appleLanguages = %q, want %q", got, testdataLanguages)
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
	// A minimal hand-assembled plist decodes, so the hostile ones below fail for the intended reason.
	minimal := craftPlist(1, dictAppleLanguages, asciiAppleLanguages, []byte{0xA1, 0x03}, []byte{0x52, 'j', 'a'})
	if got, err := appleLanguages(minimal); err != nil || !reflect.DeepEqual(got, []string{"ja"}) {
		t.Fatalf("crafted plist: appleLanguages = %q, %v", got, err)
	}
	for _, tt := range hostilePlists {
		got, err := appleLanguages(tt.data)
		if (err != nil) != tt.wantErr || len(got) != 0 {
			t.Errorf("%s: appleLanguages = %q, %v; want error %v", tt.name, got, err, tt.wantErr)
		}
	}
}

// Objects for craftPlist, encoded with one-byte references: a one-entry dictionary whose key is object 1
// and whose value is object 2, and the ASCII string "AppleLanguages".
var (
	dictAppleLanguages  = []byte{0xD1, 0x01, 0x02}
	asciiAppleLanguages = append([]byte{0x5E}, "AppleLanguages"...)
)

// hostilePlists are well-formed up to one object whose count or length, an untrusted 64-bit integer,
// overflows naive arithmetic. The first one used to crash every CLI at startup: 2*n wrapped to 0, refs
// returned an empty slice and dictValue indexed it.
var hostilePlists = []struct {
	name    string
	data    []byte
	wantErr bool
}{
	{"dict count 2^63", craftPlist(1, []byte{0xDF, 0x13, 0x80, 0, 0, 0, 0, 0, 0, 0}), true},
	{"dict count 2^63+1 followed by two refs", craftPlist(1, []byte{0xDF, 0x13, 0x80, 0, 0, 0, 0, 0, 0, 1, 0, 0}), true},
	{"dict count 2^64-1", craftPlist(1, []byte{0xDF, 0x13, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}), true},
	{"dict count 2^60 with 8-byte refs", craftPlist(8, []byte{0xDF, 0x13, 0x10, 0, 0, 0, 0, 0, 0, 0}), true},
	{"array count 2^63", craftPlist(1, dictAppleLanguages, asciiAppleLanguages, []byte{0xAF, 0x13, 0x80, 0, 0, 0, 0, 0, 0, 0}), true},
	{"ascii string length 2^64-1", craftPlist(1, dictAppleLanguages, asciiAppleLanguages, []byte{0xA1, 0x03}, []byte{0x5F, 0x13, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}), false},
	{"utf-16 string length 2^63", craftPlist(1, dictAppleLanguages, asciiAppleLanguages, []byte{0xA1, 0x03}, []byte{0x6F, 0x13, 0x80, 0, 0, 0, 0, 0, 0, 0}), false},
}

// craftPlist assembles a binary plist from hand-encoded objects, object 0 being the top object. Offsets
// are one byte wide (the objects must fit in 247 bytes); references are refSize bytes wide.
func craftPlist(refSize byte, objects ...[]byte) []byte {
	b := []byte("bplist00")
	var table []byte
	for _, o := range objects {
		table = append(table, byte(len(b)))
		b = append(b, o...)
	}
	trailer := make([]byte, 32)
	trailer[6], trailer[7] = 1, refSize
	binary.BigEndian.PutUint64(trailer[8:], uint64(len(objects)))
	binary.BigEndian.PutUint64(trailer[24:], uint64(len(b)))
	return append(append(b, table...), trailer...)
}

func FuzzAppleLanguages(f *testing.F) {
	if data, err := os.ReadFile("testdata/GlobalPreferences.plist"); err == nil {
		f.Add(data)
	}
	for _, tt := range hostilePlists {
		f.Add(tt.data)
	}
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = appleLanguages(data) })
}
