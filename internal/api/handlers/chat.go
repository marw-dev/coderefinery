package handlers

import (
	"coderefinery/internal/agent"
	"coderefinery/internal/config"
	"encoding/json"
	"net/http"
)

type ChatHandler struct {
	agent  *agent.AgentService
	config *config.Config
}

func NewChatHandler(agent *agent.AgentService, config *config.Config) *ChatHandler {
	return &ChatHandler{
		agent:  agent,
		config: config,
	}
}

type ChatRequest struct {
	ProjectID string `json:"project_id"`
	Message   string `json:"message"`

	PlannerModel     *string              `json:"planner_model,omitempty"`
	CoderModel       *string              `json:"coder_model,omitempty"`
	ForcedMode       *agent.ExecutionMode `json:"forced_mode,omitempty"`
	EnableValidation *bool                `json:"enable_validation,omitempty"`
}

func (h *ChatHandler) Chat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plannerModel := h.config.LLM.Agent.PlannerModel
	if plannerModel == "" {
		plannerModel = "deepseek-r1:14b"
	}

	coderModel := h.config.LLM.Agent.CoderModel
	if coderModel == "" {
		coderModel = "qwen2.5-coder:14b"
	}

	fallbackModel := h.config.LLM.Agent.FallbackModel
	if fallbackModel == "" {
		fallbackModel = "qwen2.5-coder:14b"
	}

	pipelineConfig := agent.PipelineConfig{
		AutoSelectMode:   true,
		PlannerModel:     plannerModel,
		CoderModel:       coderModel,
		FallbackModel:    fallbackModel,
		EmbedderModel:    h.config.LLM.EmbeddingModel,
		EnableValidation: true,
	}

	if req.PlannerModel != nil {
		pipelineConfig.PlannerModel = *req.PlannerModel
	}
	if req.CoderModel != nil {
		pipelineConfig.CoderModel = *req.CoderModel
	}
	if req.ForcedMode != nil {
		pipelineConfig.ForcedMode = req.ForcedMode
	}
	if req.EnableValidation != nil {
		pipelineConfig.EnableValidation = *req.EnableValidation
	}

	execution, err := h.agent.Execute(r.Context(), req.ProjectID, req.Message, pipelineConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// Error ignoriert
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"execution": execution})
}
