// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package catalog

import (
	"reflect"
	"testing"
)

func TestParseFastMatchesParse(t *testing.T) {
	f := &File{Version: 1, Language: "ja", Messages: map[string]string{
		"Plain.":                        "プレーン。",
		"Line one\nLine two":            "一行目\n二行目",
		"Quote \"%s\" and \\ backslash": "引用 \"%s\" と \\ バックスラッシュ",
		"Tab\there":                     "タブ\tここ",
		"<html> & ampersand":            "<html> & アンパサンド",
		"emoji 🚀 and \u2028 separator":  "絵文字 🚀 と \u2028 区切り",
		"control \u0001 char":           "制御 \u0001 文字",
		"`code` and %[2]s then %[1]d":   "`code` と %[2]s と %[1]d",
	}}
	data, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	fast, err := ParseFast(data)
	if err != nil {
		t.Fatal(err)
	}
	slow, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fast, slow) {
		t.Errorf("ParseFast differs:\n%#v\n%#v", fast, slow)
	}
	if _, ok := scan(data); !ok {
		t.Error("a catalog written by Marshal must take the fast path")
	}
}

const fffd = "\xef\xbf\xbd" // U+FFFD, which encoding/json substitutes for an unpaired surrogate escape

// parseCases are inputs on which a hand-written parser can disagree with encoding/json. fast says whether
// the fast path must handle the input itself rather than defer to Parse.
var parseCases = []struct {
	name, in string
	fast     bool
}{
	{"escapes and odd spacing", `{ "messages" : { "caf\u00e9" : "\u30ab\u30d5\u30a7", "pair" : "\ud83d\ude80" } , "language":"ja","version":1 }`, true},
	{"high surrogate then a character", `{"messages":{"k":"\ud800A"}}`, true},
	{"high surrogate then a non-surrogate escape", `{"messages":{"k":"\ud800\u0041"}}`, true},
	{"high surrogate twice then low", `{"messages":{"k":"\ud800\ud800\udc00"}}`, true},
	{"high surrogate then high", `{"messages":{"k":"\ud800\udbff"}}`, true},
	{"high surrogate at the end", `{"messages":{"k":"\ud800"}}`, true},
	{"lone low surrogates", `{"messages":{"k":"\udc00\udc00"}}`, true},
	{"high surrogate then bad hex", `{"messages":{"k":"\ud800\uzzzz"}}`, false},
	{"uppercase hex", `{"messages":{"k":"\uD83D\uDE80"}}`, true},
	{"escaped key", `{"mess\u0061ges":{"a":"b"}}`, true},
	{"trailing whitespace", "{\"messages\":{}}\n \t\r", true},
	{"trailing garbage", `{"messages":{}} x`, false},
	{"second document", `{"messages":{}}{}`, false},
	{"trailing NUL", "{\"messages\":{}}\x00", false},
	{"trailing vertical tab", "{\"messages\":{}}\v", false},
	{"byte order mark", "\xef\xbb\xbf{\"messages\":{}}", false},
	{"repeated messages key", `{"messages":{"a":"1","b":"2"},"messages":{"b":"3","c":"4"}}`, true},
	{"repeated messages key, empty object", `{"messages":{"a":"1"},"messages":{}}`, true},
	{"repeated scalar keys", `{"version":1,"version":0,"language":"x","language":"y","format":"python","format":""}`, true},
	{"repeated key inside messages", `{"messages":{"a":"1","a":"2"}}`, true},
	{"null then object", `{"messages":null,"messages":{"a":"1"}}`, false},
	{"object then null", `{"messages":{"a":"1"},"messages":null}`, false},
	{"trailing comma inside messages", `{"messages":{"a":"b",}}`, false},
	{"trailing comma at the top level", `{"messages":{"a":"b"},}`, false},
	{"leading comma", `{,"messages":{}}`, false},
	{"leading zero", `{"version":01}`, false},
	{"zero", `{"version":0,"messages":{}}`, true},
	{"negative version", `{"version":-1}`, false},
	{"fraction", `{"version":1.0}`, false},
	{"exponent", `{"version":1e0}`, false},
	{"huge version", `{"version":99999999999999999999}`, false},
	{"future version", `{"version":99,"messages":{}}`, true},
	{"empty object", `{}`, true},
	{"unknown key", `{"messages":{},"extra":1}`, false},
	{"differently cased key", `{"Messages":{"a":"b"}}`, false},
	{"control character", "{\"language\":\"\x01\"}", false},
	{"DEL", "{\"language\":\"\x7f\"}", true},
	{"invalid escape", `{"language":"\x41"}`, false},
	{"short escape", `{"language":"\u004"}`, false},
	{"invalid UTF-8", "{\"messages\":{\"a\":\"\xff\"}}", false},
	{"non-string value", `{"messages":{"a":1}}`, false},
	{"null value", `{"messages":{"a":null}}`, false},
	{"garbage", `not json`, false},
	{"empty", ``, false},
	{"null document", `null`, false},
	{"array", `[]`, false},
}

