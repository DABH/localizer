// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

package localizer

import (
	"io/fs"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/DABH/localizer/engine"
	"github.com/DABH/localizer/msgfmt"
)

// Localize translates a Cobra command tree in place: every command's Short, Long, Example and
// deprecation text, every flag description, command group titles, the usage/help/version templates,
// Cobra's built-in help and completion commands, and the errors Cobra prints. Call it right before
// root.Execute(), after the application has added all commands and flags.
//
// The first call detects the user's language (see Init); later calls — for example on a tree rebuilt by an
// interactive shell — reuse that decision. When output stays in English, Localize changes nothing.
//
// Localize never panics: an internal failure leaves the tree, or the part of it not yet translated, in
// English.
func Localize(root *cobra.Command, catalogs fs.FS, opts ...Option) {
	if root == nil {
		return
	}
	cfg := newConfig(opts)
	if !inited.Load() {
		Init(catalogs, opts...)
	}
	st := current.Load()
	if path := cfg.getenv("LOCALIZER_DUMP"); path != "" {
		safely(cfg, "dump", func() {
			if err := dumpTree(root, path, st); err != nil {
				debugf(cfg, "localizer: dump failed: %v\n", err)
			}
		})
	}
	if st == nil {
		return
	}
	safely(cfg, "localize", func() { localizeTree(root, cfg) })
}

// safely runs fn and turns a panic into a debug message: whatever goes wrong inside Localizer, the CLI
// keeps running (in English where the failure left it).
func safely(cfg *config, what string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			debugf(cfg, "localizer: %s failed: %v\n", what, r)
		}
	}()
	fn()
}

func walk(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, sub := range c.Commands() {
		walk(sub, fn)
	}
}

// treeLocalizer translates commands on demand. Translating a whole tree up front costs milliseconds on a
// large CLI (Confluent has ~1,000 commands), yet a single invocation shows at most one command's help, so
// help and usage are translated right before they render. It reads the active language at each call, so
// a later Init that turned localization off stops it from translating what hasn't rendered yet.
type treeLocalizer struct {
	cfg   *config
	mu    sync.Mutex
	done  map[*cobra.Command]bool
	short map[*cobra.Command]bool
	flags map[*pflag.Flag]bool
}

func (l *treeLocalizer) engine() *engine.Engine {
	if st := current.Load(); st != nil {
		return st.eng
	}
	return nil
}

func (l *treeLocalizer) help(s string) string {
	if s == "" {
		return s
	}
	eng := l.engine()
	if eng == nil {
		return s
	}
	return eng.Translate(s, engine.Help)
}

func (l *treeLocalizer) flagSet(fs *pflag.FlagSet) {
	fs.VisitAll(func(f *pflag.Flag) {
		if l.flags[f] {
			return
		}
		l.flags[f] = true
		f.Usage = l.help(f.Usage)
		f.Deprecated = l.help(f.Deprecated)
		f.ShorthandDeprecated = l.help(f.ShorthandDeprecated)
	})
}

func (l *treeLocalizer) commandShort(c *cobra.Command) {
	if !l.short[c] {
		l.short[c] = true
		c.Short = l.help(c.Short)
	}
}

// command translates everything help or usage for c can show: its texts, flags (including inherited ones),
// group titles, and its subcommands' one-line descriptions. It never panics.
func (l *treeLocalizer) command(c *cobra.Command) {
	l.mu.Lock()
	defer l.mu.Unlock()
	safely(l.cfg, "translating "+c.CommandPath(), func() {
		if !l.done[c] {
			l.done[c] = true
			l.commandShort(c)
			c.Long = l.help(c.Long)
			c.Example = l.help(c.Example)
			c.Deprecated = l.help(c.Deprecated)
			for _, g := range c.Groups() {
				g.Title = l.help(g.Title)
			}
		}
		l.flagSet(c.Flags())
		l.flagSet(c.PersistentFlags())
		for p := c.Parent(); p != nil; p = p.Parent() {
			l.flagSet(p.PersistentFlags())
		}
		for _, sub := range c.Commands() {
			l.commandShort(sub)
		}
	})
}

