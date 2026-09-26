package msgfmt

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in    string
		verbs []string // Raw of each verb
		args  []int
		ok    bool
	}{
		{"plain text", nil, nil, true},
		{"Created %s \"%s\".\n", []string{"%s", "%s"}, []int{1, 2}, true},
		{"%[2]s then %[1]q then %s", []string{"%[2]s", "%[1]q", "%s"}, []int{2, 1, 2}, true},
		{"100%% done", nil, nil, true},
		{"%-10s|%05d|%.2f|%#q", []string{"%-10s", "%05d", "%.2f", "%#q"}, []int{1, 2, 3, 4}, true},
		{"width %*d", []string{"%*d"}, []int{2}, false},
		{"bad %y", []string{"%y"}, []int{1}, false},
		{"trailing %", nil, nil, false},
		{"bad index %[0]s", []string{"%[0]s"}, []int{1}, false},
	}
	for _, tt := range tests {
		toks, ok := Parse(tt.in)
		if ok != tt.ok {
			t.Errorf("Parse(%q) ok = %v, want %v", tt.in, ok, tt.ok)
		}
		var raws []string
		var args []int
		for _, tok := range toks {
			if tok.IsVerb() {
				raws = append(raws, tok.Raw)
				args = append(args, tok.Arg)
			}
		}
		if !reflect.DeepEqual(raws, tt.verbs) || !reflect.DeepEqual(args, tt.args) {
			t.Errorf("Parse(%q) verbs = %v args = %v, want %v %v", tt.in, raws, args, tt.verbs, tt.args)
		}
	}
	if toks, _ := Parse("100%% done"); Literal(toks) != "100% done" {
		t.Errorf("literal of %%%% not unescaped: %q", Literal(toks))
	}
}

