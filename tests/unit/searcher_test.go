package unit

import (
	"context"
	"errors"
	"testing"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/search"
	"coderefinery/tests/unit/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSearcher_Search_Success(t *testing.T) {
	// 1. Setup
	// WICHTIG: MockVectorStore statt MockChunkRepo
	mockVectorStore := new(mocks.MockVectorStore)
	mockEmbedder := new(mocks.MockEmbedder)
	mockCache := new(mocks.MockCache)

	// Cache übergeben (jetzt möglich dank Interface)
	searcher := search.NewSearcher(mockVectorStore, mockEmbedder, mockCache)
	ctx := context.Background()

	query := "database connection"
	fakeEmbedding := []float32{0.1, 0.2, 0.3}

	// 2. Erwartungen

	// A: Cache Check (Miss simulieren)
	// Get erwartet (ctx, key, dest) -> return (found, error)
	// Wir nutzen mock.Anything für dest (Pointer)
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(false, nil)

	// B: Embedding
	mockEmbedder.On("Embed", mock.Anything, query).Return(fakeEmbedding, nil)

	// Neue SearchResult Struktur (VectorStore Interface)
	mockResults := []domain.SearchResult{
		{
			Chunk:         domain.CodeChunk{ID: "1", Content: "func ConnectDB() ..."},
			SemanticScore: 0.95,
			CombinedScore: 0.95,
		},
		{
			Chunk:         domain.CodeChunk{ID: "2", Content: "var dbURL string"},
			SemanticScore: 0.80,
			CombinedScore: 0.80,
		},
	}

	// C: Vector Search
	// SearchSimilar(ctx, vector, limit, minScore, repoIDs)
	// RepoIDs ist hier nil oder leer
	mockVectorStore.On("SearchSimilar", mock.Anything, fakeEmbedding, 10, 0.5, mock.Anything).Return(mockResults, nil)

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
	mockVectorStore.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestSearcher_EmbedderFail(t *testing.T) {
	mockVectorStore := new(mocks.MockVectorStore)
	mockEmbedder := new(mocks.MockEmbedder)
	mockCache := new(mocks.MockCache)

	searcher := search.NewSearcher(mockVectorStore, mockEmbedder, mockCache)
	ctx := context.Background()

	// A: Cache Check (Miss)
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(false, nil)

	// B: Embedding Fail
	mockEmbedder.On("Embed", mock.Anything, mock.Anything).Return(nil, errors.New("ollama offline"))

	// Kein VectorSearch oder Cache Set erwartet!

	req := domain.SearchRequest{Query: "test"}
	results, err := searcher.Search(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to embed")
	assert.Nil(t, results)

	mockCache.AssertExpectations(t)
	mockVectorStore.AssertNotCalled(t, "SearchSimilar")
}

func TestSearcher_FilterLowScores(t *testing.T) {
	mockVectorStore := new(mocks.MockVectorStore)
	mockEmbedder := new(mocks.MockEmbedder)
	mockCache := new(mocks.MockCache)

	searcher := search.NewSearcher(mockVectorStore, mockEmbedder, mockCache)
	ctx := context.Background()

	query := "irrelevant query"
	vec := []float32{0.9, 0.9, 0.9}

	// A: Cache Check (Miss)
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(false, nil)

	// B: Embedding
	mockEmbedder.On("Embed", mock.Anything, query).Return(vec, nil)

	// Das Mock liefert direkt die Ergebnisse, Filterung passiert normalerweise in Weaviate (SearchSimilar),
	// aber hier testen wir nur, dass der Searcher das Ergebnis durchreicht.
	// Wenn wir Filterung im Searcher testen wollen, müsste der Mock mehr liefern und der Searcher filtern.
	// Da Searcher aktuell nur durchreicht:
	rawResults := []domain.SearchResult{
		{
			Chunk:         domain.CodeChunk{ID: "good", Content: "pass"},
			SemanticScore: 0.8,
		},
		// (Angenommen Weaviate hat bereits gefiltert basierend auf MinScore)
	}

	req := domain.SearchRequest{
		Query:    query,
		Limit:    5,
		MinScore: 0.7,
	}

	// C: Search
	mockVectorStore.On("SearchSimilar", mock.Anything, vec, 5, 0.7, mock.Anything).Return(rawResults, nil)

	// D: Cache Set
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil)

	results, err := searcher.Search(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, 1)
	assert.Equal(t, "good", results[0].Chunk.ID)

	mockVectorStore.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}
