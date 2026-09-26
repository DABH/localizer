---
title: Integration guide
description: The one line for Cobra CLIs, output helpers for your own messages, and what gets extracted.
---

## 1. The one line (Cobra)

```go
import (
	"github.com/DABH/localizer"
	"example.com/yourcli/locales"
)

func main() {
	root := newRootCmd()
	localizer.Localize(root, locales.FS) // call last, right before Execute
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
```

`locales/embed.go` comes with the onboarding pull request:

```go
package locales

import "embed"

//go:embed *.json
var FS embed.FS
```

Call `Localize` on the production path only, after every command, flag and help function is set up.
Unit tests, doc generators and linters that build the command tree directly then stay in English. If your
CLI rebuilds its tree at runtime (in an interactive shell, for example), call `Localize` on each new root.
The language decision is reused.

The one line translates:

- command descriptions: `Short`, `Long`, `Example` and `Deprecated`;
- flag descriptions and command group titles;
- the help, usage and version templates;
- Cobra’s `help` and `completion` commands, and shell completion descriptions;
- the errors Cobra prints: unknown commands and flags, argument counts, required flags and flag groups.

Help is translated when it renders, so a normal invocation pays almost nothing for the size of the command
tree.

## 2. Your own messages

Add a few helpers at your output chokepoints:

| Helper | Use |
| --- | --- |
| `localizer.T(s)` | Translate a string. Translate a format string before formatting: `fmt.Sprintf(localizer.T(format), args...)`. |
| `localizer.Sprintf(format, args...)` | Shorthand for the above. |
| `localizer.Error(err)` | Translate an error for display. CLI-authored parts of a wrapped chain are translated, and server text stays as it is. |
| `localizer.Writer(w)` | An `io.Writer` that translates known strings written through it. It works per `Write`, so prompts flush immediately. |
| `localizer.Errorf(format, args...)` | `fmt.Errorf` with a translated format. `%w` still wraps. If other code compares error strings, use `Error` at display time instead. |
| `localizer.Lang()` | The active language, or `""` for English. |

Most CLIs print through a handful of helpers, so hook those instead of every call site. For example,
[a fork of the Confluent CLI](https://github.com/DABH/cli/tree/localizer) needed about fifteen lines:

- its six printing functions;
- three spots in its table renderer (column headers from struct tags, and "None found.");
- its prompt renderer;
- its "Suggestions:" block;
- its "REQUIRED:" flag label.

Never pass serialized output (JSON, YAML) through these helpers.

For a CLI that doesn’t use Cobra, call `localizer.Init(locales.FS)` once at startup and use the helpers.

## 3. What is extracted

The service reads your Go source statically. It never builds or runs it. It collects:

- Cobra command and group fields, and pflag and `flag` definitions;
- `fmt`, `errors` and `log` messages, and Cobra’s `cmd.Print*`;
- `localizer.T`, `localizer.Sprintf` and `localizer.Errorf` calls;
- message-like struct fields (`ErrorMsg`, `Message`, `SuggestionsMsg`, `Prompt`, …) and templates;
- whatever you configure with [`extract.funcs`, `extract.fields` and `extract.struct_tags`](../../reference/configuration/).

Constants are folded across packages, helper functions are followed to the strings they return, and
`fmt.Sprintf` is expanded over every value its arguments can take.

Mark exceptions with a `//localizer:ignore` comment. Add a note for the translator with
`//localizer:context <note>`.

To see which strings don’t go through Localizer yet, run your CLI with pseudo-localization or debug output
(see [Testing](../testing/)).

## 4. Known limitations

- Sentences assembled at runtime from English fragments (`"Deleted " + noun + "."`) are only partly
  translated. Full sentences with placeholders translate well.
- Output that bypasses your hooked chokepoints, such as third-party full-screen terminal UIs or child
  processes, stays in English.
- `text/tabwriter` pads by rune count, so columns with wide (CJK) characters may not line up. Table
  libraries based on `go-runewidth` handle this.
- Plural rules aren’t modeled. Go CLIs usually write "item(s)", which translates fine.
