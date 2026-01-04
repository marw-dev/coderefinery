package parser

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"coderefinery/internal/domain"
)

// UniversalASTParser ist ein intelligenter Parser, der für ALLE Sprachen funktioniert
// durch Pattern-Matching und heuristische Analyse statt sprachspezifischer Grammars
type UniversalASTParser struct {
	maxChunkSize int
	minChunkSize int
}

func NewUniversalASTParser() *UniversalASTParser {
	return &UniversalASTParser{
		maxChunkSize: 2000,
		minChunkSize: 50,
	}
}

// LanguageProfile definiert die Syntax-Charakteristiken einer Sprache
type LanguageProfile struct {
	// Block delimiters
	BlockStart []string // {, begin, do, etc.
	BlockEnd   []string // }, end, done, etc.

	// Function/Method patterns
	FunctionPatterns []*regexp.Regexp
	ClassPatterns    []*regexp.Regexp

	// Comment styles
	LineComment  []string // //, #, --, etc.
	BlockComment []struct{ Start, End string }

	// Indentation-based?
	IndentationBased bool
}

// detectLanguageProfile analysiert die Sprache und gibt das passende Profil zurück
func (p *UniversalASTParser) detectLanguageProfile(lang string, content []byte) LanguageProfile {
	// Basis-Profile für bekannte Sprach-Familien
	profiles := map[string]LanguageProfile{
		// C-Familie (C, C++, Java, C#, Rust, Go, JavaScript, TypeScript, PHP, Kotlin, Swift)
		"c-family": {
			BlockStart: []string{"{"},
			BlockEnd:   []string{"}"},
			FunctionPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*(?:pub\s+)?(?:async\s+)?(?:fn|func|function|def|fun)\s+\w+\s*\(`),
				regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal)?\s*(?:static|async|virtual|override)?\s*\w+\s+\w+\s*\(`),
				regexp.MustCompile(`(?m)^\s*\w+\s*:\s*\([^)]*\)\s*=>`), // Arrow functions
			},
			ClassPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*(?:pub\s+)?(?:class|struct|interface|trait|enum|impl)\s+\w+`),
			},
			LineComment:  []string{"//"},
			BlockComment: []struct{ Start, End string }{{Start: "/*", End: "*/"}},
		},

		// Python-Familie (Python)
		"python": {
			IndentationBased: true,
			FunctionPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*(?:async\s+)?def\s+\w+\s*\(`),
			},
			ClassPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*class\s+\w+`),
			},
			LineComment:  []string{"#"},
			BlockComment: []struct{ Start, End string }{{Start: `"""`, End: `"""`}, {Start: "'''", End: "'''"}},
		},

		// Ruby-Familie (Ruby, Crystal)
		"ruby": {
			BlockStart: []string{"do", "begin", "class", "module", "def"},
			BlockEnd:   []string{"end"},
			FunctionPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*def\s+\w+`),
			},
			ClassPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*(?:class|module)\s+\w+`),
			},
			LineComment: []string{"#"},
		},

		// Lisp-Familie (Lisp, Scheme, Clojure)
		"lisp": {
			BlockStart: []string{"("},
			BlockEnd:   []string{")"},
			FunctionPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*\(def(?:n|un|unc)?\s+\w+`),
			},
			LineComment: []string{";", ";;"},
		},

		// ML-Familie (OCaml, F#, Haskell, Elm)
		"ml": {
			BlockStart: []string{"let", "match", "if"},
			BlockEnd:   []string{"in", "end"},
			FunctionPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*let\s+(?:rec\s+)?\w+\s*(?:\(|=)`),
				regexp.MustCompile(`(?m)^\w+\s*::\s*.*->`),
			},
			LineComment:  []string{"--", "//"},
			BlockComment: []struct{ Start, End string }{{Start: "(*", End: "*)"}, {Start: "{-", End: "-}"}},
		},

		// Shell-Familie (Bash, Zsh, Fish)
		"shell": {
			FunctionPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*(?:function\s+)?\w+\s*\(\s*\)`),
			},
			LineComment: []string{"#"},
		},

		// Assembly
		"asm": {
			FunctionPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*\w+\s*:\s*$`), // Labels
				regexp.MustCompile(`(?m)^\s*(?:PROC|proc|FUNCTION|function)\s+\w+`),
			},
			LineComment: []string{";", "#", "//"},
		},

		// SQL
		"sql": {
			FunctionPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?mi)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:PROCEDURE|FUNCTION)\s+\w+`),
			},
			LineComment:  []string{"--"},
			BlockComment: []struct{ Start, End string }{{Start: "/*", End: "*/"}},
		},

		// Lua
		"lua": {
			BlockStart: []string{"function", "do", "then"},
			BlockEnd:   []string{"end"},
			FunctionPatterns: []*regexp.Regexp{
				regexp.MustCompile(`(?m)^\s*(?:local\s+)?function\s+\w+`),
			},
			LineComment:  []string{"--"},
			BlockComment: []struct{ Start, End string }{{Start: "--[[", End: "]]"}},
		},
	}

	// Language Mapping zu Profilen
	langMap := map[string]string{
		"c": "c-family", "cpp": "c-family", "cc": "c-family", "cxx": "c-family", "h": "c-family", "hpp": "c-family",
		"java": "c-family", "cs": "c-family", "rs": "c-family", "go": "c-family",
		"js": "c-family", "ts": "c-family", "jsx": "c-family", "tsx": "c-family",
		"php": "c-family", "kt": "c-family", "swift": "c-family", "scala": "c-family",
		"py": "python", "pyw": "python",
		"rb": "ruby", "cr": "ruby",
		"lisp": "lisp", "cl": "lisp", "scm": "lisp", "clj": "lisp", "cljs": "lisp",
		"ml": "ml", "mli": "ml", "fs": "ml", "fsi": "ml", "hs": "ml", "elm": "ml",
		"sh": "shell", "bash": "shell", "zsh": "shell", "fish": "shell",
		"asm": "asm", "s": "asm", "S": "asm",
		"sql": "sql",
		"lua": "lua",
	}

	profile, ok := profiles[langMap[lang]]
	if !ok {
		// Fallback: Heuristische Erkennung durch Content-Analyse
		profile = p.detectProfileFromContent(content)
	}

	return profile
}

