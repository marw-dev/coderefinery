package mocks

import (
	"coderefinery/internal/core/domain"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockEmbedder simuliert Ollama
type MockEmbedder struct {
	mock.Mock
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}

func (m *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	args := m.Called(ctx, texts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([][]float32), args.Error(1)
}

func (m *MockEmbedder) Dimensions() (int, error) {
	args := m.Called()
	if len(args) == 0 {
		return 768, nil
	}
	return args.Int(0), args.Error(1)
}

func (m *MockEmbedder) SetModel(newModel string) error {
	args := m.Called(newModel)
	return args.Error(0)
}

func (m *MockEmbedder) ListModels(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockEmbedder) GetCurrentModel() string {
	args := m.Called()
	if len(args) == 0 {
		return "mock-model"
	}
	return args.String(0)
}

// MockVectorStore simuliert die Vektor-Datenbank (Weaviate)
// Ersetzt MockChunkRepo
type MockVectorStore struct {
	mock.Mock
}

func (m *MockVectorStore) BatchUpsert(ctx context.Context, chunks []domain.CodeChunk) error {
	args := m.Called(ctx, chunks)
	return args.Error(0)
}

func (m *MockVectorStore) SearchSimilar(ctx context.Context, queryVector []float32, limit int, minScore float64, repoIDs []uuid.UUID) ([]domain.SearchResult, error) {
	args := m.Called(ctx, queryVector, limit, minScore, repoIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	// Wichtig: Rückgabetyp ist jetzt []domain.SearchResult
	return args.Get(0).([]domain.SearchResult), args.Error(1)
}

func (m *MockVectorStore) DeleteByRepoID(ctx context.Context, repoID uuid.UUID) error {
	args := m.Called(ctx, repoID)
	return args.Error(0)
}

// MockCache simuliert den HybridCache
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	// Hier simulieren wir das Verhalten, dass dest (Pointer) gefüllt wird
	args := m.Called(ctx, key, dest)

	// Wenn wir Mock-Daten haben, müssen wir sie manuell in 'dest' kopieren oder
	// im Test-Setup beachten, dass args.Get(0) nicht direkt zurückgegeben wird.
	// Für einfache Mocks reicht oft die Rückgabe von found/error.

	return args.Bool(0), args.Error(1)
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}
