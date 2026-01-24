package integration

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"coderefinery/internal/adapters/vectordb"
	"coderefinery/internal/config"
	"coderefinery/internal/core/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper: Creates random []float32 with normalized values (mostly)
func randomEmbedding() []float32 {
	vec := make([]float32, 768)
	for i := range vec {
		vec[i] = rand.Float32()
	}
	return vec
}

func TestVectorSearch_Integration(t *testing.T) {
	// 1. Setup: Verbindung zu Weaviate herstellen
	// Wir nutzen hier Hardcoded Ports passend zur docker-compose Umgebung
	cfg := config.VectorDBConfig{
		Host:      "localhost:8090", // Port aus docker-compose für Weaviate
		Scheme:    "http",
		IndexName: "TestCodeChunk", // Eigener Index-Name für Tests, um Prod-Daten nicht zu stören
		Timeout:   5 * time.Second,
	}

	store, err := vectordb.NewWeaviateVectorStore(cfg)
	require.NoError(t, err, "Failed to connect to Weaviate. Is docker-compose running?")

	ctx := context.Background()
	repoID := uuid.New()
	otherRepoID := uuid.New()

	// Cleanup vor dem Test (falls alte Daten existieren) und danach
	_ = store.DeleteByRepoID(ctx, repoID)
	_ = store.DeleteByRepoID(ctx, otherRepoID)
	defer func() {
		_ = store.DeleteByRepoID(ctx, repoID)
		_ = store.DeleteByRepoID(ctx, otherRepoID)
	}()

	// 2. Testdaten vorbereiten
	targetVec := randomEmbedding()

	chunk1 := domain.CodeChunk{
		ID:        uuid.New().String(),
		RepoID:    repoID, // Unser Ziel-Repo
		FilePath:  "main.go",
		Content:   "func TargetFunction() { fmt.Println('Hello') }",
		StartLine: 10,
		EndLine:   12,
		ChunkType: domain.ChunkTypeFunction,
		Language:  "go",
		Embedding: targetVec,
	}

	// Ein Chunk aus einem anderen Repo (sollte später gefiltert werden)
	chunk2 := domain.CodeChunk{
		ID:        uuid.New().String(),
		RepoID:    otherRepoID, // Anderes Repo
		FilePath:  "other.go",
		Content:   "func OtherFunction() {}",
		Embedding: targetVec, // Gleicher Vektor! Würde ohne Filter gefunden werden.
	}

	// 3. Batch Upsert (Indexing)
	err = store.BatchUpsert(ctx, []domain.CodeChunk{chunk1, chunk2})
	require.NoError(t, err, "BatchUpsert failed")

	// WICHTIG: Weaviate ist "eventually consistent".
	// In Integrationstests müssen wir kurz warten, bis der Index aktualisiert ist.
	time.Sleep(1500 * time.Millisecond)

	// 4. Test Case A: Suche OHNE Repo-Filter (Global Admin Search)
	// Sollte beide finden, da Vektoren identisch sind
	results, err := store.SearchSimilar(ctx, targetVec, 10, 0.5, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2, "Should find vectors from both repos without filter")

	// 5. Test Case B: Suche MIT Repo-Filter
	// Sollte NUR chunk1 finden
	resultsFiltered, err := store.SearchSimilar(ctx, targetVec, 10, 0.5, []uuid.UUID{repoID})
	require.NoError(t, err)

	assert.NotEmpty(t, resultsFiltered)
	if len(resultsFiltered) > 0 {
		assert.Equal(t, chunk1.Content, resultsFiltered[0].Chunk.Content)
		assert.Equal(t, repoID, resultsFiltered[0].Chunk.RepoID)
		// Sicherstellen, dass KEIN Ergebnis aus dem anderen Repo dabei ist
		for _, res := range resultsFiltered {
			assert.NotEqual(t, otherRepoID, res.Chunk.RepoID, "Leak: Found chunk from wrong repo")
		}
	}

	// 6. Test Case C: Score Prüfung
	// Da wir exakt denselben Vektor suchen, sollte Score sehr hoch sein (nahe 1.0)
	assert.Greater(t, resultsFiltered[0].SemanticScore, 0.9, "Exact vector match should have high score")

	// 7. Cleanup Test (DeleteByRepoID)
	err = store.DeleteByRepoID(ctx, repoID)
	require.NoError(t, err)

	// Kurz warten für Weaviate Delete
	time.Sleep(1000 * time.Millisecond)

	// Prüfen ob es wirklich weg ist
	resultsAfterDelete, err := store.SearchSimilar(ctx, targetVec, 10, 0.1, []uuid.UUID{repoID})
	require.NoError(t, err)
	assert.Empty(t, resultsAfterDelete, "Result should be empty after deletion")
}
