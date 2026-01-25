package ports

import (
	"context"
	"time"

	"coderefinery/internal/core/domain"

	"github.com/google/uuid"
)

// RepositoryService ist der "Driver Port" (Eingang)
type RepositoryService interface {
	Create(ctx context.Context, name, path string, isManaged bool) (*domain.Repository, error)
	Get(ctx context.Context, id uuid.UUID) (*domain.Repository, error)
	List(ctx context.Context) ([]*domain.Repository, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Reindex(ctx context.Context, id uuid.UUID) error
	DeleteAllIndices(ctx context.Context) error
}

// RepositoryStore verwaltet Repo-Metadaten in SQL
type RepositoryStore interface {
	Save(ctx context.Context, repo *domain.Repository) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Repository, error)
	FindAll(ctx context.Context) ([]*domain.Repository, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// UserStore für Authentifizierung (SQL)
type UserStore interface {
	Save(ctx context.Context, user *domain.User) error
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type Cache interface {
	Get(ctx context.Context, key string, dest any) (bool, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
}

// VectorStore ist die Schnittstelle zu Weaviate
type VectorStore interface {
	BatchUpsert(ctx context.Context, chunks []domain.CodeChunk) error
	SearchSimilar(ctx context.Context, queryVector []float32, limit int, minScore float64, repoIDs []uuid.UUID) ([]domain.SearchResult, error)
	DeleteByRepoID(ctx context.Context, repoID uuid.UUID) error
}

// Indexer Interface
type Indexer interface {
	Index(ctx context.Context, repo *domain.Repository) error
	DeleteIndex(ctx context.Context, repo *domain.Repository) error
	DeleteAllIndices(ctx context.Context) error
}

// Embedder Interface
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() (int, error)

	// LLM Management
	ListModels(ctx context.Context) ([]string, error)
	GetCurrentModel() string
	SetModel(model string) error
}
