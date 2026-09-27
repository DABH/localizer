// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Package locale works out which language the user wants: explicit overrides, the POSIX locale
// environment, then the operating system's preferred languages.
package locale

import (
	"os"
	"strings"

	"golang.org/x/text/language"
)

// Options configures Detect. Zero values use the process environment and the running OS.
type Options struct {
	// Override lists environment variables checked first, in order (for example an app-specific
	// CONFLUENT_LANG followed by LOCALIZER_LANG).
	Override []string
	// Getenv looks up environment variables (os.Getenv when nil).
	Getenv func(string) string
	// OSLanguages returns the OS-wide preferred languages (the platform implementation when nil).
	OSLanguages func() []string
}

// Result describes the user's language preference.
type Result struct {
	Tags   []string // BCP 47 tags, most preferred first; empty means "use the source language"
	Source string   // the setting the preference came from (an env var name or "os")
	Off    bool     // localization explicitly disabled
	Pseudo bool     // pseudo-localization requested (LOCALIZER_LANG=qps)
}

// Detect determines the user's preferred languages.
//
// Order: override variables; then POSIX LC_ALL, LC_MESSAGES and LANG (first non-empty), with GNU
// LANGUAGE honored unless that locale is C/POSIX; then the OS preference (macOS AppleLanguages, Windows
// display languages). A C/POSIX locale means English, so scripts that set LC_ALL=C get stable output.
func Detect(o Options) Result {
	getenv := o.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	for _, name := range o.Override {
		if v := strings.TrimSpace(getenv(name)); v != "" {
			return parseOverride(v, name)
		}
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			continue
		}
		tag, isC := Normalize(v)
		if isC {
			return Result{Tags: []string{"en"}, Source: name}
		}
		tags := splitList(getenv("LANGUAGE"))
		if tag != "" {
			tags = append(tags, tag)
		}
		return Result{Tags: tags, Source: name}
	}
	if tags := splitList(getenv("LANGUAGE")); len(tags) > 0 {
		return Result{Tags: tags, Source: "LANGUAGE"}
	}
	osLangs := o.OSLanguages
	if osLangs == nil {
		osLangs = platformLanguages
	}
	if tags := osLangs(); len(tags) > 0 {
		return Result{Tags: tags, Source: "os"}
	}
	return Result{}
}

func parseOverride(v, source string) Result {
	switch strings.ToLower(v) {
	case "off", "none", "0", "false", "no", "disable", "disabled":
		return Result{Off: true, Source: source}
	case "qps", "qps-ploc", "pseudo":
		return Result{Pseudo: true, Source: source}
	}
	return Result{Tags: splitList(v), Source: source}
}

func splitList(v string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(v, func(r rune) bool { return r == ':' || r == ',' }) {
		if tag, isC := Normalize(f); tag != "" && !isC {
			out = append(out, tag)
		} else if isC {
			out = append(out, "en")
		}
	}
	return out
}

// Normalize converts a POSIX locale name such as "pt_BR.UTF-8" or "sr_RS@latin" to a BCP 47 tag
// ("pt-BR", "sr-Latn-RS"). isC reports the C/POSIX locale (including C.UTF-8).
func Normalize(posix string) (tag string, isC bool) {
	s := strings.TrimSpace(posix)
	modifier := ""
	if i := strings.IndexByte(s, '@'); i >= 0 {
		s, modifier = s[:i], strings.ToLower(s[i+1:])
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	switch s {
	case "":
		return "", false
	case "C", "POSIX":
		return "", true
	}
	s = strings.ReplaceAll(s, "_", "-")
	switch modifier {
	case "latin":
		s = insertScript(s, "Latn")
	case "cyrillic":
		s = insertScript(s, "Cyrl")
	}
	return s, false
}

func insertScript(tag, script string) string {
	lang, rest, found := strings.Cut(tag, "-")
	if !found {
		return lang + "-" + script
	}
	return lang + "-" + script + "-" + rest
}

// Match picks the best catalog language in available for the user's preferred tags. It returns "" when
// the source language (English) is at least as preferred as any available catalog, or when no catalog
// matches with high confidence (a Traditional Chinese reader is not shown Simplified Chinese, for example).
func Match(prefs, available []string) string {
	if len(prefs) == 0 || len(available) == 0 {
		return ""
	}
	supported := []language.Tag{language.English}
	names := []string{""}
	for _, a := range available {
		t, err := language.Parse(a)
		if err != nil {
			continue
		}
		supported = append(supported, t)
		names = append(names, a)
	}
	var want []language.Tag
	for _, p := range prefs {
		if t, err := language.Parse(p); err == nil {
			want = append(want, t)
		}
	}
	if len(want) == 0 || len(supported) == 1 {
		return ""
	}
	_, idx, conf := language.NewMatcher(supported).Match(want...)
	if idx <= 0 || conf < language.High {
		return ""
	}
	return names[idx]
}
