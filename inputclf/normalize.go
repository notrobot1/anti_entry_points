package inputclf

import (
	"fmt"
	"strings"
	"unicode"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// Порт normalize.py. Любое изменение здесь должно зеркалиться в Python
// (и наоборот), иначе модель получит не те признаки, на которых обучалась.

type param struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// sample — то же, что dict из normalize.py (только поля, нужные модели).
type sample struct {
	Name       string   `json:"name"`
	NameTokens []string `json:"name_tokens"`
	Params     []param  `json:"params"`
	Body       string   `json:"body"`
}

var exactCategory = map[string]string{
	"func_literal": "LAMBDA", "do_statement": "LOOP",
	"short_var_declaration": "ASSIGN", "variable_declarator": "ASSIGN", "var_spec": "ASSIGN",
	"array": "COLLECTION", "list": "COLLECTION", "dictionary": "COLLECTION",
	"hash": "COLLECTION", "object": "COLLECTION", "composite_literal": "COLLECTION",
	"array_creation_expression": "COLLECTION",
	"let_declaration":           "ASSIGN", "init_declarator": "ASSIGN",
	"val_definition": "ASSIGN", "var_definition": "ASSIGN",
	"match_arm": "CASE", "match_block": "", "match_pattern": "", "try_expression": "",
	"pair": "PAIR", "array_element_initializer": "PAIR", "keyed_element": "PAIR",
}

var wordRules = []struct {
	words []string
	cat   string
}{
	{[]string{"argument", "arguments", "parameter", "parameters"}, ""},
	{[]string{"call", "invocation"}, "CALL"},
	{[]string{"creation", "new"}, "NEW"},
	{[]string{"lambda", "arrow", "anonymous", "closure"}, "LAMBDA"},
	{[]string{"return"}, "RETURN"},
	{[]string{"throw", "raise"}, "THROW"},
	{[]string{"try"}, "TRY"},
	{[]string{"catch", "except", "rescue"}, "CATCH"},
	{[]string{"finally", "ensure"}, "FINALLY"},
	{[]string{"if", "conditional", "ternary", "unless"}, "IF"},
	{[]string{"else", "elif", "elsif"}, "ELSE"},
	{[]string{"for", "foreach", "while", "until"}, "LOOP"},
	{[]string{"switch", "match"}, "SWITCH"},
	{[]string{"case"}, "CASE"},
	{[]string{"assignment"}, "ASSIGN"},
	{[]string{"subscript", "index", "element"}, "INDEX"},
	{[]string{"member", "selector", "attribute", "field"}, "MEMBER"},
	{[]string{"binary", "comparison", "boolean"}, "BINOP"},
	{[]string{"unary", "not"}, "UNOP"},
	{[]string{"await"}, "AWAIT"},
	{[]string{"yield"}, "YIELD"},
	{[]string{"cast"}, "CAST"},
}

func category(nodeType string) string {
	if c, ok := exactCategory[nodeType]; ok {
		return c
	}
	words := strings.Split(nodeType, "_")
	for _, rule := range wordRules {
		for _, rw := range rule.words {
			for _, w := range words {
				if w == rw {
					return rule.cat
				}
			}
		}
	}
	return ""
}

const maxStr = 40

// isIdent — регулярка ^[$@:]*[\w][\w\d]*$ из Python (с Unicode-\w).
func isIdent(s string) bool {
	s = strings.TrimSuffix(s, "\n") // `$` в Python допускает завершающий \n
	s = strings.TrimLeft(s, "$@:")
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isWord(r) {
			return false
		}
	}
	return true
}

func text(n *ts.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return decode(src[n.StartByte():n.EndByte()])
}

func ident(s string) string {
	s = strip(strings.TrimLeft(s, "$@&*.:"))
	if s == "self" {
		return "this"
	}
	return s
}

func cleanType(s string) string {
	return strings.Join(fields(strings.TrimLeft(s, ":->? \t")), "")
}

