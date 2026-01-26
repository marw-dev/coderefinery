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

// Repository repräsentiert ein Code-Projekt
type Repository struct {
	ID          	uuid.UUID        `json:"id"`
	Name        	string           `json:"name"`
	Path        	string           `json:"path"`
	Status      	RepositoryStatus `json:"status"`
	LastIndexed 	time.Time        `json:"last_indexed"`
	CreatedAt   	time.Time        `json:"created_at"`
	UpdatedAt   	time.Time        `json:"updated_at"`

	// Stats
	FileCount  		int    			 `json:"file_count"`
	ChunkCount 		int    			 `json:"chunk_count"`
	ErrorMsg   		string 			 `json:"error_msg,omitempty"`

	IsManaged  		bool 			 `json:"is_managed"`

	IndexConfig     map[string]any 	`json:"index_config"`
	DefaultPipeline map[string]any 	`json:"default_pipeline"`
	TotalExecutions int            	`json:"total_executions"`
	LastExecutedAt  time.Time      	`json:"last_executed_at"`
}

// NewRepository erstellt eine neue Instanz mit Defaults
func NewRepository(name, path string, isManaged bool) *Repository {
	now := time.Now()
	return &Repository{
		ID:              uuid.New(),
		Name:            name,
		Path:            path,
		IsManaged:       isManaged,
		Status:          StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
		IndexConfig:     make(map[string]any),
		DefaultPipeline: make(map[string]any),
	}
}
