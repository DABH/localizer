// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package kongx_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/alecthomas/kong"

	"github.com/DABH/localizer"
	"github.com/DABH/localizer/kongx"
)

// grammar is a small task CLI: two commands, a grouped one, flags, a positional argument and a help string
// with a ${default} reference, which kong interpolates after the model is translated.
type grammar struct {
	Verbose bool `help:"Print more details."`

	Add struct {
		Title    string `arg:"" help:"Title of the task."`
		Priority string `help:"Priority of the task: low, normal or high." default:"normal" enum:"low,normal,high"`
		Due      string `help:"Due date (default: ${default})." default:"today"`
	} `cmd:"" help:"Add a task."`

	List struct {
		All    bool   `help:"Include completed tasks."`
		Output string `help:"Output format." default:"human" enum:"human,json"`
	} `cmd:"" help:"List tasks."`

	Purge struct{} `cmd:"" help:"Delete completed tasks." group:"admin"`
}

// catalogs holds one Japanese catalog. "Output format." has a translation that adds a ${var} reference,
// which kong would reject as undefined: the adapter must keep the English text instead.
var catalogs = fstest.MapFS{"ja.json": &fstest.MapFile{Data: []byte(`{
  "version": 1,
  "language": "ja",
  "messages": {
    "taskctl keeps a small list of tasks.": "taskctl は小さなタスクリストを管理します。",
    "Print more details.": "詳細を表示します。",
    "Add a task.": "タスクを追加します。",
    "Title of the task.": "タスクのタイトル。",
    "Priority of the task: low, normal or high.": "タスクの優先度: low、normal、high のいずれか。",
    "Due date (default: ${default}).": "期限 (デフォルト: ${default})。",
    "List tasks.": "タスクを一覧表示します。",
    "Include completed tasks.": "完了したタスクも含めます。",
    "Output format.": "BROKEN ${nope}",
    "Delete completed tasks.": "完了したタスクを削除します。",
    "Admin Commands:": "管理コマンド:",
    "Commands for administrators.": "管理者向けのコマンド。",
    "task %d not found": "タスク %d が見つかりません"
  }
}`)}}

// exit is what the Exit override panics with, so that nothing exits and tests see the code.
type exit int

func baseOptions(out, errOut *bytes.Buffer) []kong.Option {
	return []kong.Option{
		kong.Name("taskctl"),
		kong.Description("taskctl keeps a small list of tasks."),
		kong.ExplicitGroups([]kong.Group{{Key: "admin", Title: "Admin Commands:", Description: "Commands for administrators."}}),
		kong.Writers(out, errOut),
		kong.Exit(func(code int) { panic(exit(code)) }),
	}
}

// newCLI builds the test CLI with the usual options followed by extra ones (kongx.Localize goes last, as
// documented).
func newCLI(t *testing.T, extra ...kong.Option) (k *kong.Kong, out, errOut *bytes.Buffer) {
	t.Helper()
	var cli grammar
	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	k, err := kong.New(&cli, append(baseOptions(out, errOut), extra...)...)
	if err != nil {
		t.Fatal(err)
	}
	return k, out, errOut
}

// run parses args the way a main function would: kong prints help or the error itself and "exits" through
// the override, which run recovers.
func run(k *kong.Kong, args ...string) (code int, exited bool) {
	defer func() {
		if r := recover(); r != nil {
			c, ok := r.(exit)
			if !ok {
				panic(r)
			}
			code, exited = int(c), true
		}
	}()
	_, err := k.Parse(args)
	k.FatalIfErrorf(err)
	return 0, false
}

func contains(t *testing.T, what, s string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(s, want) {
			t.Errorf("%s missing %q:\n%s", what, want, s)
		}
	}
}

func lacks(t *testing.T, what, s string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(s, u) {
			t.Errorf("%s still contains %q:\n%s", what, u, s)
		}
	}
}