// detectProfileFromContent analysiert den Code-Inhalt für unbekannte Sprachen
func (p *UniversalASTParser) detectProfileFromContent(content []byte) LanguageProfile {
	text := string(content)
	profile := LanguageProfile{
		BlockStart:       []string{},
		BlockEnd:         []string{},
		FunctionPatterns: []*regexp.Regexp{},
		LineComment:      []string{},
	}

	// Zähle Vorkommen verschiedener Patterns
	braceCount := strings.Count(text, "{")
	indentCount := 0
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			indentCount++
		}
	}

	// Entscheide basierend auf Heuristiken
	if braceCount > len(lines)/10 {
		// Wahrscheinlich brace-based
		profile.BlockStart = []string{"{"}
		profile.BlockEnd = []string{"}"}
		profile.LineComment = []string{"//", "#"}
	} else if float64(indentCount)/float64(len(lines)) > 0.3 {
		// Wahrscheinlich indentation-based
		profile.IndentationBased = true
		profile.LineComment = []string{"#", "//"}
	}

	// Suche nach Function-Patterns
	if strings.Contains(text, "def ") {
		profile.FunctionPatterns = append(profile.FunctionPatterns,
			regexp.MustCompile(`(?m)^\s*def\s+\w+`))
	}
	if strings.Contains(text, "function ") {
		profile.FunctionPatterns = append(profile.FunctionPatterns,
			regexp.MustCompile(`(?m)^\s*function\s+\w+`))
	}

	return profile
}

func (p *UniversalASTParser) Parse(filePath string, content []byte, modTime time.Time) ([]domain.CodeChunk, error) {
	ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
	if ext == "" {
		ext = "txt"
	}

	profile := p.detectLanguageProfile(ext, content)
	lines := strings.Split(string(content), "\n")

	// Wähle Parsing-Strategie basierend auf Profil
	if profile.IndentationBased {
		return p.parseIndentationBased(filePath, lines, ext, modTime, profile)
	} else if len(profile.BlockStart) > 0 {
		return p.parseBlockBased(filePath, lines, ext, modTime, profile)
	} else {
		return p.parseGeneric(filePath, lines, ext, modTime, profile)
	}
}