// splitIdent: getQueryParams / get_query_params / HTTPRequest →
// [get query params] / [http request].
func splitIdent(s string) []string {
	out := []string{}
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	chars := []rune(s)
	for i, c := range chars {
		if !(unicode.IsLetter(c) || unicode.IsDigit(c)) {
			flush()
			continue
		}
		if unicode.IsUpper(c) && i > 0 &&
			(unicode.IsLower(chars[i-1]) || unicode.IsDigit(chars[i-1]) ||
				(unicode.IsUpper(chars[i-1]) && i+1 < len(chars) && unicode.IsLower(chars[i+1]))) {
			flush()
		}
		cur = append(cur, c)
	}
	flush()
	return out
}

func namedChildren(n *ts.Node) []*ts.Node {
	cnt := n.NamedChildCount()
	out := make([]*ts.Node, 0, cnt)
	for i := uint(0); i < cnt; i++ {
		if c := n.NamedChild(i); c != nil {
			out = append(out, c)
		}
	}
	return out
}

func signature(n *ts.Node) (name, params *ts.Node) {
	name = n.ChildByFieldName("name")
	params = n.ChildByFieldName("parameters")
	if params == nil {
		for _, c := range namedChildren(n) {
			if k := c.Kind(); k == "function_value_parameters" || k == "formal_parameters" {
				params = c
				break
			}
		}
	}
	d := n.ChildByFieldName("declarator")
	for d != nil && params == nil {
		if p := d.ChildByFieldName("parameters"); p != nil {
			name = d.ChildByFieldName("declarator")
			params = p
		}
		d = d.ChildByFieldName("declarator")
	}
	return name, params
}

func bodyChild(n *ts.Node) *ts.Node {
	if b := n.ChildByFieldName("body"); b != nil {
		return b
	}
	for _, c := range namedChildren(n) {
		if k := c.Kind(); k == "function_body" || k == "block" {
			return c
		}
	}
	return nil
}

func isFunc(n *ts.Node) bool {
	t := n.Kind()
	if !(strings.Contains(t, "function") || strings.Contains(t, "method") ||
		strings.Contains(t, "constructor") || t == "func_literal") {
		return false
	}
	_, params := signature(n)
	return (bodyChild(n) != nil && params != nil) || t == "method" || t == "singleton_method"
}

func findFunction(root *ts.Node, startRow, endRow int) *ts.Node {
	var best *ts.Node
	bestScore := 0.0
	var walk func(n *ts.Node)
	walk = func(n *ts.Node) {
		ns, ne := int(n.StartPosition().Row), int(n.EndPosition().Row)
		if ne < startRow || ns > endRow {
			return
		}
		if isFunc(n) {
			inter := min(ne, endRow) - max(ns, startRow) + 1
			union := max(ne, endRow) - min(ns, startRow) + 1
			if union > 0 {
				if score := float64(inter) / float64(union); score > bestScore {
					bestScore, best = score, n
				}
			}
		}
		for _, c := range namedChildren(n) {
			walk(c)
		}
	}
	walk(root)
	return best
}

func firstIdent(n *ts.Node, src []byte) *ts.Node {
	if n.ChildCount() == 0 {
		if n.IsNamed() && isIdent(text(n, src)) {
			return n
		}
		return nil
	}
	for _, c := range namedChildren(n) {
		if r := firstIdent(c, src); r != nil {
			return r
		}
	}
	return nil
}

func parseParam(n *ts.Node, src []byte) (param, bool) {
	if strings.Contains(n.Kind(), "comment") {
		return param{}, false
	}
	typeStr := ""
	if tn := n.ChildByFieldName("type"); tn != nil {
		typeStr = cleanType(text(tn, src))
	}
	nameNode := n.ChildByFieldName("name")
	if nameNode == nil {
		nameNode = n.ChildByFieldName("pattern")
	}
	if nameNode == nil {
		if d := n.ChildByFieldName("declarator"); d != nil {
			nameNode = firstIdent(d, src)
		}
	}
	if nameNode == nil {
		nameNode = firstIdent(n, src)
	}
	name := ident(text(nameNode, src))
	if name == "" {
		return param{}, false
	}
	return param{Name: name, Type: typeStr}, true
}

