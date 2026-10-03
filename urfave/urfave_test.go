// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package urfave

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/urfave/cli/v3"

	"github.com/DABH/localizer"
)

const jaCatalog = `{"version": 1, "language": "ja", "messages": {
	"Demo CLI.": "デモ CLI。",
	"Demo is a tool for testing Localizer.": "Demo は Localizer をテストするためのツールです。",
	"Manage topics.": "トピックを管理します。",
	"List topics.": "トピックを一覧表示します。",
	"Create a topic.": "トピックを作成します。",
	"Create a topic with the given name.": "指定した名前のトピックを作成します。",
	"Example:": "例:",
	"Number of partitions.": "パーティション数。",
	"Cleanup ` + "`policy`" + ` to use.": "使用するクリーンアップ ` + "`policy`" + `。",
	"Topic name to use.": "BROKEN %d",
	"Show what would be created.": "作成される内容を表示します。",
	"Storage options": "ストレージオプション",
	"Admin": "管理",
	"Enable verbose output.": "詳細な出力を有効にします。",
	"use --retention instead.": "代わりに --retention を使ってください。",
	"Created topic %q.\n": "トピック %q を作成しました。\n"
}}`

var catalogs = fstest.MapFS{"ja.json": &fstest.MapFile{Data: []byte(jaCatalog)}}

func newTestCLI() *cli.Command {
	create := &cli.Command{
		Name:        "create",
		Usage:       "Create a topic.",
		Description: "Create a topic with the given name.\n\nExample:\n  $ demo topic create orders",
		ArgsUsage:   "<name>",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "partitions", Value: 6, Usage: "Number of partitions.", Category: "Storage options"},
			&cli.StringFlag{Name: "cleanup-policy", Value: "delete", Usage: "Cleanup `policy` to use.", Category: "Storage options", Deprecated: "use --retention instead."},
			&cli.StringFlag{Name: "name", Usage: "Topic name to use.", Required: true},
			&cli.BoolFlag{Name: "dry-run", Usage: "Show what would be created."},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			fmt.Fprintf(c.Root().Writer, localizer.T("Created topic %q.\n"), c.String("name"))
			return nil
		},
	}
	topic := &cli.Command{Name: "topic", Usage: "Manage topics.", Category: "Admin", Commands: []*cli.Command{create}}
	list := &cli.Command{Name: "list", Usage: "List topics.", Action: func(context.Context, *cli.Command) error { return nil }}
	return &cli.Command{
		Name:        "demo",
		Usage:       "Demo CLI.",
		Description: "Demo is a tool for testing Localizer.",
		Version:     "1.2.3",
		Authors:     []any{"Alice", "Bob"},
		Copyright:   "(c) 2026 Example",
		Suggest:     true,
		Flags:       []cli.Flag{&cli.BoolFlag{Name: "verbose", Usage: "Enable verbose output."}},
		Commands:    []*cli.Command{topic, list},
	}
}

// setup keeps the test process alive on exit-coded errors, captures what urfave/cli writes to its
// package-level error writer, and restores everything Localize changed when the test ends. It returns the
// capture buffer.
func setup(t *testing.T) *bytes.Buffer {
	t.Helper()
	exiter, errw := cli.OsExiter, cli.ErrWriter
	pkgErr := &bytes.Buffer{}
	cli.OsExiter = func(int) {}
	cli.ErrWriter = pkgErr
	t.Cleanup(func() {
		restore()
		localizer.Reset()
		cli.OsExiter, cli.ErrWriter = exiter, errw
	})
	return pkgErr
}

func withBuffers(root *cli.Command) (*cli.Command, *bytes.Buffer, *bytes.Buffer) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	root.Writer, root.ErrWriter = out, errBuf
	return root, out, errBuf
}

func run(root *cli.Command, args ...string) error {
	return root.Run(context.Background(), append([]string{"demo"}, args...))
}

// runLocalized builds a fresh test CLI (urfave/cli keeps flag state between runs), localizes it to Japanese
// and runs it with args.
func runLocalized(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root, out, errBuf := withBuffers(newTestCLI())
	Localize(root, catalogs, localizer.WithLanguage("ja"))
	if localizer.Lang() != "ja" {
		t.Fatalf("Lang() = %q", localizer.Lang())
	}
	err = run(root, args...)
	return out.String(), errBuf.String(), err
}

func wantAll(t *testing.T, what, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("%s missing %q:\n%s", what, w, got)
		}
	}
}

