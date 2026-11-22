package inputclf

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Regex-фолбэк для языков без tree-sitter грамматики (порт _normalize_regex).

var regexLangs = map[string]bool{".cj": true, ".swift": true}

var keywordTokens = map[string]bool{
	"if": true, "else": true, "for": true, "while": true, "return": true, "try": true,
	"catch": true, "throw": true, "match": true, "case": true, "let": true, "var": true,
	"val": true, "new": true, "await": true, "async": true, "true": true, "false": true,
	"null": true, "nil": true, "none": true,
}

const w = `[\p{L}\p{N}_]` // \w из Python

var (
	reDQ     = regexp.MustCompile(`"[^"]*"`)
	reSQ     = regexp.MustCompile(`'[^']*'`)
	rePunct  = regexp.MustCompile(`[{}()\[\];,]`)
	reTokId  = regexp.MustCompile(`^[a-zA-Z_]` + w + `*$`)
	reFunc   = regexp.MustCompile(`(?:public\s+|private\s+|protected\s+|static\s+|async\s+|override\s+)*func\s+(` + w + `+)\s*\(`)
	reParamT = regexp.MustCompile(`^(` + w + `+)\s*[!?]?\s*:\s*([\p{L}\p{N}_.<>\[\]?]+)`)
	reParam  = regexp.MustCompile(`^(` + w + `+)`)
)

// replaceNumbers — re.sub(r'\b\d+(\.\d+)?\b', 'NUM', s) с Unicode-семантикой Python.
func replaceNumbers(s string) string {
	rs := []rune(s)
	n := len(rs)
	wordAt := func(i int) bool { return i >= 0 && i < n && isWord(rs[i]) }
	digits := func(i int) int {
		for i < n && unicode.IsDigit(rs[i]) {
			i++
		}
		return i
	}
	var b strings.Builder
	for i := 0; i < n; {
		if unicode.IsDigit(rs[i]) && !wordAt(i-1) {
			j := digits(i)
			end := -1
			if j < n && rs[j] == '.' {
				if k := digits(j + 1); k > j+1 && !wordAt(k) {
					end = k
				} else {
					end = j
				}
			} else if !wordAt(j) {
				end = j
			}
			if end > 0 {
				b.WriteString("NUM")
				i = end
				continue
			}
		}
		b.WriteRune(rs[i])
		i++
	}
	return b.String()
}

func tokenizeBody(s string) string {
	s = reDQ.ReplaceAllString(s, "STR")
	s = reSQ.ReplaceAllString(s, "STR")
	s = replaceNumbers(s)
	s = rePunct.ReplaceAllString(s, " ")
	var toks []string
	for _, tok := range fields(s) {
		low := strings.ToLower(tok)
		if keywordTokens[low] {
			toks = append(toks, strings.ToUpper(low))
		} else if reTokId.MatchString(tok) {
			toks = append(toks, tok)
		}
	}
	return strings.Join(toks, " ")
}

// matchClose возвращает индекс скобки, закрывающей уровень depth, либо -1.
func matchClose(s string, from int, open, close byte, depth int) int {
	for i := from; i < len(s); i++ {
		switch s[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func normalizeRegex(src []byte, start, end int) (*sample, error) {
	lines := splitLines(decode(src))
	lo, hi := max(start-1, 0), min(end, len(lines))
	snippet := ""
	if lo < hi {
		snippet = strings.Join(lines[lo:hi], "\n")
	}

	m := reFunc.FindStringSubmatchIndex(snippet)
	if m == nil {
		return nil, fmt.Errorf("inputclf: no function at lines %d-%d", start, end)
	}
	name := snippet[m[2]:m[3]]
	s := &sample{Name: name, NameTokens: splitIdent(name), Params: []param{}}

	sig := snippet[m[0]:]
	pStart := strings.IndexByte(sig, '(') + 1
	pEnd := matchClose(sig, pStart, '(', ')', 1)
	if pEnd < 0 {
		pEnd = pStart
	}
	for _, part := range strings.Split(sig[pStart:pEnd], ",") {
		part = strip(part)
		if part == "" {
			continue
		}
		if pm := reParamT.FindStringSubmatch(part); pm != nil {
			s.Params = append(s.Params, param{Name: pm[1], Type: pm[2]})
		} else if wm := reParam.FindStringSubmatch(part); wm != nil {
			s.Params = append(s.Params, param{Name: wm[1]})
		}
	}

	if off := strings.IndexByte(sig, '{'); off >= 0 {
		bStart := m[0] + off
		bEnd := matchClose(snippet, bStart, '{', '}', 0)
		if bEnd < 0 {
			bEnd = bStart + 1 // как в Python: пустое тело, если скобка не закрыта
		}
		s.Body = tokenizeBody(snippet[bStart+1 : bEnd])
	} else {
		s.Body = tokenizeBody(snippet)
	}
	return s, nil
}
