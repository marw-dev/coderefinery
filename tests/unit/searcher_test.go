package unit

import (
	"context"
	"errors"
	"testing"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"
	"coderefinery/internal/search"
	"coderefinery/tests/unit/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSearcher_Search_Success(t *testing.T) {
	// 1. Setup
	mockRepo := new(mocks.MockChunkRepo)
	mockEmbedder := new(mocks.MockEmbedder)
	mockCache := new(mocks.MockCache) // NEU

	// Cache übergeben
	searcher := search.NewSearcher(mockRepo, mockEmbedder, mockCache)
	ctx := context.Background()

	query := "database connection"
	fakeEmbedding := []float32{0.1, 0.2, 0.3}

	// 2. Erwartungen

	// A: Cache Check (Miss simulieren)
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(false, nil)

	// B: Embedding
	mockEmbedder.On("Embed", mock.Anything, query).Return(fakeEmbedding, nil)

	mockResults := []*ports.ChunkSearchResult{
		{
			CodeChunk:  domain.CodeChunk{ID: "1", Content: "func ConnectDB() ..."},
			Similarity: 0.95,
		},
		{
			CodeChunk:  domain.CodeChunk{ID: "2", Content: "var dbURL string"},
			Similarity: 0.80,
		},
	}

	// C: Vector Search
	mockRepo.On("VectorSearch", mock.Anything, fakeEmbedding, 10, 0.5).Return(mockResults, nil)

	// D: Cache Set (Ergebnis speichern)
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil)

	// 3. Ausführen
	req := domain.SearchRequest{
		Query:    query,
		Limit:    10,
		MinScore: 0.5,
	}
	results, err := searcher.Search(ctx, req)

	// 4. Assertions
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "func ConnectDB() ...", results[0].Chunk.Content)
	assert.Equal(t, 0.95, results[0].SemanticScore)

	mockEmbedder.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestSearcher_EmbedderFail(t *testing.T) {
	mockRepo := new(mocks.MockChunkRepo)
	mockEmbedder := new(mocks.MockEmbedder)
	mockCache := new(mocks.MockCache)

	searcher := search.NewSearcher(mockRepo, mockEmbedder, mockCache)
	ctx := context.Background()

	// A: Cache Check (Miss)
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(false, nil)

	// B: Embedding Fail
	mockEmbedder.On("Embed", mock.Anything, mock.Anything).Return(nil, errors.New("ollama offline"))

	// Kein Cache Set erwartet bei Fehler!

	req := domain.SearchRequest{Query: "test"}
	results, err := searcher.Search(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to embed")
	assert.Nil(t, results)

	mockCache.AssertExpectations(t)
}

func TestSearcher_FilterLowScores(t *testing.T) {
	mockRepo := new(mocks.MockChunkRepo)
	mockEmbedder := new(mocks.MockEmbedder)
	mockCache := new(mocks.MockCache)

	searcher := search.NewSearcher(mockRepo, mockEmbedder, mockCache)
	ctx := context.Background()

	query := "irrelevant query"
	vec := []float32{0.9, 0.9, 0.9}

	// A: Cache Check (Miss)
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(false, nil)

	// B: Embedding
	mockEmbedder.On("Embed", mock.Anything, query).Return(vec, nil)

	rawResults := []*ports.ChunkSearchResult{
		{
			CodeChunk:  domain.CodeChunk{ID: "good", Content: "pass"},
			Similarity: 0.8,
		},
		{
			CodeChunk:  domain.CodeChunk{ID: "bad", Content: "fail"},
			Similarity: 0.3,
		},
	}

	req := domain.SearchRequest{
		Query:    query,
		Limit:    5,
		MinScore: 0.7,
	}

	// C: Search
	mockRepo.On("VectorSearch", mock.Anything, vec, 5, 0.7).Return(rawResults, nil)

	// D: Cache Set
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil)

	results, err := searcher.Search(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, 2)
	assert.Equal(t, "good", results[0].Chunk.ID)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}
