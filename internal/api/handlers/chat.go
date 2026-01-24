package handlers

import (
	"coderefinery/internal/agent"
	"encoding/json"
	"net/http"
)

type ChatHandler struct {
	agent *agent.AgentService
	// db Repository Interface
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

	// Mock Project Config Fetching
	config := agent.PipelineConfig{
		AutoSelectMode: true,
		PlannerModel:   "deepseek-r1",
		CoderModel:     "qwen2.5-coder",
	}

	// Apply Overrides
	if req.PlannerModel != nil { config.PlannerModel = *req.PlannerModel }
	if req.CoderModel != nil { config.CoderModel = *req.CoderModel }
	if req.ForcedMode != nil { config.ForcedMode = req.ForcedMode }

	execution, err := h.agent.Execute(r.Context(), req.ProjectID, req.Message, config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"execution": execution})
}
