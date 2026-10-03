// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Package urfave localizes a urfave/cli (v3) application with Localizer. The whole integration is one line
// before the root command runs:
//
//	urfave.Localize(cmd, locales.FS)
//	if err := cmd.Run(context.Background(), os.Args); err != nil { ... }
//
// where locales.FS is an embed.FS holding "<language>.json" catalogs. Localize calls localizer.Init, so the
// helpers of package localizer (T, Sprintf, Errorf, Error, Translate, Writer) translate the application's
// own messages afterwards.
//
// Localize changes package-level variables of urfave/cli for the rest of the process: the help templates
// (RootCommandHelpTemplate, CommandHelpTemplate, SubcommandHelpTemplate), the usage of HelpFlag and
// VersionFlag, UsageCommandHelp, ArgsUsageCommandHelp and SuggestDidYouMeanTemplate are translated in
// place, and VersionPrinter, FlagStringer and ErrWriter are wrapped. Call it once, from main, after the
// command tree is complete and before cmd.Run.
package urfave

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/urfave/cli/v3"

	"github.com/DABH/localizer"
	"github.com/DABH/localizer/msgfmt"
)

// Localize renders cmd's help and errors in the user's language. Call it on the root command right before
// cmd.Run.
//
// Localize detects the language and loads the matching catalog (localizer.Init with opts); when output
// stays in English it changes nothing. Otherwise it translates the command tree in place — every command's
// Usage, UsageText, Description, ArgsUsage, Category and deprecation notice, every flag's usage, category,
// default text and deprecation notice, the usage text of arguments, custom help templates — and
// urfave/cli's own strings: the help templates, the help and version flags, the help and completion
// commands, the version line, the "(default: X)" suffix of flag usage lines, and the usage errors and
// deprecation notices written to the root command's ErrWriter and to cli.ErrWriter. Help is translated at
// the source, so the command's Writer is left alone.
//
// Localize never panics: an internal failure leaves the affected part of the output in English.
func Localize(cmd *cli.Command, catalogs fs.FS, opts ...localizer.Option) {
	if cmd == nil {
		return
	}
	if localizer.Init(catalogs, opts...) == "" {
		return
	}
	safely("localize", func() { localize(cmd) })
}

func localize(root *cli.Command) {
	t := &tree{seen: map[*cli.Command]bool{}, ptrs: map[uintptr]bool{}}
	t.command(root)
	localizePackage(t)
	root.ErrWriter = errWriter(root.ErrWriter)
	if root.EnableShellCompletion {
		hookCompletion(root, t)
	}
}

// safely runs fn and turns a panic into a LOCALIZER_DEBUG message: whatever goes wrong inside Localizer,
// the CLI keeps running, in English where the failure left it.
func safely(what string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			debugf("localizer: urfave: %s failed: %v\n", what, r)
		}
	}()
	fn()
}

