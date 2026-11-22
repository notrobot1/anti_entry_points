package inputclf

import (
	"fmt"
	"sync"
	"unsafe"

	ts "github.com/tree-sitter/go-tree-sitter"

	ts_kotlin "github.com/tree-sitter-grammars/tree-sitter-kotlin/bindings/go"
	ts_c_sharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	ts_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
	ts_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
	ts_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	ts_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
	ts_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	ts_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	ts_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	ts_ruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
	ts_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	ts_scala "github.com/tree-sitter/tree-sitter-scala/bindings/go"
	ts_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// Расширение → грамматика. Повторяет _load_languages() из normalize.py.
var grammars = map[string]func() unsafe.Pointer{
	".py":    ts_python.Language,
	".go":    ts_go.Language,
	".js":    ts_javascript.Language,
	".jsx":   ts_javascript.Language,
	".mjs":   ts_javascript.Language,
	".ts":    ts_typescript.LanguageTypescript,
	".tsx":   ts_typescript.LanguageTSX,
	".java":  ts_java.Language,
	".rb":    ts_ruby.Language,
	".cs":    ts_c_sharp.Language,
	".c":     ts_c.Language,
	".h":     ts_c.Language,
	".rs":    ts_rust.Language,
	".cpp":   ts_cpp.Language,
	".cc":    ts_cpp.Language,
	".cxx":   ts_cpp.Language,
	".hpp":   ts_cpp.Language,
	".kt":    ts_kotlin.Language,
	".kts":   ts_kotlin.Language,
	".php":   ts_php.LanguagePHP,
	".scala": ts_scala.Language,
}

var (
	langMu    sync.Mutex
	langCache = map[string]*ts.Language{}
)

func language(ext string) (*ts.Language, error) {
	langMu.Lock()
	defer langMu.Unlock()
	if l, ok := langCache[ext]; ok {
		return l, nil
	}
	g, ok := grammars[ext]
	if !ok {
		return nil, fmt.Errorf("inputclf: unsupported extension %q", ext)
	}
	l := ts.NewLanguage(g())
	langCache[ext] = l
	return l, nil
}
