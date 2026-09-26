package localizer

import (
	"io/fs"
	"strings"
	"sync"
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
		if err := dumpTree(root, path, st); err != nil {
			debugf(cfg, "localizer: dump failed: %v\n", err)
		}
	}
	if st == nil {
		return
	}
	localizeTree(root, st.eng, cfg)
}

func walk(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, sub := range c.Commands() {
		walk(sub, fn)
	}
}

// treeLocalizer translates commands on demand. Translating a whole tree up front costs milliseconds on a
// large CLI (Confluent has ~1,000 commands), yet a single invocation shows at most one command's help, so
// help and usage are translated right before they render.
type treeLocalizer struct {
	eng   *engine.Engine
	mu    sync.Mutex
	done  map[*cobra.Command]bool
	short map[*cobra.Command]bool
	flags map[*pflag.Flag]bool
}

func (l *treeLocalizer) help(s string) string {
	if s == "" {
		return s
	}
	return l.eng.Translate(s, engine.Help)
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
// group titles, and its subcommands' one-line descriptions.
func (l *treeLocalizer) command(c *cobra.Command) {
	l.mu.Lock()
	defer l.mu.Unlock()
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
}

func localizeTree(root *cobra.Command, eng *engine.Engine, cfg *config) {
	l := &treeLocalizer{eng: eng, done: map[*cobra.Command]bool{}, short: map[*cobra.Command]bool{}, flags: map[*pflag.Flag]bool{}}

	// Cobra adds its help and completion commands lazily; create them now so their descriptions are
	// translated like any other command. Both calls are idempotent.
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()

	if completionRequest(cfg) {
		// Shell completion lists descriptions of many commands and flags in one go: translate eagerly.
		walk(root, func(c *cobra.Command) {
			c.InitDefaultHelpFlag()
			c.InitDefaultVersionFlag()
			l.command(c)
		})
	} else {
		help := root.HelpFunc()
		root.SetHelpFunc(func(c *cobra.Command, args []string) {
			l.command(c)
			help(c, args)
		})
		usage := root.UsageFunc()
		root.SetUsageFunc(func(c *cobra.Command) error {
			l.command(c)
			return usage(c)
		})
		// Deprecation notices print before any help renders.
		walk(root, func(c *cobra.Command) {
			if c.Deprecated != "" {
				c.Deprecated = l.help(c.Deprecated)
			}
		})
	}

	localizeTemplates(root, eng)

	if tr, ok := eng.Lookup("(default %s)"); ok {
		cobra.AddTemplateFunc("trimTrailingWhitespaces", func(s string) string {
			return localizeDefaults(strings.TrimRightFunc(s, unicode.IsSpace), tr)
		})
	}

	prefix := root.ErrPrefix()
	if tr, ok := eng.Lookup(prefix); ok {
		root.SetErrPrefix(tr)
		prefix = tr
	}
	if cfg.writers {
		root.SetErr(&writer{w: root.ErrOrStderr(), eng: eng, mode: engine.Error, prefix: prefix})
	}
}

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
// any command's own template that differs from its parent's. Only literal text and printf string
// literals change; template functions and actions are untouched, so applications that render templates
// with a private FuncMap keep working.
func localizeTemplates(root *cobra.Command, eng *engine.Engine) {
	tr := func(unit string) (string, bool) { return eng.Lookup(unit) }
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
		// Collect first: setting a template on a parent changes what children inherit.
		type change struct {
			c *cobra.Command
			t string
		}
		var changes []change
		walk(root, func(c *cobra.Command) {
			orig := k.get(c)
			if c != root && c.HasParent() && k.get(c.Parent()) == orig {
				return // inherited
			}
			if t := msgfmt.TranslateTemplate(orig, tr); t != orig {
				changes = append(changes, change{c, t})
			}
		})
		for _, ch := range changes {
			k.set(ch.c, ch.t)
		}
	}
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
