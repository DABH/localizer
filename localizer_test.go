// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package localizer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var testCatalogs = os.DirFS("testdata/locales")

// reset clears package state between tests.
func reset() {
	current.Store(nil)
	inited.Store(false)
}

func env(m map[string]string) Option {
	return withEnv(func(k string) string { return m[k] }, func() []string { return nil })
}

func newTestCLI() *cobra.Command {
	root := &cobra.Command{Use: "demo", Short: "Demo CLI.", Long: "Demo is a tool for testing Localizer."}
	root.PersistentFlags().Bool("verbose", false, "Enable verbose output.")
	topic := &cobra.Command{Use: "topic", Short: "Manage topics.", GroupID: "admin"}
	create := &cobra.Command{
		Use:     "create <name>",
		Short:   "Create a topic.",
		Args:    cobra.ExactArgs(1),
		Example: "Create a topic named orders.\n\n  $ demo topic create orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "dup" {
				return fmt.Errorf("failed to create topic %q: %w", args[0], errors.New("topic already exists"))
			}
			fmt.Fprintf(cmd.OutOrStdout(), T("Created topic %q.\n"), args[0])
			return nil
		},
	}
	create.Flags().Int("partitions", 6, "Number of partitions.")
	create.Flags().String("cleanup-policy", "delete", "Cleanup `policy` to use.")
	create.Flags().String("name", "", "Topic name to use.")
	topic.AddCommand(create)
	root.AddGroup(&cobra.Group{ID: "admin", Title: "Admin Commands:"})
	root.AddCommand(topic)
	root.SilenceUsage = false
	return root
}

// buffers holds the writers a test CLI was configured with before Localize (which wraps the error
// writer, so tests must not replace it afterwards).
var buffers = map[*cobra.Command][2]*bytes.Buffer{}

func withBuffers(root *cobra.Command) *cobra.Command {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(errBuf)
	buffers[root] = [2]*bytes.Buffer{out, errBuf}
	return root
}

func run(root *cobra.Command, args ...string) (stdout, stderr string, err error) {
	b, ok := buffers[root]
	if !ok {
		withBuffers(root)
		b = buffers[root]
	}
	b[0].Reset()
	b[1].Reset()
	root.SetArgs(args)
	err = root.Execute()
	return b[0].String(), b[1].String(), err
}

func localized(t *testing.T, vars map[string]string) *cobra.Command {
	t.Helper()
	reset()
	root := withBuffers(newTestCLI())
	Localize(root, testCatalogs, env(vars))
	return root
}

func child(c *cobra.Command, name string) *cobra.Command {
	for _, sub := range c.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}

func TestEnglishIsUntouched(t *testing.T) {
	reset()
	plain := newTestCLI()
	wantOut, _, _ := run(plain, "topic", "create", "--help")

	root := localized(t, map[string]string{"LANG": "en_US.UTF-8"})
	gotOut, _, _ := run(root, "topic", "create", "--help")
	if gotOut != wantOut {
		t.Errorf("English help changed:\n%s\nwant:\n%s", gotOut, wantOut)
	}
	if Lang() != "" {
		t.Errorf("Lang() = %q, want \"\"", Lang())
	}
}

func TestHelpIsTranslated(t *testing.T) {
	root := localized(t, map[string]string{"LANG": "ja_JP.UTF-8"})
	if Lang() != "ja" {
		t.Fatalf("Lang() = %q", Lang())
	}
	out, _, err := run(root, "topic", "create", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"トピックを作成します。",                                                  // Short (help template falls back to Short)
		"使い方:\n  demo topic create <name> [flags]",                     // built-in heading
		"例:\norders という名前のトピックを作成します。\n\n  $ demo topic create orders", // example text only
		"フラグ:",
		"--cleanup-policy policy   使用するクリーンアップ policy。 (デフォルト: \"delete\")", // backquoted placeholder + default suffix
		"パーティション数。 (デフォルト: 6)",
		"create のヘルプ", // cobra's help flag via built-in pattern
		"グローバルフラグ:",
		"詳細な出力を有効にします。",
		"Topic name to use.", // invalid translation ("BROKEN %d") falls back to English
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q:\n%s", want, out)
		}
	}

	out, _, _ = run(root, "--help")
	for _, want := range []string{"Demo は Localizer をテストするためのツールです。", "管理コマンド:", "トピックを管理します。", "その他のコマンド:", "任意のコマンドのヘルプを表示します", "指定したシェル用の自動補完スクリプトを生成します", "コマンドの詳細については \"demo [command] --help\" を実行してください。"} {
		if !strings.Contains(out, want) {
			t.Errorf("root help missing %q:\n%s", want, out)
		}
	}
}

