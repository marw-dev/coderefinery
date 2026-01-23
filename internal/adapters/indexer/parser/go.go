package parser

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"time"

	"coderefinery/internal/core/domain"
)

type GoParser struct{}

func (p *GoParser) Parse(filePath string, content []byte, modTime time.Time) ([]domain.CodeChunk, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var chunks []domain.CodeChunk
	relPath := filePath

	// Extract package-level imports
	imports := extractImports(file)

	// Parse functions
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			chunk := p.parseFuncDecl(fset, file, x, relPath, imports, modTime)
			chunks = append(chunks, chunk)

		case *ast.GenDecl:
			// Parse type declarations (structs, interfaces)
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						chunk := p.parseTypeSpec(fset, file, typeSpec, x, relPath, imports, modTime)
						if chunk.Content != "" {
							chunks = append(chunks, chunk)
						}
					}
				}
			}
		}
		return true
	})

	return chunks, nil
}

func (p *GoParser) parseFuncDecl(fset *token.FileSet, file *ast.File,
	fn *ast.FuncDecl, filePath string, imports []string, modTime time.Time) domain.CodeChunk {

	start := fset.Position(fn.Pos())
	end := fset.Position(fn.End())

	// Extract function signature
	var signature strings.Builder
	if fn.Recv != nil {
		signature.WriteString("func (")
		if len(fn.Recv.List) > 0 {
			signature.WriteString(formatFieldList(fn.Recv.List[0]))
		}
		signature.WriteString(") ")
	} else {
		signature.WriteString("func ")
	}
	signature.WriteString(fn.Name.Name)
	signature.WriteString(formatParams(fn.Type.Params))
	if fn.Type.Results != nil {
		signature.WriteString(" ")
		signature.WriteString(formatResults(fn.Type.Results))
	}

	// Extract content
	content := extractNodeContent(fset, file, fn)

	// Extract comments
	comments := extractComments(file, fn.Doc)

	chunkType := domain.ChunkTypeFunction
	if fn.Recv != nil {
		chunkType = domain.ChunkTypeMethod
	}

	return domain.CodeChunk{
		ID:           generateID(filePath, start.Line),
		FilePath:     filePath,
		Content:      content,
		StartLine:    start.Line,
		EndLine:      end.Line,
		ChunkType:    chunkType,
		Signature:    signature.String(),
		Language:     "go",
		Imports:      imports,
		Comments:     comments,
		LastModified: modTime,
	}
}

func (p *GoParser) parseTypeSpec(fset *token.FileSet, file *ast.File,
	typeSpec *ast.TypeSpec, genDecl *ast.GenDecl, filePath string,
	imports []string, modTime time.Time) domain.CodeChunk {

	start := fset.Position(genDecl.Pos())
	end := fset.Position(genDecl.End())

	var chunkType domain.ChunkType
	var signature string

	switch t := typeSpec.Type.(type) {
	case *ast.StructType:
		chunkType = domain.ChunkTypeStruct
		signature = fmt.Sprintf("type %s struct", typeSpec.Name.Name)
	case *ast.InterfaceType:
		chunkType = domain.ChunkTypeInterface
		signature = fmt.Sprintf("type %s interface", typeSpec.Name.Name)
	default:
		chunkType = domain.ChunkTypeGeneric
		signature = fmt.Sprintf("type %s %v", typeSpec.Name.Name, t)
	}

	content := extractNodeContent(fset, file, genDecl)
	comments := extractComments(file, genDecl.Doc)

	return domain.CodeChunk{
		ID:           generateID(filePath, start.Line),
		FilePath:     filePath,
		Content:      content,
		StartLine:    start.Line,
		EndLine:      end.Line,
		ChunkType:    chunkType,
		Signature:    signature,
		Language:     "go",
		Imports:      imports,
		Comments:     comments,
		LastModified: modTime,
	}
}

func (p *GoParser) SupportsLanguage(lang string) bool {
	return lang == "go"
}

// Helper functions
func extractImports(file *ast.File) []string {
	var imports []string
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, path)
	}
	return imports
}

func extractComments(file *ast.File, doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	var comments strings.Builder
	for _, comment := range doc.List {
		comments.WriteString(strings.TrimPrefix(comment.Text, "//"))
		comments.WriteString("\n")
	}
	return strings.TrimSpace(comments.String())
}

func extractNodeContent(fset *token.FileSet, file *ast.File, node ast.Node) string {
	start := fset.Position(node.Pos())
	end := fset.Position(node.End())

	// Read file to extract exact content
	content, err := os.ReadFile(fset.File(node.Pos()).Name())
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	if start.Line > len(lines) || end.Line > len(lines) {
		return ""
	}

	return strings.Join(lines[start.Line-1:end.Line], "\n")
}

func formatFieldList(field *ast.Field) string {
	if len(field.Names) > 0 {
		return field.Names[0].Name
	}
	return "_"
}

func formatParams(params *ast.FieldList) string {
	if params == nil || len(params.List) == 0 {
		return "()"
	}
	return "(...)"
}

func formatResults(results *ast.FieldList) string {
	if results == nil || len(results.List) == 0 {
		return ""
	}
	if len(results.List) == 1 && len(results.List[0].Names) == 0 {
		return "(...)"
	}
	return "(...)"
}

func generateID(filePath string, line int) string {
	data := fmt.Sprintf("%s:%d", filePath, line)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}