func wantNone(t *testing.T, what, got string, english ...string) {
	t.Helper()
	for _, e := range english {
		if strings.Contains(got, e) {
			t.Errorf("%s still contains %q:\n%s", what, e, got)
		}
	}
}

func TestEnglishIsUntouched(t *testing.T) {
	pkgErr := setup(t)
	for _, args := range [][]string{{"--help"}, {"topic", "create", "--help"}, {"topic", "--help"}, {"help", "list"}, {"--nope"}, {"help", "nosuch"}, {"--version"}, {"topic", "create"}} {
		plain, out, errBuf := withBuffers(newTestCLI())
		_ = run(plain, args...)
		wantOut, wantErr, wantPkg := out.String(), errBuf.String(), pkgErr.String()
		pkgErr.Reset()

		root, out, errBuf := withBuffers(newTestCLI())
		Localize(root, catalogs, localizer.WithLanguage("en"))
		_ = run(root, args...)
		if out.String() != wantOut || errBuf.String() != wantErr || pkgErr.String() != wantPkg {
			t.Errorf("%v: English output changed:\n%s%s%s\nwant:\n%s%s%s", args, out, errBuf, pkgErr, wantOut, wantErr, wantPkg)
		}
		pkgErr.Reset()
	}
	if localizer.Lang() != "" || saved != nil {
		t.Error("English must leave the language off and urfave/cli's package variables alone")
	}
}

func TestRootHelp(t *testing.T) {
	setup(t)
	out, _, err := runLocalized(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, "root help", out,
		"名前:\n   demo - デモ CLI。",
		"使い方:\n   demo [グローバルオプション] [コマンド [コマンドオプション]]",
		"バージョン:\n   1.2.3",
		"説明:\n   Demo は Localizer をテストするためのツールです。",
		"作成者:\n   Alice\n   Bob",
		"コマンド:",
		"管理:\n     topic", // command category
		"トピックを管理します。",
		"トピックを一覧表示します。",
		"コマンドの一覧、または特定のコマンドのヘルプを表示します", // the built-in help command
		"グローバルオプション:",
		"--verbose", "詳細な出力を有効にします。",
		"--help, -h", "ヘルプを表示します",
		"--version, -v", "バージョンを表示します",
		"著作権:\n   (c) 2026 Example",
	)
	wantNone(t, "root help", out, "NAME:", "USAGE:", "VERSION:", "DESCRIPTION:", "AUTHOR", "COMMANDS:", "GLOBAL OPTIONS:", "COPYRIGHT:",
		"[global options]", "[command [command options]]", "show help", "print the version", "Shows a list of commands", "Demo CLI.")
}

func TestCommandHelp(t *testing.T) {
	setup(t)
	out, _, err := runLocalized(t, "topic", "create", "--help")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, "command help", out,
		"名前:\n   demo topic create - トピックを作成します。",
		"使い方:\n   demo topic create [オプション] <name>",
		"説明:\n   指定した名前のトピックを作成します。",
		"例:", "$ demo topic create orders", // the shell example stays as it is
		"オプション:",
		"ストレージオプション", // flag category
		"--partitions int", "パーティション数。 (デフォルト: 6)",
		"--cleanup-policy policy", "使用するクリーンアップ policy。 (デフォルト: ", "delete",
		"--name string", "Topic name to use.", // an invalid translation ("BROKEN %d") falls back to English
		"--dry-run", "作成される内容を表示します。",
		"ヘルプを表示します",
		"グローバルオプション:", "詳細な出力を有効にします。",
	)
	wantNone(t, "command help", out, "NAME:", "USAGE:", "DESCRIPTION:", "OPTIONS:", "[options]", "(default:", "show help", "Storage options", "Create a topic.")

	out, _, err = runLocalized(t, "topic", "--help")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, "subcommand help", out,
		"名前:\n   demo topic - トピックを管理します。",
		"使い方:\n   demo topic [コマンド [コマンドオプション]]",
		"カテゴリ:\n   管理",
		"コマンド:", "create", "トピックを作成します。",
		"オプション:", "ヘルプを表示します",
	)
	wantNone(t, "subcommand help", out, "CATEGORY:", "COMMANDS:", "OPTIONS:", "[command [command options]]", "Admin")
}