// deprecations translates the notices Cobra and pflag print while parsing, before any help renders.
func (l *treeLocalizer) deprecations(root *cobra.Command) {
	walk(root, func(c *cobra.Command) {
		if c.Deprecated != "" {
			c.Deprecated = l.help(c.Deprecated)
		}
		visit := func(f *pflag.Flag) {
			if f.Deprecated == "" && f.ShorthandDeprecated == "" {
				return
			}
			l.flags[f] = true
			f.Usage = l.help(f.Usage)
			f.Deprecated = l.help(f.Deprecated)
			f.ShorthandDeprecated = l.help(f.ShorthandDeprecated)
		}
		c.Flags().VisitAll(visit)
		c.PersistentFlags().VisitAll(visit)
	})
}

func localizeTree(root *cobra.Command, cfg *config) {
	l := &treeLocalizer{cfg: cfg, done: map[*cobra.Command]bool{}, short: map[*cobra.Command]bool{}, flags: map[*pflag.Flag]bool{}}

	// Cobra adds its help and completion commands lazily; create them now so their descriptions are
	// translated like any other command. Both calls are idempotent.
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()

	if completionRequest(cfg) {
		// Shell completion lists descriptions of many commands and flags in one go: translate eagerly.
		walk(root, func(c *cobra.Command) {
			if !c.DisableFlagParsing {
				// Commands that parse their own flags must not gain --help and --version in completions.
				c.InitDefaultHelpFlag()
				c.InitDefaultVersionFlag()
			}
			l.command(c)
		})
	} else {
		hookHelp(root, l)
		l.deprecations(root)
	}

	localizeTemplates(root, l)
	installTemplateFunc()

	if cfg.writers {
		prefix := root.ErrPrefix()
		if tr, ok := l.lookup(prefix); ok {
			root.SetErrPrefix(tr)
			prefix = tr
		}
		root.SetErr(&writer{w: root.ErrOrStderr(), mode: engine.Error, prefix: prefix})
	}
}

func (l *treeLocalizer) lookup(s string) (string, bool) {
	eng := l.engine()
	if eng == nil {
		return s, false
	}
	return eng.Lookup(s)
}

// hookHelp wraps the help and usage functions so that a command is translated right before it renders:
// the root's, which every command inherits, and those of commands that set their own (Cobra prefers a
// command's own function to its parent's).
func hookHelp(root *cobra.Command, l *treeLocalizer) {
	type funcs struct {
		help  func(*cobra.Command, []string)
		usage func(*cobra.Command) error
	}
	own := map[*cobra.Command]funcs{}
	walk(root, func(c *cobra.Command) {
		if c == root || !c.HasParent() {
			return
		}
		f := funcs{}
		if h := c.HelpFunc(); funcPtr(h) != funcPtr(c.Parent().HelpFunc()) {
			f.help = h
		}
		if u := c.UsageFunc(); funcPtr(u) != funcPtr(c.Parent().UsageFunc()) {
			f.usage = u
		}
		if f.help != nil || f.usage != nil {
			own[c] = f
		}
	})
	wrap := func(c *cobra.Command, help func(*cobra.Command, []string), usage func(*cobra.Command) error) {
		if help != nil {
			c.SetHelpFunc(func(c *cobra.Command, args []string) {
				l.command(c)
				help(c, args)
			})
		}
		if usage != nil {
			c.SetUsageFunc(func(c *cobra.Command) error {
				l.command(c)
				return usage(c)
			})
		}
	}
	wrap(root, root.HelpFunc(), root.UsageFunc())
	for c, f := range own {
		wrap(c, f.help, f.usage)
	}
}

func funcPtr(f any) uintptr { return reflect.ValueOf(f).Pointer() }

