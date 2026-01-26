package handlers

import (
	"coderefinery/internal/agent"
	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ProjectsHandler struct {
	repo ports.RepositoryStore
}

func (h *ProjectsHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		EmbedderModel string `json:"embedder_model"`
		PlannerModel  string `json:"planner_model"`
		CoderModel    string `json:"coder_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	project := &domain.Repository{
		ID:        uuid.New(),
		Name:      req.Name,
		CreatedAt: time.Now(),
	}

	if err := h.repo.Save(r.Context(), project); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	// Error ignoriert
	_ = json.NewEncoder(w).Encode(project)
}

func (h *ProjectsHandler) UpdatePipeline(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := uuid.Parse(idStr)

	var updates agent.PipelineConfig
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repo, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	if err := h.repo.Save(r.Context(), repo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
