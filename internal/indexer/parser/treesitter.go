package parser

import (
	"context"
	"slices"
	"strings"
	"time"

	"coderefinery/internal/domain"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

type TreeSitterParser struct {
	language *sitter.Language
	langName string
}

func NewTreeSitterParser(lang string) *TreeSitterParser {
	var language *sitter.Language

	switch lang {
	case "go":
		language = golang.GetLanguage()
	case "python", "py":
		language = python.GetLanguage()
	case "javascript", "js":
		language = javascript.GetLanguage()
	case "typescript", "ts":
		language = typescript.GetLanguage()
	default:
		return nil
	}

	return &TreeSitterParser{
		language: language,
		langName: lang,
	}
}

func (p *TreeSitterParser) Parse(filePath string, content []byte, modTime time.Time) ([]domain.CodeChunk, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(p.language)

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	var chunks []domain.CodeChunk
	root := tree.RootNode()

	// Extract top-level definitions
	chunks = append(chunks, p.extractDefinitions(root, content, filePath, modTime)...)

	return chunks, nil
}

func (p *TreeSitterParser) extractDefinitions(node *sitter.Node, content []byte, filePath string, modTime time.Time) []domain.CodeChunk {
	var chunks []domain.CodeChunk

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		nodeType := child.Type()

		var chunkType domain.ChunkType
		var signature string

		switch nodeType {
		case "function_declaration", "function_definition", "method_declaration":
			chunkType = domain.ChunkTypeFunction
			signature = p.extractSignature(child, content)
		case "class_declaration", "class_definition":
			chunkType = domain.ChunkTypeClass
			signature = p.extractSignature(child, content)
		case "interface_declaration":
			chunkType = domain.ChunkTypeInterface
			signature = p.extractSignature(child, content)
		default:
			// Recursively process children
			chunks = append(chunks, p.extractDefinitions(child, content, filePath, modTime)...)
			continue
		}

		// Extract content
		startByte := child.StartByte()
		endByte := child.EndByte()
		chunkContent := string(content[startByte:endByte])

		// Extract comments
		comments := p.extractComments(child, content)

		chunk := domain.CodeChunk{
			ID:           generateID(filePath, int(child.StartPoint().Row)),
			FilePath:     filePath,
			Content:      chunkContent,
			Signature:    signature,
			Comments:     comments,
			StartLine:    int(child.StartPoint().Row) + 1,
			EndLine:      int(child.EndPoint().Row) + 1,
			ChunkType:    chunkType,
			Language:     p.langName,
			LastModified: modTime,
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}

func (p *TreeSitterParser) extractSignature(node *sitter.Node, content []byte) string {
	// Try to find identifier/name node
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" || child.Type() == "name" {
			return string(content[node.StartByte():child.EndByte()])
		}
	}

	// Fallback: first line
	lines := strings.Split(string(content[node.StartByte():node.EndByte()]), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

func (p *TreeSitterParser) extractComments(node *sitter.Node, content []byte) string {
	// Look for comment nodes before this node
	parent := node.Parent()
	if parent == nil {
		return ""
	}

	var comments []string
	for i := 0; i < int(parent.ChildCount()); i++ {
		child := parent.Child(i)
		if child == node {
			break
		}
		if strings.Contains(child.Type(), "comment") {
			comments = append(comments, string(content[child.StartByte():child.EndByte()]))
		}
	}

	return strings.Join(comments, "\n")
}

func (p *TreeSitterParser) SupportsLanguage(lang string) bool {
	supported := []string{"go", "python", "py", "javascript", "js", "typescript", "ts"}
	return slices.Contains(supported, lang)
}