func TestHelpIsTranslated(t *testing.T) {
	t.Cleanup(localizer.Reset)
	k, out, _ := newCLI(t, kongx.Localize(catalogs, localizer.WithLanguage("ja")))
	if localizer.Lang() != "ja" {
		t.Fatalf("Lang() = %q", localizer.Lang())
	}
	if code, exited := run(k, "--help"); !exited || code != 0 {
		t.Fatalf("--help: exited=%v code=%d", exited, code)
	}
	help := out.String()
	contains(t, "root help", help,
		"使い方: taskctl <command> [flags]\n",
		"taskctl は小さなタスクリストを管理します。",
		"フラグ:\n",
		"コンテキストに応じたヘルプを表示します。", // kong's --help flag, from the built-in catalog
		"詳細を表示します。",
		"コマンド:\n",
		"タスクを追加します。",
		"タスクを一覧表示します。",
		"管理コマンド:\n", // group title and description
		"管理者向けのコマンド。",
		"完了したタスクを削除します。",
		"コマンドの詳細については \"taskctl <command> --help\" を実行してください。",
	)
	lacks(t, "root help", help, "Usage:", "Flags:", "Commands:", "Show context-sensitive help.", "Print more details.",
		"Add a task.", "List tasks.", "Admin Commands:", "Commands for administrators.", "Delete completed tasks.", "Run \"")

	out.Reset()
	run(k, "add", "--help")
	help = out.String()
	contains(t, "add help", help,
		"使い方: taskctl add <title> [flags]\n",
		"引数:\n",
		"タスクのタイトル。",
		"タスクの優先度: low、normal、high のいずれか。",
		"期限 (デフォルト: today)。", // ${default} interpolated after translation
		"詳細を表示します。",          // inherited flag
	)
	lacks(t, "add help", help, "Arguments:", "Title of the task.", "Due date", "${default}")

	out.Reset()
	run(k, "list", "--help")
	help = out.String()
	contains(t, "list help", help, "Output format.", "完了したタスクも含めます。")
	lacks(t, "list help", help, "BROKEN", "${nope}")
}

func TestParseErrorsAreTranslated(t *testing.T) {
	t.Cleanup(localizer.Reset)
	k, out, errOut := newCLI(t, kongx.Localize(catalogs, localizer.WithLanguage("ja")))
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--nope"}, "taskctl: エラー: 不明なフラグです: --nope\n"},
		{[]string{"--verbos"}, "taskctl: エラー: 不明なフラグ --verbos です。もしかして \"--verbose\" ですか?\n"},
		{[]string{}, "taskctl: エラー: \"add\", \"list\", \"purge\" のいずれかを指定してください\n"},
		{[]string{"add"}, "taskctl: エラー: \"<title>\" を指定してください\n"},
		{[]string{"lst"}, "taskctl: エラー: 予期しない引数 lst です。もしかして \"list\" ですか?\n"},
		{[]string{"add", "x", "y"}, "taskctl: エラー: 予期しない引数です: y\n"},
		{[]string{"add", "x", "--priority", "urgent"}, "taskctl: エラー: --priority は \"low\",\"normal\",\"high\" のいずれかである必要がありますが、\"urgent\" が指定されました\n"},
	}
	for _, c := range cases {
		out.Reset()
		errOut.Reset()
		code, exited := run(k, c.args...)
		if !exited || code == 0 {
			t.Errorf("%v: exited=%v code=%d", c.args, exited, code)
		}
		if errOut.String() != c.want {
			t.Errorf("%v: stderr = %q, want %q", c.args, errOut.String(), c.want)
		}
		if out.Len() != 0 {
			t.Errorf("%v: unexpected stdout %q", c.args, out.String())
		}
	}
}

func TestUsageOnErrorIsTranslated(t *testing.T) {
	t.Cleanup(localizer.Reset)
	k, out, errOut := newCLI(t, kong.UsageOnError(), kongx.Localize(catalogs, localizer.WithLanguage("ja")))
	run(k, "add")
	contains(t, "usage on error", out.String(), "使い方: taskctl add <title> [flags]\n", "引数:\n", "タスクのタイトル。")
	lacks(t, "usage on error", out.String(), "Usage:", "Arguments:")
	if got := errOut.String(); got != "taskctl: エラー: \"<title>\" を指定してください\n" {
		t.Errorf("stderr = %q", got)
	}
}

func TestEnglishIsUntouched(t *testing.T) {
	t.Cleanup(localizer.Reset)
	exercise := func(k *kong.Kong) {
		run(k, "--help")
		run(k, "add", "--help")
		run(k, "--nope")
		run(k, "add", "x", "--priority", "urgent")
	}
	plain, out, errOut := newCLI(t)
	exercise(plain)
	wantOut, wantErr := out.String(), errOut.String()
	if !strings.Contains(wantOut, "Usage: taskctl add <title> [flags]\n") || !strings.Contains(wantErr, "taskctl: error: unknown flag --nope\n") {
		t.Fatalf("unexpected plain kong output:\n%s\n%s", wantOut, wantErr)
	}

	for _, opt := range []localizer.Option{localizer.WithLanguage("en"), localizer.WithLanguage("off")} {
		k, out, errOut := newCLI(t, kongx.Localize(catalogs, opt))
		if localizer.Lang() != "" {
			t.Fatalf("Lang() = %q, want \"\"", localizer.Lang())
		}
		if k.Stdout != out || k.Stderr != errOut {
			t.Error("the writers must not be wrapped when output stays in English")
		}
		exercise(k)
		if got := out.String(); got != wantOut {
			t.Errorf("English help changed:\n%s\nwant:\n%s", got, wantOut)
		}
		if got := errOut.String(); got != wantErr {
			t.Errorf("English errors changed:\n%s\nwant:\n%s", got, wantErr)
		}
	}
}

