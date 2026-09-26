// Package localizer renders a CLI's own strings (help, flag descriptions, messages, errors) in the user's
// language, using translation catalogs that are committed to the repository and embedded in the binary.
//
// Typical integration is one line before executing a Cobra root command:
//
//	localizer.Localize(rootCmd, locales.FS)
//
// where locales.FS is an embed.FS holding "<language>.json" catalogs (see the catalog format in the docs).
// Localization never makes network calls; strings without a translation, and all dynamic data such as
// server responses, are printed unchanged.
//
// Environment controls:
//
//	LOCALIZER_LANG=ja      force a language (a list like "ja:en" is allowed)
//	LOCALIZER_LANG=en|off  disable localization
//	LOCALIZER_LANG=qps     pseudo-localize every known string (QA: shows what flows through Localizer)
//	LOCALIZER_DEBUG=1      report untranslated help strings on stderr
//	LOCALIZER_DUMP=<file>  write every help string in the command tree, with hit/miss, as JSON
package localizer

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/DABH/localizer/catalog"
	"github.com/DABH/localizer/engine"
	"github.com/DABH/localizer/internal/builtin"
	"github.com/DABH/localizer/internal/locale"
	"github.com/DABH/localizer/msgfmt"
)

// EnvLang is the environment variable that overrides language detection for every Localizer-enabled CLI.
const EnvLang = "LOCALIZER_LANG"

type state struct {
	eng  *engine.Engine
	lang string
}

var (
	initMu  sync.Mutex
	inited  atomic.Bool
	current atomic.Pointer[state]
)

type config struct {
	envVars []string
	lang    string
	getenv  func(string) string
	osLangs func() []string
	stderr  io.Writer
	writers bool
	args    []string
}

// Option configures Localize and Init.
type Option func(*config)

// WithEnvVar adds an application-specific override variable (for example "CONFLUENT_LANG") that is
// consulted before LOCALIZER_LANG and the system locale.
func WithEnvVar(name string) Option {
	return func(c *config) { c.envVars = append(c.envVars, name) }
}

// WithLanguage forces a language (for example from a --lang flag). "en" or "off" disables localization.
func WithLanguage(tag string) Option {
	return func(c *config) { c.lang = tag }
}

// WithoutErrWriter keeps Localize from wrapping the root command's error writer. Cobra's own error
// messages are then printed in English.
func WithoutErrWriter() Option {
	return func(c *config) { c.writers = false }
}

// withEnv replaces environment and OS lookups; used by tests.
func withEnv(getenv func(string) string, osLangs func() []string) Option {
	return func(c *config) { c.getenv, c.osLangs = getenv, osLangs }
}