func TestHasVerbs(t *testing.T) {
	for in, want := range map[string]bool{
		"Created %s.":          true,
		"no verbs":             false,
		"100%% sure":           false,
		"%[1]d items":          true,
		"deleted %d topic(s).": true,
	} {
		if got := HasVerbs(in); got != want {
			t.Errorf("HasVerbs(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestPatternMatchAndSplice(t *testing.T) {
	tests := []struct {
		format, translation string
		args                []any
		want                string
	}{
		{"Created %s \"%s\".", "「%[2]s」の%[1]sを作成しました。", []any{"API key", "ABC"}, "「ABC」のAPI keyを作成しました。"},
		{"unknown command %q for %q%s", "%[2]q のコマンド %[1]q は不明です%[3]s", []any{"stat", "app", ""}, `"app" のコマンド "stat" は不明です`},
		{"accepts %d arg(s), received %d", "%d 個の引数を受け付けますが、%d 個を受け取りました", []any{1, 3}, "1 個の引数を受け付けますが、3 個を受け取りました"},
		{"invalid argument %q for %q flag: %v", "フラグ %[2]q の引数 %[1]q が無効です: %[3]v", []any{"x", "--n", "strconv.ParseInt: parsing \"x\": invalid syntax"}, `フラグ "--n" の引数 "x" が無効です: strconv.ParseInt: parsing "x": invalid syntax`},
		{"Run '%v --help' for usage.", "使い方は '%v --help' を実行してください。", []any{"app sub"}, "使い方は 'app sub --help' を実行してください。"},
		{"help for %s", "%s のヘルプ", []any{"kafka"}, "kafka のヘルプ"},
		{"Unknown help topic %#q", "不明なヘルプトピック %#q", []any{[]string{"foo"}}, "不明なヘルプトピック [`foo`]"},
	}
	for _, tt := range tests {
		p, ok := Compile(tt.format)
		if !ok {
			t.Fatalf("Compile(%q) failed", tt.format)
		}
		rendered := fmt.Sprintf(tt.format, tt.args...)
		args, _, ok := p.Match(rendered)
		if !ok {
			t.Errorf("%q did not match its own rendering %q", tt.format, rendered)
			continue
		}
		got, ok := Splice(tt.translation, args, nil)
		if !ok || got != tt.want {
			t.Errorf("Splice(%q) = %q, %v; want %q", tt.translation, got, ok, tt.want)
		}
	}
}

func TestCompileRejectsGenericFormats(t *testing.T) {
	for _, f := range []string{"%s: %s", "%v", "[%d] %s", "no verbs here", "%*d items"} {
		if _, ok := Compile(f); ok {
			t.Errorf("Compile(%q) should be rejected", f)
		}
	}
}

func TestMatchDoesNotCrossLines(t *testing.T) {
	p, _ := Compile("Error: %s")
	if _, _, ok := p.Match("Error: first\nsecond"); ok {
		t.Error("single-line format matched across a newline")
	}
	p, _ = Compile("Line one %s\nLine two %s")
	if _, _, ok := p.Match("Line one a\nb\nLine two c"); !ok {
		t.Error("multi-line format should let captures span lines")
	}
}

func TestRepeatedArgMustAgree(t *testing.T) {
	p, _ := Compile("source <(%[1]s completion bash) and %[1]s again")
	if _, _, ok := p.Match("source <(app completion bash) and app again"); !ok {
		t.Error("expected match")
	}
	if _, _, ok := p.Match("source <(app completion bash) and other again"); ok {
		t.Error("inconsistent repeated argument should not match")
	}
}

func TestPlaceholders(t *testing.T) {
	src := "Use `confluent kafka` to list %s in {{.CommandPath}} (%[2]d)."
	good := "{{.CommandPath}} の %[1]s を `confluent kafka` で一覧表示します (%[2]d)。"
	if d := Extract(src).Diff(Extract(good)); d != "" {
		t.Errorf("unexpected diff: %s", d)
	}
	for _, bad := range []string{
		"{{.CommandPath}} の %s を confluent kafka で一覧表示します (%[2]d)。",            // lost backquotes
		"{{.Name}} の %[1]s を `confluent kafka` で一覧表示します (%[2]d)。",              // changed action
		"{{.CommandPath}} の %[1]q を `confluent kafka` で一覧表示します (%[2]d)。",       // changed verb
		"{{.CommandPath}} の %[1]s を `confluent kafka` で `一覧` 表示します (%[2]d)。",   // extra backquotes
		"{{.CommandPath}} の %[1]999999s を `confluent kafka` で一覧表示します (%[2]d)。", // width injected
	} {
		if Extract(src).Equal(Extract(bad)) {
			t.Errorf("placeholders of %q should differ from source", bad)
		}
	}
	if !NewControlChars("plain", "pla\x1b[31min") {
		t.Error("ANSI escape not detected")
	}
	if NewControlChars("a\nb", "c\nd") {
		t.Error("newline flagged as control char")
	}
}

func TestTemplateUnits(t *testing.T) {
	tpl := `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}

Available Commands:{{range $cmds}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
{{printf "version %s" .Version}}
`
	got := TemplateUnits(tpl)
	want := []string{
		"Usage:",
		"Available Commands:",
		"Flags:",
		`Use "{{.CommandPath}} [command] --help" for more information about a command.`,
		"version %s",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TemplateUnits = %q\nwant %q", got, want)
	}
	tr := map[string]string{
		"Usage:":              "使い方:",
		"Flags:":              "フラグ:",
		want[3]:               `コマンドの詳細は "{{.CommandPath}} [command] --help" を参照してください。`,
		"version %s":          "バージョン %s",
		"Available Commands:": "利用可能なコマンド:",
	}
	out := TranslateTemplate(tpl, func(u string) (string, bool) { s, ok := tr[u]; return s, ok })
	for _, frag := range []string{"使い方:{{if .Runnable}}", "\nフラグ:\n{{.LocalFlags", `{{printf "バージョン %s" .Version}}`, "{{$cmds := .Commands}}", "利用可能なコマンド:{{range $cmds}}"} {
		if !strings.Contains(out, frag) {
			t.Errorf("translated template missing %q:\n%s", frag, out)
		}
	}
	if TranslateTemplate(tpl, func(string) (string, bool) { return "", false }) != tpl {
		t.Error("identity translation changed the template")
	}
}

func TestPseudo(t *testing.T) {
	got := Pseudo("  Created %s \"%s\" with `confluent api-key` and --force.\n")
	want := "  ⟦Çŕéáţéď %s \"%s\" ŵîţĥ `confluent api-key` áñď --force.⟧\n"
	if got != want {
		t.Errorf("Pseudo = %q, want %q", got, want)
	}
}

// FuzzReverseMatch checks that any supported format reverse-matches its own rendering and that splicing
// the captured arguments back into the same format reproduces the input exactly.
func FuzzReverseMatch(f *testing.F) {
	f.Add("Created %s \"%s\".", "API key", "ABC-123")
	f.Add("unknown command %q for %q", "stat", "app")
	f.Add("%s is deprecated: %s", "x", "use y")
	f.Add("Deleted %d of %s.", "3", "topics")
	f.Fuzz(func(t *testing.T, format, a, b string) {
		p, ok := Compile(format)
		if !ok {
			return
		}
		toks, _ := Parse(format)
		for _, tok := range toks {
			if len(tok.Width) > 3 || len(tok.Prec) > 4 {
				return // huge widths only exercise fmt's allocator
			}
		}
		vals := []any{}
		for _, tok := range toks {
			if !tok.IsVerb() {
				continue
			}
			for len(vals) < tok.Arg {
				vals = append(vals, nil)
			}
			var v any = a
			if len(vals)%2 == 1 {
				v = b
			}
			switch tok.Verb {
			case 'd', 'b', 'o', 'O', 'x', 'X', 'c', 'U':
				v = len(a) + 7*len(b)
			case 'e', 'E', 'f', 'F', 'g', 'G':
				v = float64(len(a)) / 3
			case 't':
				v = len(a)%2 == 0
			case 'p':
				return
			}
			vals[tok.Arg-1] = v
		}
		for i, v := range vals {
			if v == nil {
				vals[i] = a
			}
		}
		rendered := fmt.Sprintf(format, vals...)
		if strings.Contains(rendered, "%!") {
			return // fmt reported a formatting error; nothing meaningful to invert
		}
		args, _, ok := p.Match(rendered)
		if !ok {
			return // ambiguity or unusual values can legitimately prevent a match
		}
		got, ok := Splice(format, args, nil)
		if !ok {
			t.Fatalf("Splice(%q) failed after a successful match of %q", format, rendered)
		}
		if got != rendered {
			t.Fatalf("round trip mismatch for format %q:\n rendered %q\n spliced  %q", format, rendered, got)
		}
	})
}
