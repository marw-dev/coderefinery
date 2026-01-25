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

// NewChatHandler erstellt den Handler mit den Abhängigkeiten
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

	// 1. Defaults aus der globalen Config laden
	// Falls in der Config nichts steht (z.B. alte Config), setzen wir Fallbacks.
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

	// 2. Pipeline Config erstellen
	pipelineConfig := agent.PipelineConfig{
		AutoSelectMode:   true, // Standardmäßig an, könnte man auch in Config auslagern
		PlannerModel:     plannerModel,
		CoderModel:       coderModel,
		FallbackModel:    fallbackModel,
		EmbedderModel:    h.config.LLM.EmbeddingModel, // Wichtig: Auch das Embedding Model aus Config nutzen
		EnableValidation: true,                        // Standardmäßig an
	}

	// 3. Overrides aus dem Request anwenden (falls der User im Frontend etwas anderes wählt)
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

	// 4. Ausführen
	execution, err := h.agent.Execute(r.Context(), req.ProjectID, req.Message, pipelineConfig)
	if err != nil {
		// Fehler loggen wäre hier gut
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"execution": execution})
}