func newConfig(opts []Option) *config {
	c := &config{getenv: os.Getenv, stderr: os.Stderr, writers: true, args: os.Args[1:]}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Init detects the user's language and loads the matching catalog from catalogs, enabling T, Sprintf,
// Errorf, Error and Writer. It returns the selected language, or "" when output stays in English. Most
// Cobra applications call Localize instead, which calls Init. Init can be called again to switch catalogs.
func Init(catalogs fs.FS, opts ...Option) string {
	initMu.Lock()
	defer initMu.Unlock()
	st := setup(catalogs, newConfig(opts))
	current.Store(st)
	inited.Store(true)
	if st == nil {
		return ""
	}
	return st.lang
}

func setup(catalogs fs.FS, cfg *config) *state {
	var res locale.Result
	if cfg.lang != "" {
		res = locale.Detect(locale.Options{Override: []string{"_forced"}, Getenv: func(string) string { return cfg.lang }})
	} else {
		res = locale.Detect(locale.Options{
			Override:    append(append([]string(nil), cfg.envVars...), EnvLang),
			Getenv:      cfg.getenv,
			OSLanguages: cfg.osLangs,
		})
	}
	if res.Off {
		return nil
	}
	available := catalog.Languages(catalogs)
	var eng *engine.Engine
	lang := ""
	if res.Pseudo {
		var all []map[string]string
		for _, l := range available {
			if f, err := catalog.Load(catalogs, l); err == nil {
				all = append(all, f.Messages)
			}
		}
		for _, l := range builtin.Languages() {
			all = append(all, builtin.Messages(l))
		}
		eng, lang = engine.NewPseudo(all...), "qps"
	} else {
		lang = locale.Match(res.Tags, available)
		if lang == "" {
			return nil
		}
		f, err := catalog.Load(catalogs, lang)
		if err != nil {
			debugf(cfg, "localizer: cannot load %s catalog: %v\n", lang, err)
			return nil
		}
		eng = engine.New(lang, builtin.Messages(lang), f.Messages)
	}
	if on(cfg.getenv("LOCALIZER_DEBUG")) {
		w := cfg.stderr
		eng.OnMiss = func(s string, _ engine.Mode) {
			first, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
			fmt.Fprintf(w, "localizer: untranslated (%s): %q\n", lang, first)
		}
	}
	return &state{eng: eng, lang: lang}
}

func on(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func debugf(cfg *config, format string, args ...any) {
	if on(cfg.getenv("LOCALIZER_DEBUG")) {
		fmt.Fprintf(cfg.stderr, format, args...)
	}
}

// Lang returns the active language tag ("qps" for pseudo-localization), or "" when output is English.
func Lang() string {
	if st := current.Load(); st != nil {
		return st.lang
	}
	return ""
}

// T returns the translation of s, or s unchanged. For a format string, call T before formatting:
// fmt.Sprintf(localizer.T(format), args...) — or use Sprintf.
func T(s string) string {
	st := current.Load()
	if st == nil || s == "" {
		return s
	}
	if msgfmt.HasVerbs(s) {
		if t, ok := st.eng.Lookup(s); ok {
			return t
		}
		return s
	}
	return st.eng.Translate(s, engine.Output)
}

// Sprintf formats according to the translation of format.
func Sprintf(format string, args ...any) string {
	return fmt.Sprintf(T(format), args...)
}

// Errorf is fmt.Errorf with a translated format. %w wrapping is preserved. Prefer translating errors
// when they are displayed (Error) if code elsewhere inspects error strings.
func Errorf(format string, args ...any) error {
	return fmt.Errorf(T(format), args...)
}

// Error returns err's message translated for display: CLI-authored parts of a wrapped error chain are
// translated, server-supplied text is left as is. It returns "" for a nil error.
func Error(err error) string {
	if err == nil {
		return ""
	}
	st := current.Load()
	if st == nil {
		return err.Error()
	}
	return st.eng.Translate(err.Error(), engine.Error)
}

// Writer returns a writer that translates CLI strings written to w, one Write call at a time (so
// prompts without a trailing newline are passed through immediately). It returns w itself when output is
// not being localized. The returned writer exposes w's Fd method, if any, for terminal detection.
func Writer(w io.Writer) io.Writer {
	st := current.Load()
	if st == nil {
		return w
	}
	return &writer{w: w, eng: st.eng, mode: engine.Output}
}

type writer struct {
	w      io.Writer
	eng    *engine.Engine
	mode   engine.Mode
	prefix string // a known prefix such as cobra's "Error:", stripped before translating
}

func (w *writer) Write(p []byte) (int, error) {
	s := string(p)
	var out string
	if w.prefix != "" && strings.HasPrefix(s, w.prefix+" ") {
		out = w.prefix + " " + w.eng.Translate(s[len(w.prefix)+1:], w.mode)
	} else {
		out = w.eng.Translate(s, w.mode)
	}
	if _, err := io.WriteString(w.w, out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Fd returns the file descriptor of the underlying writer, so isatty-style checks keep working.
func (w *writer) Fd() uintptr {
	if f, ok := w.w.(interface{ Fd() uintptr }); ok {
		return f.Fd()
	}
	return ^uintptr(0)
}