func TestParseFastCases(t *testing.T) {
	for _, tt := range parseCases {
		fast, ferr := ParseFast([]byte(tt.in))
		slow, serr := Parse([]byte(tt.in))
		if (ferr == nil) != (serr == nil) {
			t.Errorf("%s: ParseFast err=%v, Parse err=%v", tt.name, ferr, serr)
			continue
		}
		if ferr == nil && !reflect.DeepEqual(fast, slow) {
			t.Errorf("%s: ParseFast = %#v\n\tParse = %#v", tt.name, fast, slow)
		}
		if _, ok := scan([]byte(tt.in)); ok != tt.fast {
			t.Errorf("%s: fast path handled it = %v, want %v", tt.name, ok, tt.fast)
		}
	}
	// The values behind the cases above, spelled out.
	for in, want := range map[string]map[string]string{
		`{"messages":{"k":"\ud800A"}}`:                                {"k": fffd + "A"},
		`{"messages":{"k":"\ud800\u0041"}}`:                           {"k": fffd + "A"},
		`{"messages":{"k":"\ud800\ud800\udc00"}}`:                     {"k": fffd + "\xf0\x90\x80\x80"},
		`{"messages":{"k":"\ud800\udbff"}}`:                           {"k": fffd + fffd},
		`{"messages":{"k":"\udc00\udc00"}}`:                           {"k": fffd + fffd},
		`{"messages":{"k":"\uD83D\uDE80"}}`:                           {"k": "🚀"},
		`{"messages":{"a":"1","b":"2"},"messages":{"b":"3","c":"4"}}`: {"a": "1", "b": "3", "c": "4"},
		`{"messages":{"a":"1"},"messages":{}}`:                        {"a": "1"},
		`{"messages":{"a":"1","a":"2"}}`:                              {"a": "2"},
	} {
		f, err := ParseFast([]byte(in))
		if err != nil || !reflect.DeepEqual(f.Messages, want) {
			t.Errorf("ParseFast(%s) = %#v, %v; want %#v", in, f, err, want)
		}
	}
	for _, in := range []string{`{"messages":{}} x`, "\xef\xbb\xbf{}", `{"version": 99, "messages": {}}`, `{"messages":{"a":"b",}}`, `{"version":01}`} {
		if _, err := ParseFast([]byte(in)); err == nil {
			t.Errorf("ParseFast(%q) succeeded, want an error", in)
		}
	}
	f, err := ParseFast([]byte(`{"version":1,"version":0,"language":"x","language":"y","format":"python","format":""}`))
	if err != nil || f.Version != 0 || f.Language != "y" || f.Format != "" {
		t.Errorf("repeated scalar keys: %#v, %v; the last value must win", f, err)
	}
}

func FuzzParseFast(f *testing.F) {
	f.Add([]byte(`{"version":1,"language":"ja","messages":{"a":"b"}}`))
	f.Add([]byte(`{"messages":{"a\n":"b\u00e9"}}`))
	for _, c := range parseCases {
		f.Add([]byte(c.in))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		fast, ferr := ParseFast(data)
		slow, serr := Parse(data)
		if (ferr == nil) != (serr == nil) {
			t.Fatalf("ParseFast err=%v, Parse err=%v for %q", ferr, serr, data)
		}
		if ferr == nil && !reflect.DeepEqual(fast, slow) {
			t.Fatalf("mismatch for %q:\n%#v\n%#v", data, fast, slow)
		}
	})
}