// emit превращает тело функции в S-выражение (список токенов).
func emit(n *ts.Node, src []byte, parts *[]string) {
	t := n.Kind()
	if strings.Contains(t, "comment") {
		return
	}
	if strings.Contains(t, "string") || strings.Contains(t, "heredoc") || t == "char_literal" {
		s := strings.Trim(text(n, src), "\"'`")
		if len([]rune(s)) <= maxStr && !strings.Contains(s, "\n") && !strings.Contains(s, `"`) {
			*parts = append(*parts, pyRepr(s))
		} else {
			*parts = append(*parts, "STR")
		}
		for _, c := range namedChildren(n) {
			if c.NamedChildCount() > 0 || strings.Contains(c.Kind(), "variable") {
				emit(c, src, parts)
			}
		}
		return
	}
	if t == "variable_name" {
		*parts = append(*parts, ident(text(n, src)))
		return
	}
	if n.ChildCount() == 0 {
		if !n.IsNamed() {
			return
		}
		s := text(n, src)
		first, _ := firstRune(s)
		switch low := strings.ToLower(s); {
		case low == "true" || low == "false":
			*parts = append(*parts, "BOOL")
		case low == "null" || low == "nil" || low == "none" || low == "undefined":
			*parts = append(*parts, "NULL")
		case s != "" && (unicode.IsDigit(first) || strings.Contains(t, "integer") ||
			strings.Contains(t, "float") || strings.Contains(t, "number")):
			*parts = append(*parts, "NUM")
		case isIdent(s):
			*parts = append(*parts, ident(s))
		}
		return
	}

	cat := category(t)
	if cat != "" {
		op := n.ChildByFieldName("operator")
		if op != nil && op.ChildCount() == 0 && (cat == "BINOP" || cat == "UNOP" || cat == "ASSIGN") {
			*parts = append(*parts, "("+cat+" "+text(op, src))
		} else {
			*parts = append(*parts, "("+cat)
		}
	}
	for _, c := range namedChildren(n) {
		emit(c, src, parts)
	}
	if cat != "" {
		*parts = append(*parts, ")")
	}
}

func firstRune(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

// normalize находит в src функцию, лучше всего совпадающую со строками
// start..end (с 1), и приводит её к виду, на котором обучалась модель.
func normalize(src []byte, ext string, start, end int) (*sample, error) {
	if regexLangs[ext] {
		return normalizeRegex(src, start, end)
	}
	lang, err := language(ext)
	if err != nil {
		return nil, err
	}
	parser := ts.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("inputclf: %w", err)
	}
	tree := parser.Parse(src, nil)
	if tree == nil {
		return nil, fmt.Errorf("inputclf: parse failed")
	}
	defer tree.Close()

	fn := findFunction(tree.RootNode(), start-1, end-1)
	if fn == nil {
		return nil, fmt.Errorf("inputclf: no function at lines %d-%d", start, end)
	}

	s := &sample{Params: []param{}}
	nameNode, paramsNode := signature(fn)
	name := ident(text(nameNode, src))
	if i := strings.LastIndex(name, "::"); i >= 0 {
		name = name[i+2:]
	}
	s.Name = name
	s.NameTokens = splitIdent(name)

	if paramsNode != nil {
		for _, c := range namedChildren(paramsNode) {
			if p, ok := parseParam(c, src); ok {
				s.Params = append(s.Params, p)
			}
		}
	}

	body := bodyChild(fn)
	if body == nil {
		body = fn
	}
	var parts []string
	emit(body, src, &parts)
	s.Body = strings.Join(parts, " ")
	return s, nil
}
