package integration

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"coderefinery/internal/adapters/repository/postgres"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper: Erstellt einen zufälligen Vektor der Länge 768
func randomEmbedding() []float32 {
	vec := make([]float32, 768)
	for i := range vec {
		vec[i] = rand.Float32()
	}
	return vec
}

// Formatiert []float32 in einen String "[0.1,0.2,...]" für pgvector
func formatVector(vec []float32) string {
	var b strings.Builder
	b.WriteString("[")
	for i, v := range vec {
		if i > 0 {
			b.WriteString(",")
		}
		// %f reicht meist, %g ist kompakter
		fmt.Fprintf(&b, "%f", v)
	}
	b.WriteString("]")
	return b.String()
}

func TestVectorSearch_Integration(t *testing.T) {
	cleanup()
	ctx := context.Background()

	// 1. IDs
	projectID := uuid.New()
	userID := uuid.New()
	fileID := uuid.New()

	// 2. User anlegen
	_, err := testDB.ExecContext(ctx,
		"INSERT INTO users (id, username, password_hash) VALUES ($1, 'vectoruser', 'x')",
		userID)
	require.NoError(t, err, "User insert failed")

	// 3. Projekt anlegen
	_, err = testDB.ExecContext(ctx,
		"INSERT INTO projects (id, name, owner_id) VALUES ($1, 'VectorTest', $2)",
		projectID, userID)
	require.NoError(t, err, "Project insert failed")

	// 4. Datei anlegen
	_, err = testDB.ExecContext(ctx,
		"INSERT INTO files (id, project_id, path) VALUES ($1, $2, 'main.go')",
		fileID, projectID)
	require.NoError(t, err, "File insert failed")

	// 5. Chunk Repository
	chunkRepo := postgres.NewChunkRepository(testDB)

	// 6. Chunk mit Embedding speichern
	targetVec := randomEmbedding()
	targetVecStr := formatVector(targetVec)
	chunkID := uuid.New()

	_, err = testDB.ExecContext(ctx, `
		INSERT INTO code_chunks (id, file_id, content, start_line, end_line, chunk_type, embedding)
		VALUES ($1, $2, 'func Target() {}', 1, 10, 'function', $3)
	`, chunkID, fileID, targetVecStr)
	require.NoError(t, err, "Chunk insert failed")

	// 7. Suche durchführen
	results, err := chunkRepo.VectorSearch(ctx, targetVec, 5, 0.8)
	require.NoError(t, err, "Vector search failed")

	// 8. Assertions
	assert.NotEmpty(t, results)
	if len(results) > 0 {
		assert.Equal(t, chunkID.String(), results[0].ID)
		assert.InDelta(t, 1.0, results[0].Similarity, 0.0001)
	}
}
