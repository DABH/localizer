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
	// Hand-written JSON with escaped unicode and odd spacing.
	odd := []byte(`{ "messages" : { "caf\u00e9" : "\u30ab\u30d5\u30a7", "pair" : "\ud83d\ude80" } , "language":"ja","version":1 }`)
	fast, err = ParseFast(odd)
	if err != nil || fast.Messages["café"] != "カフェ" || fast.Messages["pair"] != "🚀" || fast.Language != "ja" {
		t.Errorf("odd input: %#v %v", fast, err)
	}
	if _, err := ParseFast([]byte(`{"version": 99, "messages": {}}`)); err == nil {
		t.Error("future version must be rejected")
	}
	if _, err := ParseFast([]byte(`not json`)); err == nil {
		t.Error("garbage must be rejected")
	}
}

func FuzzParseFast(f *testing.F) {
	f.Add([]byte(`{"version":1,"language":"ja","messages":{"a":"b"}}`))
	f.Add([]byte(`{"messages":{"a\n":"b\u00e9"}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		fast, ferr := ParseFast(data)
		slow, serr := Parse(data)
		if ferr == nil && serr == nil && !reflect.DeepEqual(fast.Messages, slow.Messages) {
			t.Fatalf("mismatch for %q:\n%#v\n%#v", data, fast.Messages, slow.Messages)
		}
	})
}