func TestHelpCommand(t *testing.T) {
	pkgErr := setup(t)
	out, _, err := runLocalized(t, "help", "topic")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, "help topic", out, "名前:\n   demo topic - トピックを管理します。", "カテゴリ:\n   管理", "コマンド:")
	wantNone(t, "help topic", out, "NAME:", "COMMANDS:")

	out, _, _ = runLocalized(t, "help")
	wantAll(t, "help", out, "名前:\n   demo - デモ CLI。", "コマンド:")

	// "help <unknown>" is an exit-coded error, printed through cli.ErrWriter; with Suggest, urfave/cli
	// appends the closest command name.
	_, _, err = runLocalized(t, "help", "topc")
	if err == nil {
		t.Error("help for an unknown command should fail")
	}
	if got := pkgErr.String(); got != "'topc' のヘルプトピックはありません。topic\n" {
		t.Errorf("help topc: %q", got)
	}
	pkgErr.Reset()

	root, _, _ := withBuffers(newTestCLI())
	root.Suggest = false
	Localize(root, catalogs, localizer.WithLanguage("ja"))
	_ = run(root, "help", "nosuch")
	if got := pkgErr.String(); got != "'nosuch' のヘルプトピックはありません\n" {
		t.Errorf("help nosuch: %q", got)
	}
}

func TestUsageErrors(t *testing.T) {
	setup(t)
	out, errOut, err := runLocalized(t, "--nope")
	if err == nil {
		t.Error("an unknown flag should fail")
	}
	if !strings.HasPrefix(errOut, "使い方が正しくありません: 定義されていないフラグです: -nope\n\n") {
		t.Errorf("unknown flag: %q", errOut)
	}
	if !strings.Contains(errOut, "もしかして: \"--") {
		t.Errorf("suggestion not translated: %q", errOut)
	}
	wantAll(t, "help after a usage error", out, "使い方:", "グローバルオプション:")
	wantNone(t, "unknown flag", errOut, "Incorrect Usage", "flag provided", "Did you mean")

	_, errOut, _ = runLocalized(t, "topic", "create", "--name")
	if !strings.HasPrefix(errOut, "使い方が正しくありません: フラグには引数が必要です: --name\n\n") {
		t.Errorf("missing argument: %q", errOut)
	}

	_, errOut, _ = runLocalized(t, "topic", "create", "--name", "x", "--partitions", "abc")
	if !strings.HasPrefix(errOut, "使い方が正しくありません: フラグ -partitions の値 \"abc\" は無効です: ") {
		t.Errorf("invalid value: %q", errOut)
	}

	_, errOut, _ = runLocalized(t, "topic", "create")
	if !strings.HasPrefix(errOut, "使い方が正しくありません: 必須フラグ \"name\" が設定されていません\n\n") {
		t.Errorf("required flag: %q", errOut)
	}

	out, _, err = runLocalized(t, "--version")
	if err != nil || out != "demo バージョン 1.2.3\n" {
		t.Errorf("version: %q, %v", out, err)
	}
}

func TestRuntimeOutputAndDeprecation(t *testing.T) {
	setup(t)
	out, errOut, err := runLocalized(t, "topic", "create", "--name", "orders", "--cleanup-policy", "compact")
	if err != nil || out != "トピック \"orders\" を作成しました。\n" {
		t.Errorf("action output = %q, %v", out, err)
	}
	// The notice is urfave/cli's format with the flag's (translated) deprecation text spliced in.
	if errOut != "フラグ --cleanup-policy は非推奨です。代わりに --retention を使ってください。\n" {
		t.Errorf("deprecation notice = %q", errOut)
	}
}

func TestCompletionCommand(t *testing.T) {
	setup(t)
	root, out, _ := withBuffers(newTestCLI())
	root.EnableShellCompletion = true
	Localize(root, catalogs, localizer.WithLanguage("ja"))
	if err := run(root, "help", "completion"); err != nil {
		t.Fatal(err)
	}
	wantAll(t, "completion help", out.String(),
		"bash、zsh、fish、Powershell 用のシェル補完スクリプトを出力します。\n   出力を読み込むと補完が有効になります。",
		"source <(demo completion bash)", // shell lines stay
		"スクリプトを path/to/autocomplete/demo.ps1 に出力して実行してください。",
		"bash", "bash 用の補完スクリプトを出力します",
		"zsh 用の補完スクリプトを出力します",
	)
	wantNone(t, "completion help", out.String(), "Output shell completion", "Source the output", "Output bash completion", "and run it.")
}

