package models

import "time"

type ModelRole string

const (
	RoleEmbedder   ModelRole = "embedder"
	RolePlanner    ModelRole = "planner"
	RoleCoder      ModelRole = "coder"
	RoleGeneralist ModelRole = "generalist"
)

type ModelCapability struct {
	Name          string    `json:"name"`
	Role          ModelRole `json:"role"`
	ParameterSize string    `json:"parameter_size"` // "7B", "14B", "70B"
	ContextWindow int       `json:"context_window"`
	Family        string    `json:"family"`  // "llama", "qwen", "deepseek"
	Version       string    `json:"version"` // "v1.5", "v2.0"
	Tags          []string  `json:"tags"`

	// Performance-Metriken
	AvgLatencyMs *float64 `json:"avg_latency_ms,omitempty"`
	SuccessRate  *float64 `json:"success_rate,omitempty"`
}

type ModelClassification struct {
	Embedders   []ModelCapability `json:"embedders"`
	Planners    []ModelCapability `json:"planners"`
	Coders      []ModelCapability `json:"coders"`
	Generalists []ModelCapability `json:"generalists"`

	Recommendations ModelRecommendations `json:"recommendations"`
	ScannedAt       time.Time            `json:"scanned_at"`
}

type ModelRecommendations struct {
	Embedder ModelRecommendation `json:"embedder"`
	Planner  ModelRecommendation `json:"planner"`
	Coder    ModelRecommendation `json:"coder"`
}

type ModelRecommendation struct {
	ModelName string   `json:"model_name"`
	Score     float64  `json:"score"`
	Reasons   []string `json:"reasons"`
}
