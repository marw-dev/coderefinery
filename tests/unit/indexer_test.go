package unit

import (
	"os"
	"path/filepath"
	"testing"

	"coderefinery/internal/adapters/indexer"
	"coderefinery/internal/config"
	"coderefinery/tests/unit/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexer_GitIgnore(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "indexer_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 2. Create structure
	// .gitignore
	// main.go
	// secret.key
	// vendor/lib.go

	err = os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("secret.key\nvendor/"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "secret.key"), []byte("secret"), 0644)
	require.NoError(t, err)

	err = os.Mkdir(filepath.Join(tmpDir, "vendor"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "vendor/lib.go"), []byte("package lib"), 0644)
	require.NoError(t, err)

	// 3. Init Indexer
	// Wir benötigen nun Embedder und VectorStore Mocks statt sqlx.DB
	mockEmbedder := new(mocks.MockEmbedder)
	mockVectorStore := new(mocks.MockVectorStore)

	cfg := config.IndexerConfig{
		SupportedExts: map[string]string{"go": "go"},
		ExcludePaths:  []string{".git"},
	}

	// Hier übergeben wir jetzt die Mocks
	idx, err := indexer.NewIndexer(cfg, mockEmbedder, mockVectorStore)

	assert.NoError(t, err)
	assert.NotNil(t, idx)
}