// parseBlockBased für Sprachen mit expliziten Block-Delimitern (C, Ruby, Lua, etc.)
func (p *UniversalASTParser) parseBlockBased(filePath string, lines []string, lang string, modTime time.Time, profile LanguageProfile) ([]domain.CodeChunk, error) {
	var chunks []domain.CodeChunk
	var currentChunk strings.Builder
	chunkStart := 0
	blockDepth := 0
	var currentSignature string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Block-Tiefe tracken
		for _, start := range profile.BlockStart {
			blockDepth += strings.Count(line, start)
		}
		for _, end := range profile.BlockEnd {
			blockDepth -= strings.Count(line, end)
		}

		// Erkenne Funktions-Definitionen
		for _, pattern := range profile.FunctionPatterns {
			if pattern.MatchString(line) {
				currentSignature = trimmed
				break
			}
		}

		currentChunk.WriteString(line + "\n")

		// Split wenn Block geschlossen wird
		if blockDepth == 0 && currentChunk.Len() > p.minChunkSize {
			chunks = append(chunks, p.createChunk(
				filePath, chunkStart, i, currentChunk.String(),
				lang, modTime, currentSignature, profile,
			))
			currentChunk.Reset()
			chunkStart = i + 1
			currentSignature = ""
		}

		// Size-basierter Split
		if currentChunk.Len() > p.maxChunkSize && blockDepth == 0 {
			chunks = append(chunks, p.createChunk(
				filePath, chunkStart, i, currentChunk.String(),
				lang, modTime, currentSignature, profile,
			))
			currentChunk.Reset()
			chunkStart = i + 1
		}
	}

	if currentChunk.Len() > p.minChunkSize {
		chunks = append(chunks, p.createChunk(
			filePath, chunkStart, len(lines)-1, currentChunk.String(),
			lang, modTime, currentSignature, profile,
		))
	}

	return chunks, nil
}

// parseIndentationBased für Python, YAML, etc.
func (p *UniversalASTParser) parseIndentationBased(filePath string, lines []string, lang string, modTime time.Time, profile LanguageProfile) ([]domain.CodeChunk, error) {
	var chunks []domain.CodeChunk
	var currentChunk strings.Builder
	chunkStart := 0
	var currentSignature string
	var lastIndent int

	for i, line := range lines {
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		trimmed := strings.TrimSpace(line)

		// Erkenne Top-Level Definitionen
		isTopLevel := false
		for _, pattern := range profile.FunctionPatterns {
			if pattern.MatchString(line) && indent <= lastIndent {
				isTopLevel = true
				currentSignature = trimmed
				break
			}
		}

		for _, pattern := range profile.ClassPatterns {
			if pattern.MatchString(line) && indent <= lastIndent {
				isTopLevel = true
				currentSignature = trimmed
				break
			}
		}

		if isTopLevel && currentChunk.Len() > p.minChunkSize {
			chunks = append(chunks, p.createChunk(
				filePath, chunkStart, i-1, currentChunk.String(),
				lang, modTime, currentSignature, profile,
			))
			currentChunk.Reset()
			chunkStart = i
		}

		currentChunk.WriteString(line + "\n")
		lastIndent = indent

		if currentChunk.Len() > p.maxChunkSize {
			chunks = append(chunks, p.createChunk(
				filePath, chunkStart, i, currentChunk.String(),
				lang, modTime, currentSignature, profile,
			))
			currentChunk.Reset()
			chunkStart = i + 1
		}
	}

	if currentChunk.Len() > p.minChunkSize {
		chunks = append(chunks, p.createChunk(
			filePath, chunkStart, len(lines)-1, currentChunk.String(),
			lang, modTime, currentSignature, profile,
		))
	}

	return chunks, nil
}

