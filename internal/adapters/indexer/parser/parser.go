package parser

import (
	"coderefinery/internal/core/domain"
	"time"
)

type Parser interface {
	Parse(filePath string, content []byte, modTime time.Time) ([]domain.CodeChunk, error)
	SupportsLanguage(lang string) bool
}

// GetParser gibt den besten verfügbaren Parser für eine Sprache zurück
func GetParser(lang string) Parser {

	// 1. Versuch: Go's nativer AST Parser (beste Qualität für Go)
	if lang == "go" {
		return &GoParser{}
	}

	// 2. Fallback: Universal AST Parser
	//    Funktioniert für ALLE Sprachen durch intelligente Heuristiken:
	//    - C, C++, Rust, Java, C#, Kotlin, Swift, PHP, Scala
	//    - Ruby, Crystal, Lua
	//    - Lisp, Scheme, Clojure
	//    - OCaml, F#, Haskell, Elm
	//    - Assembly, SQL, Shell Scripts
	//    - Und jede unbekannte Sprache
	return NewUniversalASTParser()
}

// GetParserForceUniversal erzwingt den Universal Parser (für Testing)
func GetParserForceUniversal() Parser {
	return NewUniversalASTParser()
}
