// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

//go:build !urfave_cli_no_template

package urfave

import (
	"strings"
	"text/template"

	"github.com/urfave/cli/v3"

	"github.com/DABH/localizer/msgfmt"
)

// templates holds urfave/cli's package-level help templates as they were before Localize translated them.
type templates struct {
	root, command, subcommand string
}

func saveTemplates() templates {
	return templates{cli.RootCommandHelpTemplate, cli.CommandHelpTemplate, cli.SubcommandHelpTemplate}
}

func (t templates) restore() {
	cli.RootCommandHelpTemplate, cli.CommandHelpTemplate, cli.SubcommandHelpTemplate = t.root, t.command, t.subcommand
}

// localizeTemplates translates the literal text of urfave/cli's help templates: the section headings and
// the tokens of the usage line. Template actions are untouched, so applications that render help with
// their own functions keep working.
func localizeTemplates() {
	cli.RootCommandHelpTemplate = translateTemplate(cli.RootCommandHelpTemplate)
	cli.CommandHelpTemplate = translateTemplate(cli.CommandHelpTemplate)
	cli.SubcommandHelpTemplate = translateTemplate(cli.SubcommandHelpTemplate)
}

// inlined are the two unexported sub-templates of urfave/cli (v3.14) whose text carries English words: the
// authors heading, whose plural "S" the sub-template adds, and the usage line of a leaf command, with its
// "[options]". Each is replaced by a body that renders the same text, so that its words can be translated
// like the rest of the template. A template without the action is unaffected.
var inlined = []struct{ action, body string }{
	{`AUTHOR{{template "authorsTemplate" .}}`, `{{if eq 1 (len .Authors)}}AUTHOR:{{else}}AUTHORS:{{end}}
   {{range $index, $author := .Authors}}{{if $index}}
   {{end}}{{$author}}{{end}}`},
	{`{{template "usageTemplate" .}}`, `{{if .UsageText}}{{wrap .UsageText 3}}{{else}}{{.FullName}}{{if .VisibleFlags}} [options]{{end}}{{if .VisibleCommands}} [command [command options]]{{end}}{{if .ArgsUsage}} {{.ArgsUsage}}{{else}}{{if .Arguments}} {{template "argsTemplate" .}}{{end}}{{end}}{{end}}`},
}

// translateTemplate returns tpl with its translatable literals translated (see msgfmt.TemplateUnits), or
// tpl itself when nothing in it has a translation, or when a translation would break the template:
// urfave/cli panics on a help template that does not parse.
func translateTemplate(tpl string) string {
	if tpl == "" {
		return tpl
	}
	expanded := tpl
	for _, in := range inlined {
		expanded = strings.ReplaceAll(expanded, in.action, in.body)
	}
	out := msgfmt.TranslateTemplate(expanded, func(unit string) (string, bool) {
		t := lookup(unit)
		return t, t != unit
	})
	if out == expanded {
		return tpl
	}
	if templateParses(tpl) && !templateParses(out) {
		debugf("localizer: urfave: a translation breaks a help template; keeping the original\n")
		return tpl
	}
	return out
}

// stubFuncs names the functions urfave/cli's help templates use, so that the templates parse; only their
// existence matters here.
var stubFuncs = template.FuncMap{
	"join": stubFunc, "subtract": stubFunc, "indent": stubFunc, "nindent": stubFunc,
	"trim": stubFunc, "wrap": stubFunc, "offset": stubFunc, "offsetCommands": stubFunc,
}

func stubFunc(...any) any { return nil }

// templateParses reports whether text is a template urfave/cli can execute. A template that uses an
// application's own functions does not parse with the stubs, so a translation of it is never checked (and
// never rejected) on that basis.
func templateParses(text string) bool {
	_, err := template.New("").Funcs(stubFuncs).Parse(text)
	return err == nil
}
