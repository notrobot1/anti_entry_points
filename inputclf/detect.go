package inputclf

import "bytes"

// Detect определяет язык исходного кода по содержимому и возвращает расширение файла.
// Если язык не распознан, возвращает "".
func Detect(code []byte) string {
	// PHP: явный тег или супер-глобалы
	if bytes.Contains(code, []byte("<?php")) || bytes.Contains(code, []byte("<?=")) {
		return ".php"
	}
	// Go: ключевое слово func + типичный синтаксис
	if bytes.Contains(code, []byte("func ")) &&
		(bytes.Contains(code, []byte(":=")) || bytes.Contains(code, []byte("package "))) {
		return ".go"
	}
	// Kotlin: fun + val/var/override
	if bytes.Contains(code, []byte("fun ")) &&
		(bytes.Contains(code, []byte("val ")) || bytes.Contains(code, []byte("var ")) ||
			bytes.Contains(code, []byte("override ")) || bytes.Contains(code, []byte(": String")) ||
			bytes.Contains(code, []byte(": Int")) || bytes.Contains(code, []byte(": Boolean"))) {
		return ".kt"
	}
	// Rust: fn + let/mut/impl
	if bytes.Contains(code, []byte("fn ")) &&
		(bytes.Contains(code, []byte("let mut")) || bytes.Contains(code, []byte("impl ")) ||
			bytes.Contains(code, []byte("-> ")) && bytes.Contains(code, []byte("{"))) {
		return ".rs"
	}
	// C#: using/namespace + типичные паттерны
	if bytes.Contains(code, []byte("using ")) && bytes.Contains(code, []byte(";")) &&
		(bytes.Contains(code, []byte("namespace ")) || bytes.Contains(code, []byte("public class")) ||
			bytes.Contains(code, []byte("private void")) || bytes.Contains(code, []byte("public void"))) {
		return ".cs"
	}
	// Java: public/private + void/class без Kotlin-специфики
	if (bytes.Contains(code, []byte("public ")) || bytes.Contains(code, []byte("private "))) &&
		(bytes.Contains(code, []byte("void ")) || bytes.Contains(code, []byte("class "))) &&
		bytes.Contains(code, []byte(";")) {
		return ".java"
	}
	// TypeScript: типы с двоеточием + async/function/const
	if (bytes.Contains(code, []byte(": string")) || bytes.Contains(code, []byte(": number")) ||
		bytes.Contains(code, []byte(": boolean")) || bytes.Contains(code, []byte(": void"))) &&
		(bytes.Contains(code, []byte("function ")) || bytes.Contains(code, []byte("const ")) ||
			bytes.Contains(code, []byte("async "))) {
		return ".ts"
	}
	// Python: def + self или import
	if bytes.Contains(code, []byte("def ")) &&
		(bytes.Contains(code, []byte("self")) || bytes.Contains(code, []byte("import ")) ||
			bytes.Contains(code, []byte(":"))) {
		return ".py"
	}
	// Ruby: def + end
	if bytes.Contains(code, []byte("def ")) && bytes.Contains(code, []byte("\nend")) {
		return ".rb"
	}
	// C++: #include + std:: или классы
	if bytes.Contains(code, []byte("#include")) &&
		(bytes.Contains(code, []byte("std::")) || bytes.Contains(code, []byte("::"))) {
		return ".cpp"
	}
	// C: #include без std::
	if bytes.Contains(code, []byte("#include")) {
		return ".c"
	}
	// JavaScript: function/const/let/var + => или function keyword
	if bytes.Contains(code, []byte("function ")) ||
		(bytes.Contains(code, []byte("=>")) &&
			(bytes.Contains(code, []byte("const ")) || bytes.Contains(code, []byte("let ")))) {
		return ".js"
	}
	return ""
}
