package domain

import (
	"time"

	"github.com/google/uuid"
)

type RepositoryStatus string

const (
	StatusPending  RepositoryStatus = "pending"
	StatusIndexing RepositoryStatus = "indexing"
	StatusReady    RepositoryStatus = "ready"
	StatusError    RepositoryStatus = "error"
)

// Repository repräsentiert ein Code-Projekt, das wir überwachen
type Repository struct {
	ID          uuid.UUID        `json:"id"`
	Name        string           `json:"name"`
	Path        string           `json:"path"`
	Status      RepositoryStatus `json:"status"`
	LastIndexed time.Time        `json:"last_indexed"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`

	// Stats für das Frontend
	FileCount   int              `json:"file_count"`
	ChunkCount  int              `json:"chunk_count"`
	ErrorMsg    string           `json:"error_msg,omitempty"`
}

// NewRepository erstellt eine neue Instanz mit Defaults
func NewRepository(name, path string) *Repository {
	now := time.Now()
	return &Repository{
		ID:        uuid.New(),
		Name:      name,
		Path:      path,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
