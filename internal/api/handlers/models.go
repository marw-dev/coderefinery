package handlers

import (
	"coderefinery/internal/infrastructure/llm"
	"encoding/json"
	"net/http"
)

type ModelsHandler struct {
	provider *llm.OllamaProvider
}

func NewModelsHandler(provider *llm.OllamaProvider) *ModelsHandler {
	return &ModelsHandler{provider: provider}
}

func (h *ModelsHandler) DiscoverModels(w http.ResponseWriter, r *http.Request) {
	classification, err := h.provider.DiscoverAndClassifyModels(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(classification)
}