func TestRuntimeOutputAndErrors(t *testing.T) {
	root := localized(t, map[string]string{"LC_ALL": "ja_JP.UTF-8"})
	out, _, err := run(root, "topic", "create", "orders")
	if err != nil || out != "トピック \"orders\" を作成しました。\n" {
		t.Errorf("RunE output = %q, %v", out, err)
	}

	_, errOut, _ := run(root, "topic", "create")
	if !strings.Contains(errOut, "エラー: 引数は 1 個必要ですが、0 個指定されました") {
		t.Errorf("arg error not translated: %q", errOut)
	}

	_, errOut, _ = run(root, "tpic")
	for _, want := range []string{"エラー: \"demo\" のコマンド \"tpic\" は不明です", "もしかして:\n\ttopic", "使い方は 'demo --help' を実行して確認してください。"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("unknown-command error missing %q: %q", want, errOut)
		}
	}

	_, errOut, _ = run(root, "topic", "create", "x", "--bogus")
	if !strings.Contains(errOut, "エラー: 不明なフラグです: --bogus") {
		t.Errorf("flag error not translated: %q", errOut)
	}

	_, errOut, _ = run(root, "topic", "create", "dup")
	if !strings.Contains(errOut, `エラー: トピック "dup" の作成に失敗しました: トピックはすでに存在します`) {
		t.Errorf("RunE error not translated: %q", errOut)
	}
}

func TestHelpers(t *testing.T) {
	reset()
	if T("Manage topics.") != "Manage topics." || Error(errors.New("x")) != "x" || Error(nil) != "" {
		t.Error("helpers must be identity before Init")
	}
	if lang := Init(testCatalogs, env(map[string]string{"LOCALIZER_LANG": "ja"})); lang != "ja" {
		t.Fatalf("Init = %q", lang)
	}
	if got := T("Manage topics."); got != "トピックを管理します。" {
		t.Errorf("T = %q", got)
	}
	if got := Sprintf("Created topic %q.\n", "a"); got != "トピック \"a\" を作成しました。\n" {
		t.Errorf("Sprintf = %q", got)
	}
	wrapped := fmt.Errorf("failed to create topic %q: %w", "b", errors.New("topic already exists"))
	if got := Error(wrapped); got != `トピック "b" の作成に失敗しました: トピックはすでに存在します` {
		t.Errorf("Error = %q", got)
	}
	e := Errorf("failed to create topic %q: %w", "c", os.ErrExist)
	if !errors.Is(e, os.ErrExist) || !strings.HasPrefix(e.Error(), "トピック") {
		t.Errorf("Errorf lost wrapping or translation: %v", e)
	}
	var buf bytes.Buffer
	w := Writer(&buf)
	fmt.Fprint(w, "Manage topics.\n")
	fmt.Fprint(w, "server said: Manage topics.\n") // output mode: no label split, stays as is
	if buf.String() != "トピックを管理します。\nserver said: Manage topics.\n" {
		t.Errorf("Writer output = %q", buf.String())
	}
	if f, ok := Writer(os.Stdout).(interface{ Fd() uintptr }); !ok || f.Fd() != os.Stdout.Fd() {
		t.Error("Writer must expose the underlying Fd")
	}
}

