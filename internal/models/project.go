package models

import (
	"coderefinery/internal/agent"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`

	IndexConfig     IndexConfiguration   `json:"index_config" db:"index_config"`
	DefaultPipeline agent.PipelineConfig `json:"default_pipeline" db:"default_pipeline"`

	TotalExecutions int        `json:"total_executions" db:"total_executions"`
	LastExecutedAt  *time.Time `json:"last_executed_at" db:"last_executed_at"`
}

type IndexConfiguration struct {
	EmbedderModel string    `json:"embedder_model"`
	VectorDB      string    `json:"vector_db"`
	ChunkSize     int       `json:"chunk_size"`
	IndexedAt     time.Time `json:"indexed_at"`
}

func (p *Project) Validate() error {
	if p.Name == "" { return errors.New("name required") }
	if p.IndexConfig.EmbedderModel == "" { return errors.New("embedder required") }
	if p.DefaultPipeline.PlannerModel == "" { return errors.New("planner required") }
	return nil
}
