package inputclf

import (
	"fmt"
	"strings"
	"unicode"
)

// Мелкие функции, повторяющие поведение Python там, где оно отличается от Go.
// От них зависит побайтовое совпадение нормализации с normalize.py.

// isSpace — как str.isspace(): Unicode-пробелы плюс разделители \x1c-\x1f.
func isSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// fields — как str.split() без аргументов.
func fields(s string) []string { return strings.FieldsFunc(s, isSpace) }

// strip — как str.strip() без аргументов.
func strip(s string) string { return strings.TrimFunc(s, isSpace) }

// isWord — как \w в Python re для str.
func isWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

// decode — как bytes.decode("utf-8", errors="replace").
func decode(b []byte) string { return strings.ToValidUTF8(string(b), "\uFFFD") }

// pyRepr — как repr(str). Вызывается только для строк без двойной кавычки.
func pyRepr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == rune(quote) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r < 0x7f || unicode.IsPrint(r):
			b.WriteRune(r)
		case r < 0x100:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r < 0x10000:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// splitLines — как str.splitlines().
func splitLines(s string) []string {
	var out []string
	rs := []rune(s)
	start := 0
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\r':
			out = append(out, string(rs[start:i]))
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			start = i + 1
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, string(rs[start:i]))
			start = i + 1
		}
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}