// completionRequest reports whether this process is answering a shell-completion request.
func completionRequest(cfg *config) bool {
	for _, a := range cfg.args {
		if a == cobra.ShellCompRequestCmd || a == cobra.ShellCompNoDescRequestCmd {
			return true
		}
	}
	return false
}

// localizeTemplates translates the literal text of usage, help and version templates: the root's, and
// any command's own template. Only literal text and printf string literals change; template functions
// and actions are untouched, so applications that render templates with a private FuncMap keep working.
// Commands are visited parents first, so a command whose own template merely repeats its parent's
// original text is recognised as changed against the parent's translated one and translated too.
func localizeTemplates(root *cobra.Command, l *treeLocalizer) {
	tr := func(unit string) (string, bool) { return l.lookup(unit) }
	type tmpl struct {
		get func(*cobra.Command) string
		set func(*cobra.Command, string)
	}
	kinds := []tmpl{
		{(*cobra.Command).UsageTemplate, (*cobra.Command).SetUsageTemplate},
		{(*cobra.Command).HelpTemplate, (*cobra.Command).SetHelpTemplate},
		{(*cobra.Command).VersionTemplate, (*cobra.Command).SetVersionTemplate},
	}
	for _, k := range kinds {
		walk(root, func(c *cobra.Command) {
			orig := k.get(c)
			if c != root && c.HasParent() && k.get(c.Parent()) == orig {
				return // inherited (the parent is already translated)
			}
			t := msgfmt.TranslateTemplate(orig, tr)
			if t == orig {
				return
			}
			if templateParses(orig) && !templateParses(t) {
				// A translation broke the template: Cobra would panic (or drop the section) rendering it.
				debugf(l.cfg, "localizer: a translation breaks the template of %s; keeping the original\n", c.CommandPath())
				return
			}
			k.set(c, t)
		})
	}
}

// stubFuncs names Cobra's template functions so that templates parse; only their existence matters here.
var stubFuncs = template.FuncMap{
	"trim": stubFunc, "trimRightSpace": stubFunc, "trimTrailingWhitespaces": stubFunc,
	"appendIfNotPresent": stubFunc, "rpad": stubFunc, "gt": stubFunc, "eq": stubFunc,
}

func stubFunc(...any) any { return nil }

// templateParses reports whether text is a template that Cobra can execute. Templates that use an
// application's own functions don't parse with the stubs, so a translation of them is never checked
// (and never rejected) on that basis.
func templateParses(text string) bool {
	_, err := template.New("").Funcs(stubFuncs).Parse(text)
	return err == nil
}

var templateFuncOnce sync.Once

// installTemplateFunc rewrites pflag's hard-coded " (default X)" suffix on flag usage lines through the
// template function Cobra applies to them. The function is registered once for the process and consults
// the active language at each render, so it is a no-op when localization is off.
func installTemplateFunc() {
	templateFuncOnce.Do(func() {
		cobra.AddTemplateFunc("trimTrailingWhitespaces", func(s string) string {
			s = strings.TrimRightFunc(s, unicode.IsSpace)
			st := current.Load()
			if st == nil {
				return s
			}
			tr, ok := st.eng.Lookup("(default %s)")
			if !ok {
				return s
			}
			return localizeDefaults(s, tr)
		})
	})
}

// localizeDefaults rewrites pflag's hard-coded " (default X)" suffix on each flag usage line.
func localizeDefaults(s, translated string) string {
	if !strings.Contains(s, " (default ") {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if !strings.HasSuffix(line, ")") {
			continue
		}
		j := strings.LastIndex(line, " (default ")
		if j < 0 {
			continue
		}
		val := line[j+len(" (default ") : len(line)-1]
		if out, ok := msgfmt.Splice(translated, map[int]string{1: val}, nil); ok {
			lines[i] = line[:j] + " " + out
		}
	}
	return strings.Join(lines, "\n")
}
