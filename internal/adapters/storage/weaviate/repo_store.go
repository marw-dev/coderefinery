package weaviate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"

	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

const ClassRepository = "Repository"

// ErrRepositoryNotFound wird zurückgegeben wenn ein Repository nicht gefunden wurde
var ErrRepositoryNotFound = fmt.Errorf("repository not found")

type RepoStore struct {
	client *weaviate.Client
}

func NewRepoStore(client *weaviate.Client) (ports.RepositoryStore, error) {
	store := &RepoStore{client: client}
	if err := store.ensureSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure schema: %w", err)
	}
	return store, nil
}

func (s *RepoStore) ensureSchema(ctx context.Context) error {
	exists, err := s.client.Schema().ClassExistenceChecker().
		WithClassName(ClassRepository).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check schema existence: %w", err)
	}

	if exists {
		return nil
	}

	classObj := &models.Class{
		Class: ClassRepository,
		Properties: []*models.Property{
			{Name: "name", DataType: []string{"string"}},
			{Name: "path", DataType: []string{"string"}},
			{Name: "status", DataType: []string{"string"}},
			{Name: "is_managed", DataType: []string{"boolean"}},
			{Name: "created_at", DataType: []string{"date"}},
			{Name: "updated_at", DataType: []string{"date"}},
			{Name: "last_indexed", DataType: []string{"date"}},
			{Name: "file_count", DataType: []string{"int"}},
			{Name: "chunk_count", DataType: []string{"int"}},
			{Name: "error_msg", DataType: []string{"text"}},
			{Name: "index_config", DataType: []string{"text"}},
			{Name: "default_pipeline", DataType: []string{"text"}},
			{Name: "total_executions", DataType: []string{"int"}},
			{Name: "last_executed_at", DataType: []string{"date"}},
		},
	}

	if err := s.client.Schema().ClassCreator().WithClass(classObj).Do(ctx); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Save erstellt oder aktualisiert ein Repository
func (s *RepoStore) Save(ctx context.Context, repo *domain.Repository) error {
	if repo.ID == uuid.Nil {
		repo.ID = uuid.New()
	}

	repo.UpdatedAt = time.Now()
	properties := s.repoToProperties(repo)

	_, err := s.client.Data().Creator().
		WithClassName(ClassRepository).
		WithID(repo.ID.String()).
		WithProperties(properties).
		Do(ctx)

	if err == nil {
		return nil
	}

	if isConflictError(err) {
		return s.client.Data().Updater().
			WithClassName(ClassRepository).
			WithID(repo.ID.String()).
			WithProperties(properties).
			Do(ctx)
	}

	return fmt.Errorf("failed to save repository: %w", err)
}

// FindByID lädt ein Repository nach ID
func (s *RepoStore) FindByID(ctx context.Context, id uuid.UUID) (*domain.Repository, error) {
	objects, err := s.client.Data().ObjectsGetter().
		WithClassName(ClassRepository).
		WithID(id.String()).
		Do(ctx)

	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrRepositoryNotFound
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	if len(objects) == 0 {
		return nil, ErrRepositoryNotFound
	}

	return s.mapObjectToRepo(objects[0])
}

// FindAll lädt alle Repositories
func (s *RepoStore) FindAll(ctx context.Context) ([]*domain.Repository, error) {
	fields := []graphql.Field{
		{Name: "name"},
		{Name: "path"},
		{Name: "status"},
		{Name: "is_managed"},
		{Name: "created_at"},
		{Name: "updated_at"},
		{Name: "last_indexed"},
		{Name: "file_count"},
		{Name: "chunk_count"},
		{Name: "error_msg"},
		{Name: "index_config"},
		{Name: "default_pipeline"},
		{Name: "total_executions"},
		{Name: "last_executed_at"},
		{Name: "_additional", Fields: []graphql.Field{{Name: "id"}}},
	}

	resp, err := s.client.GraphQL().Get().
		WithClassName(ClassRepository).
		WithFields(fields...).
		WithLimit(1000).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to query repositories: %w", err)
	}

	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("graphql errors: %v", resp.Errors)
	}

	return s.parseGraphQLResult(resp)
}

// Delete löscht ein Repository
func (s *RepoStore) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.client.Data().Deleter().
		WithClassName(ClassRepository).
		WithID(id.String()).
		Do(ctx)

	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	return nil
}

// repoToProperties konvertiert Repository zu Weaviate Properties
func (s *RepoStore) repoToProperties(repo *domain.Repository) map[string]any {
	properties := map[string]any{
		"name":             repo.Name,
		"path":             repo.Path,
		"status":           string(repo.Status),
		"is_managed":       repo.IsManaged,
		"created_at":       repo.CreatedAt,
		"updated_at":       repo.UpdatedAt,
		"file_count":       repo.FileCount,
		"chunk_count":      repo.ChunkCount,
		"total_executions": repo.TotalExecutions,
	}

	if !repo.LastIndexed.IsZero() {
		properties["last_indexed"] = repo.LastIndexed
	}

	if !repo.LastExecutedAt.IsZero() {
		properties["last_executed_at"] = repo.LastExecutedAt
	}

	if repo.ErrorMsg != "" {
		properties["error_msg"] = repo.ErrorMsg
	}

	if len(repo.IndexConfig) > 0 {
		if jsonData, err := json.Marshal(repo.IndexConfig); err == nil {
			properties["index_config"] = string(jsonData)
		}
	}

	if len(repo.DefaultPipeline) > 0 {
		if jsonData, err := json.Marshal(repo.DefaultPipeline); err == nil {
			properties["default_pipeline"] = string(jsonData)
		}
	}

	return properties
}

