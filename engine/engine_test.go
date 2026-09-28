// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package engine

import (
	"fmt"
	"strings"
	"testing"
)

var ja = map[string]string{
	"List Kafka clusters.":          "Kafka クラスターを一覧表示します。",
	"Created %s \"%s\".\n":          "%[1]s「\"%[2]s\"」を作成しました。\n",
	"failed to create topic %q: %w": "トピック %[1]q の作成に失敗しました: %[2]w",
	"topic already exists":          "トピックはすでに存在します",
	"Suggestions:":                  "提案:",
	"List available topics with `confluent kafka topic list`.": "`confluent kafka topic list` で利用可能なトピックを一覧表示します。",
	"DEPRECATED:":                 "非推奨:",
	"Manage Kafka topics.":        "Kafka トピックを管理します。",
	"Create a Kafka topic.":       "Kafka トピックを作成します。",
	"Error:":                      "エラー:",
	"unknown command %q for %q%s": "%[2]q のコマンド %[1]q は不明です%[3]s",
	"Did you mean this?":          "もしかして:",
	"bad translation %s":          "壊れた翻訳 %d",
	"with escape":                 "エスケープ\x1b[31m付き",
	"Name":                        "名前",
}

func TestExactAndWhitespace(t *testing.T) {
	e := New("ja", ja)
	if got := e.Translate("  List Kafka clusters.\n", Output); got != "  Kafka クラスターを一覧表示します。\n" {
		t.Errorf("got %q", got)
	}
	if got := e.Translate("not in catalog", Output); got != "not in catalog" {
		t.Errorf("miss changed text: %q", got)
	}
	if got, ok := e.Lookup("Created %s \"%s\".\n"); !ok || got != "%[1]s「\"%[2]s\"」を作成しました。\n" {
		t.Errorf("Lookup = %q, %v", got, ok)
	}
}

func TestPatternsAndWrappedErrors(t *testing.T) {
	e := New("ja", ja)
	got := e.Translate(fmt.Sprintf("Created %s \"%s\".\n", "API key", "ABC"), Output)
	if got != "API key「\"ABC\"」を作成しました。\n" {
		t.Errorf("pattern: %q", got)
	}
	err := fmt.Errorf("failed to create topic %q: %w", "orders", fmt.Errorf("topic already exists"))
	if got := e.Translate(err.Error(), Error); got != `トピック "orders" の作成に失敗しました: トピックはすでに存在します` {
		t.Errorf("wrapped error: %q", got)
	}
	// Server text inside a wrapped error passes through untouched.
	err = fmt.Errorf("failed to create topic %q: %w", "orders", fmt.Errorf("HTTP 500: internal"))
	if got := e.Translate(err.Error(), Error); got != `トピック "orders" の作成に失敗しました: HTTP 500: internal` {
		t.Errorf("server text: %q", got)
	}
}