func debugOn() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOCALIZER_DEBUG"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func debugf(format string, args ...any) {
	if debugOn() {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

// help translates a help string; "" stays "".
func help(s string) string {
	if s == "" {
		return s
	}
	return localizer.Translate(s, localizer.ModeHelp)
}

// lookup translates a string on its own (a template heading, a format string): the catalog entry for s
// itself, or s unchanged.
func lookup(s string) string {
	return localizer.T(s)
}

// errWriter wraps w, or urfave/cli's default of os.Stderr, so that the messages written to it are
// translated in error mode.
func errWriter(w io.Writer) io.Writer {
	if w == nil {
		w = os.Stderr
	}
	return localizer.WriterMode(w, localizer.ModeError)
}

// tree translates a command tree in place. Flags and arguments are reached through their pointers, so one
// shared between commands (or listed both in Flags and a MutuallyExclusiveFlags group) is translated once.
type tree struct {
	seen map[*cli.Command]bool
	ptrs map[uintptr]bool
}

// flagFields are the string fields of urfave/cli's flag types (FlagBase and BoolWithInverseFlag) that help
// or a warning can show. argumentFields are those of its argument types.
var (
	flagFields     = []string{"Usage", "Category", "DefaultText", "Deprecated"}
	argumentFields = []string{"UsageText"}
)

// command translates c and, recursively, its subcommands. A failure on one command leaves that command
// alone and moves on.
func (t *tree) command(c *cli.Command) {
	if c == nil || t.seen[c] {
		return
	}
	t.seen[c] = true
	safely("translating "+c.Name, func() {
		c.Usage = help(c.Usage)
		c.UsageText = help(c.UsageText)
		c.Description = help(c.Description)
		c.ArgsUsage = help(c.ArgsUsage)
		c.Category = help(c.Category)
		c.Deprecated = help(c.Deprecated)
		c.CustomRootCommandHelpTemplate = translateTemplate(c.CustomRootCommandHelpTemplate)
		c.CustomHelpTemplate = translateTemplate(c.CustomHelpTemplate)
		for _, f := range c.Flags {
			t.flag(f)
		}
		for i := range c.MutuallyExclusiveFlags {
			// urfave/cli copies a group's Category onto its flags when the command runs, so the group's
			// text is what has to change.
			grp := &c.MutuallyExclusiveFlags[i]
			grp.Category = help(grp.Category)
			for _, flags := range grp.Flags {
				for _, f := range flags {
					t.flag(f)
				}
			}
		}
		for _, a := range c.Arguments {
			t.argument(a)
		}
	})
	for _, sub := range c.Commands {
		t.command(sub)
	}
}

func (t *tree) flag(f cli.Flag) {
	safely("translating a flag", func() { t.textFields(f, flagFields) })
}

func (t *tree) argument(a cli.Argument) {
	safely("translating an argument", func() { t.textFields(a, argumentFields) })
}

// textFields translates the named string fields of the struct p points to. urfave/cli's flags are generic
// FlagBase structs behind the Flag interface, so the fields are reached by name: a value that is not a
// pointer to a struct, or a struct without the field (an unknown flag type), is skipped.
func (t *tree) textFields(p any, names []string) {
	v := reflect.ValueOf(p)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return
	}
	if t.ptrs[v.Pointer()] {
		return
	}
	t.ptrs[v.Pointer()] = true
	s := v.Elem()
	for _, name := range names {
		if f := s.FieldByName(name); f.IsValid() && f.Kind() == reflect.String && f.CanSet() {
			f.SetString(help(f.String()))
		}
	}
}

// hookCompletion arranges for the completion command, which urfave/cli builds when the root command runs
// (after Localize), to be translated as it is created. The hook is only set when shell completion is
// enabled, in which case urfave/cli adds the command anyway, so setting it changes nothing else.
func hookCompletion(root *cli.Command, t *tree) {
	configure := root.ConfigureShellCompletionCommand
	root.ConfigureShellCompletionCommand = func(c *cli.Command) {
		if configure != nil {
			configure(c)
		}
		t.command(c)
	}
}

// Package-level state of urfave/cli that Localize changes, kept so that restore can put it back.
var (
	pkgMu sync.Mutex
	saved *pkgState // the values before the first Localize; nil until then and again after restore
)

type pkgState struct {
	templates      templates
	flags          []flagState
	usageHelp      string
	argsUsageHelp  string
	didYouMean     string
	versionPrinter func(*cli.Command)
	flagStringer   cli.FlagStringFunc
	errWriter      io.Writer
}

// flagState is the text of one of urfave/cli's package-level flags before translation.
type flagState struct {
	flag   cli.Flag
	fields map[string]string
}

func saveFlag(f cli.Flag) flagState {
	st := flagState{flag: f, fields: map[string]string{}}
	v := reflect.ValueOf(f)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return st
	}
	for _, name := range flagFields {
		if fv := v.Elem().FieldByName(name); fv.IsValid() && fv.Kind() == reflect.String {
			st.fields[name] = fv.String()
		}
	}
	return st
}

func (st flagState) restore() {
	v := reflect.ValueOf(st.flag)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return
	}
	for name, text := range st.fields {
		if fv := v.Elem().FieldByName(name); fv.IsValid() && fv.Kind() == reflect.String && fv.CanSet() {
			fv.SetString(text)
		}
	}
}

