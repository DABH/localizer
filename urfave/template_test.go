// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

//go:build !urfave_cli_no_template

package urfave

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/urfave/cli/v3"

	"github.com/DABH/localizer"
)

func TestTemplatesParseAfterTranslation(t *testing.T) {
	setup(t)
	Localize(newTestCLI(), catalogs, localizer.WithLanguage("ja"))
	for name, tpl := range map[string]string{"root": cli.RootCommandHelpTemplate, "command": cli.CommandHelpTemplate, "subcommand": cli.SubcommandHelpTemplate} {
		if !templateParses(tpl) {
			t.Errorf("the %s template no longer parses:\n%s", name, tpl)
		}
		if strings.Contains(tpl, "USAGE:") || !strings.Contains(tpl, "使い方:") {
			t.Errorf("the %s template is not translated:\n%s", name, tpl)
		}
	}
}

func TestBrokenTranslationKeepsTheTemplate(t *testing.T) {
	setup(t)
	broken := fstest.MapFS{"ja.json": &fstest.MapFile{Data: []byte(`{"version":1,"language":"ja","messages":{"NAME:":"名前 {{","Demo CLI.":"デモ CLI。"}}`)}}
	root, out, _ := withBuffers(newTestCLI())
	Localize(root, broken, localizer.WithLanguage("ja"))
	if err := run(root, "--help"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "{{") || !strings.Contains(out.String(), "NAME:") || !strings.Contains(out.String(), "デモ CLI。") {
		t.Errorf("a translation with an unterminated action must leave the template in English:\n%s", out)
	}
}

// The sub-templates the adapter inlines (the authors heading, the usage line of a leaf command) must render
// exactly what urfave/cli's own render. Italian has no built-in catalog, so only "NAME:" changes here.
func TestInlinedTemplatesRenderTheSame(t *testing.T) {
	setup(t)
	it := fstest.MapFS{"it.json": &fstest.MapFile{Data: []byte(`{"version":1,"language":"it","messages":{"NAME:":"NOME:"}}`)}}
	for _, args := range [][]string{{"--help"}, {"topic", "create", "--help"}, {"topic", "--help"}} {
		plain, out, _ := withBuffers(newTestCLI())
		_ = run(plain, args...)
		want := out.String()

		root, out, _ := withBuffers(newTestCLI())
		Localize(root, it, localizer.WithLanguage("it"))
		_ = run(root, args...)
		got := out.String()
		if !strings.Contains(got, "NOME:") {
			t.Errorf("%v: heading not translated:\n%s", args, got)
		}
		if strings.ReplaceAll(got, "NOME:", "NAME:") != want {
			t.Errorf("%v: inlined templates render differently:\n%s\nwant:\n%s", args, got, want)
		}
		restore()
		localizer.Reset()
	}
}
