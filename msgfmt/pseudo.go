package msgfmt

import (
	"regexp"
	"strings"
)

var (
	pseudoProtect = regexp.MustCompile("\\{\\{.*?\\}\\}|`[^`]*`|%[-+# 0]*(?:\\[\\d+\\])?(?:\\d+)?(?:\\.(?:\\[\\d+\\])?\\d*)?(?:\\[\\d+\\])?[a-zA-Z%]|https?://\\S+|--?[a-zA-Z][-a-zA-Z0-9]*|<[^>\\s]+>|\"[^\"\\n]*\"")
	pseudoMap     = map[rune]rune{
		'a': 'á', 'b': 'ƀ', 'c': 'ç', 'd': 'ď', 'e': 'é', 'f': 'ƒ', 'g': 'ĝ', 'h': 'ĥ', 'i': 'î', 'j': 'ĵ',
		'k': 'ķ', 'l': 'ļ', 'm': 'ɱ', 'n': 'ñ', 'o': 'ö', 'p': 'þ', 'q': 'ǫ', 'r': 'ŕ', 's': 'š', 't': 'ţ',
		'u': 'û', 'v': 'ṽ', 'w': 'ŵ', 'x': 'ẋ', 'y': 'ý', 'z': 'ž',
		'A': 'Å', 'B': 'Ɓ', 'C': 'Ç', 'D': 'Ď', 'E': 'É', 'F': 'Ƒ', 'G': 'Ĝ', 'H': 'Ĥ', 'I': 'Î', 'J': 'Ĵ',
		'K': 'Ķ', 'L': 'Ļ', 'M': 'Ṁ', 'N': 'Ñ', 'O': 'Ö', 'P': 'Þ', 'Q': 'Ǫ', 'R': 'Ŕ', 'S': 'Š', 'T': 'Ţ',
		'U': 'Û', 'V': 'Ṽ', 'W': 'Ŵ', 'X': 'Ẋ', 'Y': 'Ý', 'Z': 'Ž',
	}
)

// Pseudo returns a pseudo-localized rendering of s: letters are replaced by accented look-alikes and the
// text is wrapped in ⟦ ⟧, while placeholders, template actions, backquoted spans, quoted values, URLs,
// flags and <args> are left intact. It makes untranslated or unhooked strings easy to spot.
func Pseudo(s string) string {
	lead, core, trail := SplitSpace(s)
	if core == "" {
		return s
	}
	var b strings.Builder
	b.WriteString(lead)
	b.WriteString("⟦")
	last := 0
	for _, loc := range pseudoProtect.FindAllStringIndex(core, -1) {
		accent(&b, core[last:loc[0]])
		b.WriteString(core[loc[0]:loc[1]])
		last = loc[1]
	}
	accent(&b, core[last:])
	b.WriteString("⟧")
	b.WriteString(trail)
	return b.String()
}

func accent(b *strings.Builder, s string) {
	for _, r := range s {
		if m, ok := pseudoMap[r]; ok {
			b.WriteRune(m)
		} else {
			b.WriteRune(r)
		}
	}
}

// SplitSpace splits s into leading whitespace, the trimmed core, and trailing whitespace.
func SplitSpace(s string) (lead, core, trail string) {
	core = strings.TrimLeft(s, " \t\r\n")
	lead = s[:len(s)-len(core)]
	trimmed := strings.TrimRight(core, " \t\r\n")
	trail = core[len(trimmed):]
	return lead, trimmed, trail
}
