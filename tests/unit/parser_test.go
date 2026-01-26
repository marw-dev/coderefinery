package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"coderefinery/internal/adapters/indexer/parser"
	"coderefinery/internal/core/domain" // Import hinzugefügt

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoParser_Parse(t *testing.T) {
	// Setup File
	content := `
	package main

	// Hello function
	func Hello() string {
		return "world"
	}

	type MyStruct struct {
		Field int
	}
	`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "main.go")
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	// Execute
	p := parser.GetParser("go")
	chunks, err := p.Parse(path, []byte(content), time.Now())

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, chunks)

	// Check Function Chunk
	funcChunk := findChunk(chunks, "Hello")
	require.NotNil(t, funcChunk)
	assert.Contains(t, funcChunk.Content, "return \"world\"")
	assert.Equal(t, "function", string(funcChunk.ChunkType))

	// Check Struct Chunk
	structChunk := findChunk(chunks, "MyStruct")
	require.NotNil(t, structChunk)
	assert.Equal(t, "struct", string(structChunk.ChunkType))
}

// Helper angepasst auf domain.CodeChunk
func findChunk(chunks []domain.CodeChunk, namePart string) *domain.CodeChunk {
	for _, c := range chunks {
		if strings.Contains(c.Signature, namePart) || strings.Contains(c.Content, namePart) {
			return &c
		}
	}
	return nil
}
