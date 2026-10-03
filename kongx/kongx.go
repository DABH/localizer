// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Package kongx renders a kong (github.com/alecthomas/kong) application's help and errors in the user's
// language, from the same JSON catalogs package localizer uses.
//
// The whole integration is one extra option, placed last:
//
//	ctx := kong.Parse(&cli, kong.Name("yourcli"), kong.Description("..."), kongx.Localize(locales.FS))
//
// Localize translates the help of every command, flag and argument, group titles, kong's own headings
// (Usage, Flags, Commands, Arguments), its "Run ... --help" hints and the parse errors it prints. Output
// the application writes itself goes through localizer.T, localizer.Sprintf and friends.
package kongx

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/DABH/localizer"
)

// Localize returns a kong option that renders the application's help and errors in the user's language.
//
// Applying the option detects the language and loads the matching catalog (see localizer.Init; the
// localizer options select or override the language), translates the model kong builds — command, flag,
// argument and group help, including the synthesized --help flag — and wraps the Kong's Stdout and Stderr
// so that kong's own headings, hints and "app: error: ..." lines are translated as they are written.
// Everything happens after kong has applied the other options, so kong.Writers may come before or after
// it; kong.Description, kong.AutoGroup and other options that set help text once the model is built must
// come before it.
// Translation never panics and never fails: a problem leaves that part of the output in English, and when
// output stays in English (no catalog for the user's language, LOCALIZER_LANG=off, ...) the option changes
// nothing at all.
func Localize(catalogs fs.FS, opts ...localizer.Option) kong.Option {
	return kong.OptionFunc(func(k *kong.Kong) error {
		if localizer.Init(catalogs, opts...) == "" {
			return nil
		}
		return kong.PostBuild(func(k *kong.Kong) error {
			safely("localize", func() { localize(k) })
			return nil
		}).Apply(k)
	})
}

// localize translates the built model and wraps the writers. It runs under safely.
func localize(k *kong.Kong) {
	if localizer.Lang() == "" {
		return // a later Init turned localization off before the model was built
	}
	if k.Stdout != nil {
		k.Stdout = &stdout{wrapped{raw: k.Stdout, out: localizer.WriterMode(k.Stdout, localizer.ModeOutput)}}
	}
	if k.Stderr != nil {
		k.Stderr = &stderr{wrapped{raw: k.Stderr, out: localizer.WriterMode(k.Stderr, localizer.ModeError)}}
	}
	if k.Model != nil {
		(&translator{groups: map[*kong.Group]bool{}}).node(k.Model.Node)
	}
}

// safely runs fn and turns a panic into a debug message: whatever goes wrong inside the adapter, the CLI
// keeps running, in English where the failure left it.
func safely(what string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			debugf("localizer: kong %s failed: %v\n", what, r)
		}
	}()
	fn()
}

func debugf(format string, args ...any) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOCALIZER_DEBUG"))) {
	case "1", "true", "yes", "on":
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

// translator translates a model in place. Help strings are translated as kong read them from the struct
// tags, before kong interpolates ${var} references, so catalog keys are the tag texts.
type translator struct {
	groups map[*kong.Group]bool // every node and flag holds its own copy of a group
}

func (t *translator) node(n *kong.Node) {
	if n == nil {
		return
	}
	safely("translating "+n.FullPath(), func() {
		n.Help = t.help(n.Help)
		n.Detail = t.help(n.Detail)
		t.group(n.Group)
		if n.Argument != nil {
			n.Argument.Help = t.help(n.Argument.Help)
		}
		for _, f := range n.Flags {
			if f == nil {
				continue
			}
			if f.Value != nil {
				f.Help = t.help(f.Help)
			}
			t.group(f.Group)
		}
		for _, p := range n.Positional {
			if p != nil {
				p.Help = t.help(p.Help)
			}
		}
	})
	for _, c := range n.Children {
		t.node(c)
	}
}

func (t *translator) group(g *kong.Group) {
	if g == nil || t.groups[g] {
		return
	}
	t.groups[g] = true
	g.Title = t.help(g.Title)
	g.Description = t.help(g.Description)
}