// mapObjectToRepo konvertiert Weaviate Object zu Repository
func (s *RepoStore) mapObjectToRepo(obj *models.Object) (*domain.Repository, error) {
	props, ok := obj.Properties.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid properties type")
	}

	id, err := uuid.Parse(obj.ID.String())
	if err != nil {
		return nil, fmt.Errorf("invalid repository id: %w", err)
	}

	repo := &domain.Repository{
		ID:              id,
		IndexConfig:     make(map[string]any),
		DefaultPipeline: make(map[string]any),
	}

	if v, ok := props["name"].(string); ok {
		repo.Name = v
	}
	if v, ok := props["path"].(string); ok {
		repo.Path = v
	}
	if v, ok := props["status"].(string); ok {
		repo.Status = domain.RepositoryStatus(v)
	}
	if v, ok := props["error_msg"].(string); ok {
		repo.ErrorMsg = v
	}

	if v, ok := props["is_managed"].(bool); ok {
		repo.IsManaged = v
	}

	if v, ok := props["file_count"].(float64); ok {
		repo.FileCount = int(v)
	}
	if v, ok := props["chunk_count"].(float64); ok {
		repo.ChunkCount = int(v)
	}
	if v, ok := props["total_executions"].(float64); ok {
		repo.TotalExecutions = int(v)
	}

	if v, ok := props["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			repo.CreatedAt = t
		}
	}
	if v, ok := props["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			repo.UpdatedAt = t
		}
	}
	if v, ok := props["last_indexed"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			repo.LastIndexed = t
		}
	}
	if v, ok := props["last_executed_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			repo.LastExecutedAt = t
		}
	}

	if v, ok := props["index_config"].(string); ok && v != "" {
		_ = json.Unmarshal([]byte(v), &repo.IndexConfig)
	}
	if v, ok := props["default_pipeline"].(string); ok && v != "" {
		_ = json.Unmarshal([]byte(v), &repo.DefaultPipeline)
	}

	return repo, nil
}

// parseGraphQLResult parsed die GraphQL Response
func (s *RepoStore) parseGraphQLResult(resp *models.GraphQLResponse) ([]*domain.Repository, error) {
	data, ok := resp.Data["Get"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid response structure: missing 'Get'")
	}

	objects, ok := data[ClassRepository].([]any)
	if !ok {
		return []*domain.Repository{}, nil
	}

	repos := make([]*domain.Repository, 0, len(objects))

	for _, obj := range objects {
		pmap, ok := obj.(map[string]any)
		if !ok {
			continue
		}

		repo, err := s.parseGraphQLObject(pmap)
		if err != nil {
			continue
		}

		repos = append(repos, repo)
	}

	return repos, nil
}

// parseGraphQLObject parsed ein einzelnes GraphQL Objekt
func (s *RepoStore) parseGraphQLObject(pmap map[string]any) (*domain.Repository, error) {
	var id uuid.UUID
	if add, ok := pmap["_additional"].(map[string]any); ok {
		if idStr, ok := add["id"].(string); ok {
			var err error
			id, err = uuid.Parse(idStr)
			if err != nil {
				return nil, fmt.Errorf("invalid id in response: %w", err)
			}
		}
	}

	if id == uuid.Nil {
		return nil, fmt.Errorf("missing id in response")
	}

	repo := &domain.Repository{
		ID:              id,
		IndexConfig:     make(map[string]any),
		DefaultPipeline: make(map[string]any),
	}

	if v, ok := pmap["name"].(string); ok {
		repo.Name = v
	}
	if v, ok := pmap["path"].(string); ok {
		repo.Path = v
	}
	if v, ok := pmap["status"].(string); ok {
		repo.Status = domain.RepositoryStatus(v)
	}
	if v, ok := pmap["error_msg"].(string); ok {
		repo.ErrorMsg = v
	}

	if v, ok := pmap["is_managed"].(bool); ok {
		repo.IsManaged = v
	}

	if v, ok := pmap["file_count"].(float64); ok {
		repo.FileCount = int(v)
	}
	if v, ok := pmap["chunk_count"].(float64); ok {
		repo.ChunkCount = int(v)
	}
	if v, ok := pmap["total_executions"].(float64); ok {
		repo.TotalExecutions = int(v)
	}

	if v, ok := pmap["created_at"].(string); ok {
		repo.CreatedAt, _ = time.Parse(time.RFC3339, v)
	}
	if v, ok := pmap["updated_at"].(string); ok {
		repo.UpdatedAt, _ = time.Parse(time.RFC3339, v)
	}
	if v, ok := pmap["last_indexed"].(string); ok {
		repo.LastIndexed, _ = time.Parse(time.RFC3339, v)
	}
	if v, ok := pmap["last_executed_at"].(string); ok {
		repo.LastExecutedAt, _ = time.Parse(time.RFC3339, v)
	}

	if v, ok := pmap["index_config"].(string); ok && v != "" {
		_ = json.Unmarshal([]byte(v), &repo.IndexConfig)
	}
	if v, ok := pmap["default_pipeline"].(string); ok && v != "" {
		_ = json.Unmarshal([]byte(v), &repo.DefaultPipeline)
	}

	return repo, nil
}

func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "already exists") ||
		strings.Contains(errStr, "status code: 422") ||
		strings.Contains(errStr, "status code: 409")
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "status code: 404")
}