func TestLanguageSelection(t *testing.T) {
	cases := []struct {
		env  map[string]string
		opts []Option
		want string
	}{
		{map[string]string{"LANG": "de_DE.UTF-8"}, nil, "de"},
		{map[string]string{"LANG": "fr_FR.UTF-8"}, nil, ""},
		{map[string]string{"LANG": "ja_JP.UTF-8", "LOCALIZER_LANG": "off"}, nil, ""},
		{map[string]string{"LANG": "ja_JP.UTF-8", "APP_LANG": "de"}, []Option{WithEnvVar("APP_LANG")}, "de"},
		{map[string]string{"LANG": "ja_JP.UTF-8"}, []Option{WithLanguage("en")}, ""},
		{map[string]string{"LANG": "C"}, nil, ""},
		{map[string]string{"LOCALIZER_LANG": "qps"}, nil, "qps"},
	}
	for _, c := range cases {
		reset()
		got := Init(testCatalogs, append([]Option{env(c.env)}, c.opts...)...)
		if got != c.want {
			t.Errorf("env %v: Init = %q, want %q", c.env, got, c.want)
		}
	}
}

func TestPseudoLocalization(t *testing.T) {
	root := localized(t, map[string]string{"LOCALIZER_LANG": "qps"})
	out, _, _ := run(root, "topic", "--help")
	if !strings.Contains(out, "⟦Ṁáñáĝé ţöþîçš.⟧") || !strings.Contains(out, "⟦Ûšáĝé:⟧") {
		t.Errorf("pseudo help output:\n%s", out)
	}
}

func TestLocalizeTwiceAndDump(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.json")
	vars := map[string]string{"LANG": "ja_JP.UTF-8", "LOCALIZER_DUMP": path}
	root := localized(t, vars)
	// A second tree (as an interactive shell would rebuild) reuses the language decision.
	second := withBuffers(newTestCLI())
	Localize(second, testCatalogs, env(vars))
	if out, _, _ := run(second, "--help"); !strings.Contains(out, "トピックを管理します。") {
		t.Errorf("second tree not localized:\n%s", out)
	}
	if child(second, "topic").Long != "" || child(child(second, "topic"), "create").Short != "Create a topic." {
		t.Error("commands that were never displayed should not have been translated (lazy translation)")
	}
	_ = root
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var d Dump
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatal(err)
	}
	var sawFlag, sawMiss bool
	for _, e := range d.Entries {
		if e.Kind == "flag" && e.Flag == "partitions" && e.Translated != nil && *e.Translated {
			sawFlag = true
		}
		if e.Text == "Topic name to use." && e.Translated != nil && !*e.Translated {
			sawMiss = true
		}
	}
	if d.Language != "ja" || !sawFlag || !sawMiss {
		t.Errorf("unexpected dump: lang=%q flag=%v miss=%v", d.Language, sawFlag, sawMiss)
	}
}

// BenchmarkLocalize measures the startup cost for a Confluent-sized tree (~1,000 commands, ~5,000 help
// strings) with a matching 10k-entry catalog.
func BenchmarkLocalize(b *testing.B) {
	msgs := map[string]string{}
	for i := 0; i < 10000; i++ {
		msgs[fmt.Sprintf("Help text number %d for a command.", i)] = fmt.Sprintf("コマンドのヘルプ %d。", i)
	}
	data, _ := json.Marshal(map[string]any{"version": 1, "language": "ja", "messages": msgs})
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ja.json"), data, 0o644); err != nil {
		b.Fatal(err)
	}
	fsys := os.DirFS(dir)
	build := func() *cobra.Command {
		root := &cobra.Command{Use: "big"}
		n := 0
		for i := 0; i < 40; i++ {
			grp := &cobra.Command{Use: fmt.Sprintf("g%d", i), Short: fmt.Sprintf("Help text number %d for a command.", n)}
			n++
			for j := 0; j < 25; j++ {
				c := &cobra.Command{Use: fmt.Sprintf("c%d", j), Short: fmt.Sprintf("Help text number %d for a command.", n), Run: func(*cobra.Command, []string) {}}
				n++
				for k := 0; k < 4; k++ {
					c.Flags().String(fmt.Sprintf("f%d", k), "", fmt.Sprintf("Help text number %d for a command.", (n*7+k)%10000))
				}
				grp.AddCommand(c)
			}
			root.AddCommand(grp)
		}
		return root
	}
	vars := map[string]string{"LANG": "ja_JP.UTF-8"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		root := build()
		reset()
		runtime.GC()
		b.StartTimer()
		Localize(root, fsys, env(vars))
	}
}
