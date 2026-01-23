package mocks

import (
	"coderefinery/internal/core/ports"
	"context"
	"time"

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

// MockChunkRepo simuliert die Vektor-Datenbank
type MockChunkRepo struct {
	mock.Mock
}

func (m *MockChunkRepo) VectorSearch(ctx context.Context, embedding []float32, limit int, threshold float64) ([]*ports.ChunkSearchResult, error) {
	args := m.Called(ctx, embedding, limit, threshold)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ports.ChunkSearchResult), args.Error(1)
}


type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	args := m.Called(ctx, key, dest)

	return args.Bool(0), args.Error(1)
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}
