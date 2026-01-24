package agent

import (
	"time"

	"github.com/google/uuid"
)

type ExecutionMode string

const (
	ModeFastPath ExecutionMode = "fast_path"
	ModeHybrid   ExecutionMode = "hybrid"
	ModeFull     ExecutionMode = "full"
)

type PipelineConfig struct {
	EmbedderModel string `json:"embedder_model"`
	PlannerModel  string `json:"planner_model"`
	CoderModel    string `json:"coder_model"`
	FallbackModel string `json:"fallback_model,omitempty"`

	AutoSelectMode    bool           `json:"auto_select_mode"`
	ForcedMode        *ExecutionMode `json:"forced_mode,omitempty"`
	EnableRetry       bool           `json:"enable_retry"`
	EnableValidation  bool           `json:"enable_validation"`
}

type PipelineStage string

const (
	StageComplexityAnalysis PipelineStage = "complexity_analysis"
	StageRetrieval          PipelineStage = "retrieval"
	StagePlanning           PipelineStage = "planning"
	StageCoding             PipelineStage = "coding"
	StageValidation         PipelineStage = "validation"
)

type StageResult struct {
	Stage      PipelineStage `json:"stage"`
	ModelUsed  string        `json:"model_used,omitempty"`
	Output     string        `json:"output"`
	TokensUsed int           `json:"tokens_used"`
	Duration   time.Duration `json:"duration"`
	Success    bool          `json:"success"`
	Error      string        `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type PipelineExecution struct {
	ID              uuid.UUID       `json:"id"`
	ProjectID       string          `json:"project_id"`
	Mode            ExecutionMode   `json:"mode"`
	Complexity      QueryComplexity `json:"complexity"`
	Stages          []StageResult   `json:"stages"`
	FinalOutput     string          `json:"final_output"`
	TotalDuration   time.Duration   `json:"total_duration"`
	Success         bool            `json:"success"`
	CreatedAt       time.Time       `json:"created_at"`
}

type CodeChunk struct {
	Content   string  `json:"content"`
	FilePath  string  `json:"file_path"`
	Score     float64 `json:"score"`
	LineStart int     `json:"line_start"`
	LineEnd   int     `json:"line_end"`
}