func TestLocalizeChangesAndRestoreUndoes(t *testing.T) {
	setup(t)
	type snapshot struct {
		root, command, subcommand, helpUsage, versionUsage, usageHelp, argsUsage, didYouMean string
		versionPrinter, flagStringer                                                         uintptr
		errWriter                                                                            any
	}
	take := func() snapshot {
		return snapshot{
			cli.RootCommandHelpTemplate, cli.CommandHelpTemplate, cli.SubcommandHelpTemplate,
			cli.HelpFlag.(*cli.BoolFlag).Usage, cli.VersionFlag.(*cli.BoolFlag).Usage,
			cli.UsageCommandHelp, cli.ArgsUsageCommandHelp, cli.SuggestDidYouMeanTemplate,
			funcPtr(cli.VersionPrinter), funcPtr(cli.FlagStringer), cli.ErrWriter,
		}
	}
	before := take()
	Localize(newTestCLI(), catalogs, localizer.WithLanguage("ja"))
	after := take()
	if after.helpUsage != "ヘルプを表示します" || after.versionUsage != "バージョンを表示します" || after.usageHelp != "コマンドの一覧、または特定のコマンドのヘルプを表示します" ||
		after.argsUsage != "[コマンド]" || after.didYouMean != "もしかして: %q" {
		t.Errorf("package-level texts not translated: %+v", after)
	}
	if after.root == before.root || after.command == before.command || after.subcommand == before.subcommand ||
		after.versionPrinter == before.versionPrinter || after.flagStringer == before.flagStringer || after.errWriter == before.errWriter {
		t.Error("templates and hooks should have changed")
	}
	// A second Localize (another tree) changes nothing further.
	Localize(newTestCLI(), catalogs, localizer.WithLanguage("ja"))
	if again := take(); again != after {
		t.Errorf("second Localize changed the package state:\n%+v\n%+v", again, after)
	}
	restore()
	if got := take(); got != before {
		t.Errorf("restore did not put the package state back:\n%+v\nwant:\n%+v", got, before)
	}
	restore() // idempotent
}

func TestNilRootAndPanicsAreContained(t *testing.T) {
	setup(t)
	Localize(nil, catalogs, localizer.WithLanguage("ja"))
	if localizer.Lang() != "" {
		t.Error("a nil root must not initialize anything")
	}
	// A flag type that is not a struct behind a pointer, and an argument without the known fields, are skipped.
	root := newTestCLI()
	root.Commands[1].Flags = append(root.Commands[1].Flags, oddFlag{}, (*cli.StringFlag)(nil))
	root.Commands[1].Arguments = []cli.Argument{&cli.StringArg{Name: "x", UsageText: "Topic name to use."}}
	Localize(root, catalogs, localizer.WithLanguage("ja"))
	if root.Commands[1].Usage != "トピックを一覧表示します。" {
		t.Errorf("the command holding odd flags was not translated: %q", root.Commands[1].Usage)
	}
}

// oddFlag is a Flag that is a struct value, not a pointer: the adapter has nothing to translate on it.
type oddFlag struct{}

func (oddFlag) String() string           { return "" }
func (oddFlag) Get() any                 { return nil }
func (oddFlag) PreParse() error          { return nil }
func (oddFlag) PostParse() error         { return nil }
func (oddFlag) Set(string, string) error { return nil }
func (oddFlag) Names() []string          { return []string{"odd"} }
func (oddFlag) IsSet() bool              { return false }

func TestPseudoLocalization(t *testing.T) {
	setup(t)
	root, out, _ := withBuffers(newTestCLI())
	Localize(root, catalogs, localizer.WithLanguage("qps"))
	if err := run(root, "topic", "--help"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "⟦Ṁáñáĝé ţöþîçš.⟧") || !strings.Contains(out.String(), "⟦ÛŠÅĜÉ:⟧") {
		t.Errorf("pseudo help output:\n%s", out)
	}
}

func TestLocalizeDefault(t *testing.T) {
	setup(t)
	if got := localizeDefault("--x value\tUsage. (default: 6)"); got != "--x value\tUsage. (default: 6)" {
		t.Errorf("before Init: %q", got)
	}
	localizer.Init(catalogs, localizer.WithLanguage("ja"))
	for in, want := range map[string]string{
		"--x value\tUsage. (default: 6)":                 "--x value\tUsage. (デフォルト: 6)",
		"--x value\tUsage. (default: \"a) b\") [$X, $Y]": "--x value\tUsage. (デフォルト: \"a) b\") [$X, $Y]",
		"--x value\tUsage (default: x) and more text":    "--x value\tUsage (default: x) and more text",
		"--x value\tUsage.":                              "--x value\tUsage.",
	} {
		if got := localizeDefault(in); got != want {
			t.Errorf("localizeDefault(%q) = %q, want %q", in, got, want)
		}
	}
}