// localizePackage translates urfave/cli's own strings: the help templates, the help and version flags
// (copied onto every command when it runs), the help command's texts, the "Did you mean" template, and it
// installs the version, flag-usage and error-writer hooks. The hooks are installed once per process and
// consult the active language at each call, so a later Init that switches the language, or turns
// localization off, is followed.
func localizePackage(t *tree) {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	first := saved == nil
	if first {
		saved = &pkgState{
			templates:      saveTemplates(),
			flags:          []flagState{saveFlag(cli.HelpFlag), saveFlag(cli.VersionFlag)},
			usageHelp:      cli.UsageCommandHelp,
			argsUsageHelp:  cli.ArgsUsageCommandHelp,
			didYouMean:     cli.SuggestDidYouMeanTemplate,
			versionPrinter: cli.VersionPrinter,
			flagStringer:   cli.FlagStringer,
			errWriter:      cli.ErrWriter,
		}
	}
	safely("templates", localizeTemplates)
	t.flag(cli.HelpFlag)
	t.flag(cli.VersionFlag)
	cli.UsageCommandHelp = help(cli.UsageCommandHelp)
	cli.ArgsUsageCommandHelp = help(cli.ArgsUsageCommandHelp)
	cli.SuggestDidYouMeanTemplate = lookup(cli.SuggestDidYouMeanTemplate)
	if first {
		installVersionPrinter()
		installFlagStringer()
		cli.ErrWriter = errWriter(cli.ErrWriter)
	}
}

// restore undoes what Localize did to urfave/cli's package-level variables: the help templates, HelpFlag,
// VersionFlag, UsageCommandHelp, ArgsUsageCommandHelp, SuggestDidYouMeanTemplate, VersionPrinter,
// FlagStringer and ErrWriter. It exists for tests, next to localizer.Reset. Command trees that were
// localized keep their translated strings.
func restore() {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	if saved == nil {
		return
	}
	saved.templates.restore()
	for _, f := range saved.flags {
		f.restore()
	}
	cli.UsageCommandHelp = saved.usageHelp
	cli.ArgsUsageCommandHelp = saved.argsUsageHelp
	cli.SuggestDidYouMeanTemplate = saved.didYouMean
	cli.VersionPrinter = saved.versionPrinter
	cli.FlagStringer = saved.flagStringer
	cli.ErrWriter = saved.errWriter
	saved = nil
}

func funcPtr(f any) uintptr { return reflect.ValueOf(f).Pointer() }

// installVersionPrinter translates the "<name> version <version>" line. An application that prints its
// own version line keeps it.
func installVersionPrinter() {
	if funcPtr(cli.VersionPrinter) != funcPtr(cli.DefaultPrintVersion) {
		return
	}
	cli.VersionPrinter = func(cmd *cli.Command) {
		w := cmd.Root().Writer
		if w == nil {
			w = os.Stdout
		}
		fmt.Fprintf(w, lookup("%v version %v\n"), cmd.Name, cmd.Version)
	}
}

// installFlagStringer rewrites the " (default: X)" suffix that urfave/cli's FlagStringer appends to a flag's
// usage line, through the catalog's "(default: %s)" entry.
func installFlagStringer() {
	stringer := cli.FlagStringer
	if stringer == nil {
		return
	}
	cli.FlagStringer = func(f cli.Flag) string {
		s := stringer(f)
		out := s
		safely("flag usage", func() { out = localizeDefault(s) })
		return out
	}
}

const defaultMark = " (default: "

// localizeDefault rewrites the " (default: X)" suffix of one flag usage line. The suffix may be followed by
// the environment or file hint ("[$VAR]"); anything else after it means the text is not urfave/cli's own
// and is left alone.
func localizeDefault(s string) string {
	j := strings.LastIndex(s, defaultMark)
	if j < 0 {
		return s
	}
	tr := lookup("(default: %s)")
	if tr == "(default: %s)" {
		return s
	}
	rest := s[j+len(defaultMark):]
	k := strings.LastIndex(rest, ")")
	if k < 0 {
		return s
	}
	value, tail := rest[:k], rest[k+1:]
	if tail != "" && !(strings.HasPrefix(tail, " [") && strings.HasSuffix(tail, "]")) {
		return s
	}
	out, ok := msgfmt.Splice(tr, map[int]string{1: value}, nil)
	if !ok {
		return s
	}
	return s[:j] + " " + out + tail
}