func TestLocalizeBeforeWritersStillWorks(t *testing.T) {
	t.Cleanup(localizer.Reset)
	var cli grammar
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	opts := []kong.Option{
		kong.Name("taskctl"),
		kong.Description("taskctl keeps a small list of tasks."),
		kong.Exit(func(code int) { panic(exit(code)) }),
		kongx.Localize(catalogs, localizer.WithLanguage("ja")),
		kong.Writers(out, errOut), // after Localize: the writers are wrapped once the model is built
	}
	k := kong.Must(&cli, opts...)
	run(k, "--help")
	contains(t, "help", out.String(), "使い方: taskctl <command> [flags]\n", "taskctl は小さなタスクリストを管理します。", "フラグ:\n")
	run(k, "--nope")
	if got := errOut.String(); got != "taskctl: エラー: 不明なフラグです: --nope\n" {
		t.Errorf("stderr = %q", got)
	}
}

func TestWriterExposesFd(t *testing.T) {
	t.Cleanup(localizer.Reset)
	var cli grammar
	k, err := kong.New(&cli, kong.Writers(os.Stdout, os.Stderr), kongx.Localize(catalogs, localizer.WithLanguage("ja")))
	if err != nil {
		t.Fatal(err)
	}
	if f, ok := k.Stdout.(interface{ Fd() uintptr }); !ok || f.Fd() != os.Stdout.Fd() {
		t.Error("Stdout wrapper must expose the terminal's Fd")
	}
	if f, ok := k.Stderr.(interface{ Fd() uintptr }); !ok || f.Fd() != os.Stderr.Fd() {
		t.Error("Stderr wrapper must expose the terminal's Fd")
	}
}

func TestPseudoLocalization(t *testing.T) {
	t.Cleanup(localizer.Reset)
	k, out, _ := newCLI(t, kongx.Localize(catalogs, localizer.WithLanguage("qps")))
	if localizer.Lang() != "qps" {
		t.Fatalf("Lang() = %q", localizer.Lang())
	}
	run(k, "--help")
	contains(t, "pseudo help", out.String(), "⟦Ûšáĝé:⟧ taskctl <command> [flags]\n", "⟦")
	lacks(t, "pseudo help", out.String(), "Usage:", "Flags:", "Add a task.")
}

func TestSecondInitSwitchesWritersOff(t *testing.T) {
	t.Cleanup(localizer.Reset)
	k, out, errOut := newCLI(t, kongx.Localize(catalogs, localizer.WithLanguage("ja")))
	if got := localizer.Init(catalogs, localizer.WithLanguage("en")); got != "" {
		t.Fatalf("Init(en) = %q", got)
	}
	run(k, "--help")
	// The model was translated in place and keeps its strings; kong's own text follows the new choice.
	contains(t, "help after Init(en)", out.String(), "Usage: taskctl <command> [flags]\n", "Flags:\n", "タスクを追加します。")
	run(k, "--nope")
	if got := errOut.String(); got != "taskctl: error: unknown flag --nope\n" {
		t.Errorf("stderr = %q", got)
	}
}

func TestRunErrorsAreTranslatedWhenDisplayed(t *testing.T) {
	t.Cleanup(localizer.Reset)
	k, _, errOut := newCLI(t, kongx.Localize(catalogs, localizer.WithLanguage("ja")))
	func() {
		defer func() {
			if r := recover(); r != nil {
				if c, ok := r.(exit); !ok || c != 1 {
					t.Errorf("exit = %v, want 1", r)
				}
			}
		}()
		k.FatalIfErrorf(localizer.Errorf("task %d not found", 7))
	}()
	if got := errOut.String(); got != "taskctl: エラー: タスク 7 が見つかりません\n" {
		t.Errorf("stderr = %q", got)
	}
}
