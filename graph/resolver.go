package graph

import (
	"coderefinery/internal/config"
	"coderefinery/internal/core/ports"
	"coderefinery/internal/core/services"
	"coderefinery/internal/search"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	RepoService ports.RepositoryService
	Searcher    *search.Searcher
	AuthService *services.AuthService
	Embedder    ports.Embedder
	Config      *config.Config
}