func TestCobraStyleError(t *testing.T) {
	e := New("ja", ja)
	in := "Error: unknown command \"stat\" for \"app\"\n\nDid you mean this?\n\tstatus\n"
	want := "エラー: \"app\" のコマンド \"stat\" は不明です\n\nもしかして:\n\tstatus\n"
	if got := e.Translate(in, Error); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestSegmentsAndModes(t *testing.T) {
	e := New("ja", ja)
	example := "Create a Kafka topic.\n\n  $ confluent kafka topic create orders"
	if got := e.Translate(example, Help); got != "Kafka トピックを作成します。\n\n  $ confluent kafka topic create orders" {
		t.Errorf("example: %q", got)
	}
	suggestions := "\nSuggestions:\n    List available topics with `confluent kafka topic list`.\n"
	want := "\n提案:\n    `confluent kafka topic list` で利用可能なトピックを一覧表示します。\n"
	if got := e.Translate(suggestions, Output); got != want {
		t.Errorf("suggestions: %q", got)
	}
	if got := e.Translate("DEPRECATED: Manage Kafka topics.", Help); got != "非推奨: Kafka トピックを管理します。" {
		t.Errorf("deprecated prefix: %q", got)
	}
	// Output mode never splits "label: value" — this could be a YAML line.
	if got := e.Translate("Name: Manage Kafka topics.", Output); got != "Name: Manage Kafka topics." {
		t.Errorf("output mode split a label: %q", got)
	}
}

func TestGenericPatternsDontSwallowProse(t *testing.T) {
	e := New("ja", map[string]string{"Manage %s.": "%s を管理します。", "Start %s.": "%s を起動します。"})
	for _, in := range []string{
		"Manage your Confluent Cloud or Confluent Platform. Log in to see all available commands.",
		"Start an interactive shell.",
	} {
		if got := e.Translate(in, Help); got != in {
			t.Errorf("generic pattern mangled %q into %q", in, got)
		}
	}
	if got := e.Translate("Manage Apache Kafka®.", Help); got != "Apache Kafka® を管理します。" {
		t.Errorf("value capture rejected: %q", got)
	}
	if got := e.Translate("Start kafka.", Help); got != "kafka を起動します。" {
		t.Errorf("single-token capture rejected: %q", got)
	}
}

func TestInvalidTranslationsFallBack(t *testing.T) {
	e := New("ja", ja)
	if got := e.Translate(fmt.Sprintf("bad translation %s", "x"), Output); got != "bad translation x" {
		t.Errorf("verb-changing translation used: %q", got)
	}
	if got := e.Translate("with escape", Output); got != "with escape" {
		t.Errorf("escape-injecting translation used: %q", got)
	}
}

func TestOverrideOrder(t *testing.T) {
	builtin := map[string]string{"Error:": "エラー（組み込み）:", "Usage:": "使い方:"}
	app := map[string]string{"Error:": "エラー:"}
	e := New("ja", builtin, app)
	if got := e.Translate("Error:", Help); got != "エラー:" {
		t.Errorf("app catalog should override builtin, got %q", got)
	}
	if got := e.Translate("Usage:", Help); got != "使い方:" {
		t.Errorf("builtin missing: %q", got)
	}
}

func TestPseudo(t *testing.T) {
	e := NewPseudo(ja)
	got := e.Translate(fmt.Sprintf("Created %s \"%s\".\n", "API key", "ABC"), Output)
	if !strings.HasPrefix(got, "⟦Çŕéáţéď API key \"ABC\"") {
		t.Errorf("pseudo pattern: %q", got)
	}
	if got := e.Translate("unknown text", Output); got != "unknown text" {
		t.Errorf("pseudo should leave unknown strings alone: %q", got)
	}
}

func TestOnMiss(t *testing.T) {
	e := New("ja", ja)
	var misses []string
	e.OnMiss = func(s string, _ Mode) { misses = append(misses, s) }
	e.Translate("Totally new help text.", Help)
	e.Translate("Totally new output.", Output)
	e.Translate("List Kafka clusters.", Help)
	if len(misses) != 1 || misses[0] != "Totally new help text." {
		t.Errorf("misses = %q", misses)
	}
}

func BenchmarkTranslate(b *testing.B) {
	cat := map[string]string{}
	for i := 0; i < 10000; i++ {
		cat[fmt.Sprintf("Message number %d about things.", i)] = fmt.Sprintf("メッセージ %d。", i)
		if i%10 == 0 {
			cat[fmt.Sprintf("Created resource kind %d named %%q.", i)] = fmt.Sprintf("種類 %d の %%q を作成しました。", i)
		}
	}
	e := New("ja", cat)
	inputs := []string{"Message number 42 about things.", "Created resource kind 990 named \"x\".", "some server data line that misses"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Defeat the memo to measure raw lookups.
		e.Translate(inputs[i%3]+strings.Repeat(" ", i%64), Output)
	}
}

func TestMemoIsBoundedByBytes(t *testing.T) {
	e := New("ja", ja)
	long := strings.Repeat("x", maxMemoEntry+1)
	e.Translate(long, Output)
	if _, ok := e.memo[Output].Load(long); ok {
		t.Error("a string longer than maxMemoEntry was memoized")
	}
	short := strings.Repeat("y", 100)
	e.Translate(short, Output)
	if _, ok := e.memo[Output].Load(short); !ok {
		t.Error("a short string was not memoized")
	}
	if got := e.memoSize.Load(); got != 200 {
		t.Errorf("memoSize = %d, want 200 (input + output bytes)", got)
	}
	// Once the budget is spent, nothing more is kept, and lookups still work.
	e.memoSize.Store(maxMemoBytes)
	other := "List Kafka clusters."
	if got := e.Translate(other, Output); got != "Kafka クラスターを一覧表示します。" {
		t.Errorf("translation with a full memo: %q", got)
	}
	if _, ok := e.memo[Output].Load(other); ok {
		t.Error("memoized past the byte budget")
	}
}

func TestTrimmedKeysAreDeterministic(t *testing.T) {
	// Two whitespace variants of the same text: the smallest original key wins, whatever the map order.
	for i := 0; i < 20; i++ {
		e := New("ja", map[string]string{" Done": "A", "Done\n": "B", "Fine ": "C", "Fine": "D"})
		if got := e.Translate("Done", Output); got != "A" {
			t.Fatalf("variant choice = %q, want A (the smaller key)", got)
		}
		if got := e.Translate("Fine", Output); got != "D" {
			t.Fatalf("exact key lost to a copy: %q", got)
		}
	}
}

func TestSuggestionLinesStayUntranslated(t *testing.T) {
	e := New("ja", map[string]string{"status": "状態", "Did you mean this?": "もしかして:", "Error:": "エラー:"})
	in := "Error: unknown\n\nDid you mean this?\n\tstatus\n"
	if got := e.Translate(in, Error); got != "エラー: unknown\n\nもしかして:\n\tstatus\n" {
		t.Errorf("got %q", got)
	}
	if got := e.Translate("status", Output); got != "状態" {
		t.Errorf("standalone key: %q", got)
	}
}
