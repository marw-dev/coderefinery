package ports

import (
	"coderefinery/internal/core/domain"
	"context"
	"time"

	"github.com/google/uuid"
)

// RepositoryService (Business Logic Primary Port)
type RepositoryService interface {
	Create(ctx context.Context, name, path string) (*domain.Repository, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Repository, error)
	List(ctx context.Context) ([]*domain.Repository, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Reindex(ctx context.Context, id uuid.UUID) error
    DeleteAllIndices(ctx context.Context) error
}

// RepositoryStore (Secondary Port)
type RepositoryStore interface {
	Save(ctx context.Context, repo *domain.Repository) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Repository, error)
	FindAll(ctx context.Context) ([]*domain.Repository, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Indexer (Secondary Port)
type Indexer interface {
	Index(ctx context.Context, repo *domain.Repository) error
	DeleteIndex(ctx context.Context, repo *domain.Repository) error
    DeleteAllIndices(ctx context.Context) error
}

// AuthService
type AuthService interface {
	Login(ctx context.Context, username, password string) (string, *domain.User, error)
	Register(ctx context.Context, username, password string) (*domain.User, error)
	Me(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

type UserStore interface {
	Save(ctx context.Context, user *domain.User) error
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// ChunkRepository
type ChunkSearchResult struct {
	domain.CodeChunk
	Similarity float64 `db:"cosine_similarity"`
	File       string  `db:"file_path"`
	Language   string  `db:"file_language"`
}

type ChunkRepository interface {
	VectorSearch(ctx context.Context, embedding []float32, limit int, threshold float64) ([]*ChunkSearchResult, error)
}

// Embedder Interface (Secondary Port)
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() (int, error)
    SetModel(newModel string) error
    ListModels(ctx context.Context) ([]string, error)
    GetCurrentModel() string
}

// Cache Interface (Secondary Port)
type Cache interface {
    Get(ctx context.Context, key string, dest any) (bool, error)
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
}