// help translates one help string. A translation that changes the set of ${var} references is not used:
// kong would fail to build the application over an undefined variable, and a dropped one loses the value
// the author meant to show.
func (t *translator) help(s string) string {
	if s == "" {
		return s
	}
	tr := localizer.Translate(s, localizer.ModeHelp)
	if tr == s {
		return s
	}
	if !sameVars(s, tr) {
		debugf("localizer: the translation of %q changes its ${var} references; keeping the original\n", s)
		return s
	}
	return tr
}

// interpolation is kong's grammar for ${var} and ${var=default} references ("$$" is a literal dollar).
var interpolation = regexp.MustCompile(`(\$\$)|((?:\${([[:alpha:]_][[:word:]]*))(?:=([^}]+))?})|(\$)|([^$]+)`)

func vars(s string) []string {
	var out []string
	for _, m := range interpolation.FindAllStringSubmatch(s, -1) {
		if m[3] != "" {
			out = append(out, m[3])
		}
	}
	sort.Strings(out)
	return out
}

func sameVars(a, b string) bool {
	va, vb := vars(a), vars(b)
	if len(va) != len(vb) {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return false
		}
	}
	return true
}

// wrapped is a Kong writer with its translating counterpart. kong writes help and errors one line at a
// time, so a line that the catalog knows as a whole (a heading, a hint, a message) is translated by the
// localizer writer in the stream's mode. The two lines kong prefixes with a label of its own are handled
// here, because the label would otherwise be swallowed: "Usage: app cmd [flags]" (output mode never splits
// a "label: rest" line, so that data the application writes to the same stream is left alone) and
// "app: error: message" (in error mode, a format string can reverse-match the whole line and capture the
// leader as a value).
type wrapped struct {
	raw io.Writer // the application's writer
	out io.Writer // localizer.WriterMode(raw, mode)
}

// Fd returns the file descriptor of the underlying writer, so isatty-style checks keep working.
func (w *wrapped) Fd() uintptr {
	if f, ok := w.raw.(interface{ Fd() uintptr }); ok {
		return f.Fd()
	}
	return ^uintptr(0)
}

// write writes render()'s result to raw, or p itself when the translation fails.
func (w *wrapped) write(p []byte, render func() string) (n int, err error) {
	out := string(p)
	func() {
		defer func() {
			if recover() != nil {
				out = string(p)
			}
		}()
		out = render()
	}()
	if _, err := io.WriteString(w.raw, out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// labeled joins a translated label and its text the way the engine does: a full-width colon carries its
// own spacing.
func labeled(label, rest string) string {
	t := localizer.T(label)
	if strings.HasSuffix(t, "：") {
		return t + rest
	}
	return t + " " + rest
}

// usagePrefix starts the line kong's help printers write first: "Usage: app cmd <arg> [flags]".
const (
	usageLabel  = "Usage:"
	usagePrefix = usageLabel + " "
)

// stdout is the Stdout wrapper: the usage line keeps its command path behind a translated label, every
// other line is translated in output mode.
type stdout struct{ wrapped }

func (w *stdout) Write(p []byte) (int, error) {
	rest, ok := strings.CutPrefix(string(p), usagePrefix)
	if !ok {
		return w.out.Write(p)
	}
	return w.write(p, func() string {
		// Further lines (a custom help printer writing its whole output at once) are translated like the
		// other lines.
		first, more, multiline := strings.Cut(rest, "\n")
		out := labeled(usageLabel, first)
		if multiline {
			out += "\n" + localizer.Translate(more, localizer.ModeOutput)
		}
		return out
	})
}

// errLeader separates the application name from the message in the lines Kong.Errorf, Fatalf and
// FatalIfErrorf write: "app: error: message". Continuation lines of a multi-line message are indented
// instead and go through the error-mode writer as they are.
const (
	errLabel  = "error:"
	errLeader = ": " + errLabel + " "
)

// stderr is the Stderr wrapper: an "app: error: message" line keeps its application name and gets a
// translated label and message, every other line is translated in error mode.
type stderr struct{ wrapped }

func (w *stderr) Write(p []byte) (int, error) {
	name, rest, ok := strings.Cut(string(p), errLeader)
	if !ok || name == "" || strings.ContainsAny(name, " \t\r\n") {
		return w.out.Write(p)
	}
	return w.write(p, func() string {
		return name + ": " + labeled(errLabel, localizer.Translate(rest, localizer.ModeError))
	})
}
