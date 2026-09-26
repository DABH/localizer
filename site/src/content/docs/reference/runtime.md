---
title: Runtime library
description: The Go API, options, environment variables and how the language is chosen.
---

```sh
go get github.com/DABH/localizer
```

The library depends on Cobra (v1.8 or later), pflag, `golang.org/x/text` and `golang.org/x/sys`. It makes
no network calls, and it has no telemetry or credentials. The full API is on
[pkg.go.dev](https://pkg.go.dev/github.com/DABH/localizer).

## Functions

| Function | Description |
| --- | --- |
| `Localize(root *cobra.Command, catalogs fs.FS, opts ...Option)` | Translates a Cobra command tree in place: descriptions, flags, group titles, templates, the built-in `help` and `completion` commands, and Cobra’s errors. Call it right before `root.Execute()`. |
| `Init(catalogs fs.FS, opts ...Option) string` | Detects the language and loads the matching catalog for the helpers below. Returns the language, or `""` for English. `Localize` calls it for you. |
| `T(s string) string` | The translation of `s`, or `s` unchanged. |
| `Sprintf(format string, args ...any) string` | `fmt.Sprintf` with a translated format. |
| `Errorf(format string, args ...any) error` | `fmt.Errorf` with a translated format. `%w` still wraps. |
| `Error(err error) string` | An error message translated for display. CLI-authored parts of a wrapped chain are translated, and server text is left as it is. |
| `Writer(w io.Writer) io.Writer` | A writer that translates CLI strings written to `w`, one `Write` at a time. It exposes `w`’s `Fd` method for terminal detection. |
| `Lang() string` | The active language tag (`"qps"` for pseudo-localization), or `""` for English. |

## Options

| Option | Description |
| --- | --- |
| `WithEnvVar(name)` | An application-specific override variable, such as `YOURCLI_LANG`, checked before `LOCALIZER_LANG`. |
| `WithLanguage(tag)` | Force a language, for example from a `--lang` flag. `"en"` or `"off"` disables localization. |
| `WithoutErrWriter()` | Don’t wrap the root command’s error writer. Cobra’s own errors then stay in English. |

## Environment variables

| Variable | Effect |
| --- | --- |
| `LOCALIZER_LANG=ja` | Force a language. A list such as `ja:en` is allowed. |
| `LOCALIZER_LANG=en` or `off` | Disable localization. |
| `LOCALIZER_LANG=qps` | Pseudo-localization: every known string is shown as `⟦Ŝţŕîñĝ⟧`, so strings that don’t go through Localizer stand out. |
| `LOCALIZER_DEBUG=1` | Report untranslated help strings on stderr. |
| `LOCALIZER_DUMP=<file>` | Write every help string in the command tree as JSON, with hit or miss (see [Testing](../../guides/testing/)). |

## Language selection

Localizer uses the first of these that is set:

1. the language passed with `WithLanguage`;
2. your application’s override variable (`WithEnvVar`), then `LOCALIZER_LANG`;
3. `LC_ALL`, `LC_MESSAGES` or `LANG`, in that order. GNU `LANGUAGE`, if set, lists preferred languages
   ahead of that locale;
4. the OS setting: the preferred languages on macOS, and the display language on Windows;
5. English.

`C` and `POSIX` locales mean English, so scripts that set `LC_ALL=C` get stable output. Locale names are
normalized to BCP 47 (`pt_BR.UTF-8` becomes `pt-BR`, and `zh_TW` becomes `zh-Hant-TW`) and matched to your
catalogs. Only confident matches are used.