// parseGeneric für unbekannte/unstrukturierte Formate
func (p *UniversalASTParser) parseGeneric(filePath string, lines []string, lang string, modTime time.Time, profile LanguageProfile) ([]domain.CodeChunk, error) {
	var chunks []domain.CodeChunk
	var currentChunk strings.Builder
	chunkStart := 0
	var currentSignature string

	for i, line := range lines {
		// Versuche Funktionen zu erkennen
		for _, pattern := range profile.FunctionPatterns {
			if pattern.MatchString(line) {
				currentSignature = strings.TrimSpace(line)
				break
			}
		}

		currentChunk.WriteString(line + "\n")

		// Split bei Leerzeilen
		if strings.TrimSpace(line) == "" && currentChunk.Len() > p.minChunkSize {
			chunks = append(chunks, p.createChunk(
				filePath, chunkStart, i, currentChunk.String(),
				lang, modTime, currentSignature, profile,
			))
			currentChunk.Reset()
			chunkStart = i + 1
		}

		if currentChunk.Len() > p.maxChunkSize {
			chunks = append(chunks, p.createChunk(
				filePath, chunkStart, i, currentChunk.String(),
				lang, modTime, currentSignature, profile,
			))
			currentChunk.Reset()
			chunkStart = i + 1
		}
	}

	if currentChunk.Len() > p.minChunkSize {
		chunks = append(chunks, p.createChunk(
			filePath, chunkStart, len(lines)-1, currentChunk.String(),
			lang, modTime, currentSignature, profile,
		))
	}

	return chunks, nil
}

func (p *UniversalASTParser) createChunk(filePath string, startLine, endLine int, content, lang string, modTime time.Time, signature string, profile LanguageProfile) domain.CodeChunk {
	content = strings.TrimSpace(content)

	// Extrahiere Comments
	comments := p.extractComments(content, profile)

	// Bestimme Chunk-Type
	chunkType := p.detectChunkType(content, signature, profile)

	return domain.CodeChunk{
		ID:           generateID(filePath, startLine),
		FilePath:     filePath,
		Content:      content,
		Signature:    signature,
		Comments:     comments,
		StartLine:    startLine + 1,
		EndLine:      endLine + 1,
		ChunkType:    chunkType,
		Language:     lang,
		LastModified: modTime,
	}
}

// extractComments extrahiert Kommentare aus dem Code
func (p *UniversalASTParser) extractComments(content string, profile LanguageProfile) string {
	var comments []string
	lines := strings.SplitSeq(content, "\n")

	for line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, commentStart := range profile.LineComment {
			if after, ok :=strings.CutPrefix(trimmed, commentStart); ok  {
				comments = append(comments, after)
				break
			}
		}
	}

	return strings.Join(comments, "\n")
}

// detectChunkType versucht den Typ des Code-Blocks zu erkennen
func (p *UniversalASTParser) detectChunkType(content, signature string, profile LanguageProfile) domain.ChunkType {
	lower := strings.ToLower(content)
	sigLower := strings.ToLower(signature)

	// Prüfe auf Class
	for _, pattern := range profile.ClassPatterns {
		if pattern.MatchString(content) {
			if strings.Contains(sigLower, "interface") {
				return domain.ChunkTypeInterface
			}
			if strings.Contains(sigLower, "struct") {
				return domain.ChunkTypeStruct
			}
			return domain.ChunkTypeClass
		}
	}

	// Prüfe auf Function
	for _, pattern := range profile.FunctionPatterns {
		if pattern.MatchString(content) {
			return domain.ChunkTypeFunction
		}
	}

	// Heuristiken
	if strings.Contains(lower, "class ") {
		return domain.ChunkTypeClass
	}
	if strings.Contains(lower, "func ") || strings.Contains(lower, "function ") || strings.Contains(lower, "def ") {
		return domain.ChunkTypeFunction
	}
	if strings.Contains(lower, "interface ") {
		return domain.ChunkTypeInterface
	}
	if strings.Contains(lower, "struct ") {
		return domain.ChunkTypeStruct
	}

	return domain.ChunkTypeGeneric
}

func (p *UniversalASTParser) SupportsLanguage(lang string) bool {
	return true // Universal!
}
