package vectordb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

const (
	// DefaultBatchSize ist die Standard Batch-Größe für Bulk Operations
	DefaultBatchSize = 100
	// MaxRetries für failed batch items
	MaxRetries = 3
)

type WeaviateVectorStore struct {
	client    *weaviate.Client
	indexName string
	timeout   time.Duration
}

// NewWeaviateVectorStore erstellt eine neue Instanz und initialisiert das Schema
func NewWeaviateVectorStore(client *weaviate.Client, indexName string) (ports.VectorStore, error) {
	if client == nil {
		return nil, fmt.Errorf("weaviate client cannot be nil")
	}
	if indexName == "" {
		return nil, fmt.Errorf("index name cannot be empty")
	}

	store := &WeaviateVectorStore{
		client:    client,
		indexName: indexName,
		timeout:   30 * time.Second,
	}

	if err := store.ensureSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure schema: %w", err)
	}

	return store, nil
}

// ensureSchema prüft und erstellt das Schema falls nötig
func (s *WeaviateVectorStore) ensureSchema(ctx context.Context) error {
	exists, err := s.client.Schema().ClassExistenceChecker().
		WithClassName(s.indexName).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check schema existence: %w", err)
	}

	if exists {
		return nil
	}

	classObj := &models.Class{
		Class:      s.indexName,
		Vectorizer: "none", // Externe Vektoren (von Ollama)
		Properties: []*models.Property{
			{
				Name:        "content",
				DataType:    []string{"text"},
				Description: "Code content of the chunk",
			},
			{
				Name:        "file_path",
				DataType:    []string{"string"},
				Description: "Path to the source file",
			},
			{
				Name:        "repo_id",
				DataType:    []string{"string"},
				Description: "Repository UUID",
			},
			{
				Name:        "start_line",
				DataType:    []string{"int"},
				Description: "Starting line number",
			},
			{
				Name:        "end_line",
				DataType:    []string{"int"},
				Description: "Ending line number",
			},
			{
				Name:        "language",
				DataType:    []string{"string"},
				Description: "Programming language",
			},
			{
				Name:        "signature",
				DataType:    []string{"text"},
				Description: "Function/class signature",
			},
			{
				Name:        "chunk_type",
				DataType:    []string{"string"},
				Description: "Type of code chunk (function, class, etc.)",
			},
		},
	}

	if err := s.client.Schema().ClassCreator().WithClass(classObj).Do(ctx); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// BatchUpsert fügt oder aktualisiert Code Chunks in Batches ein
func (s *WeaviateVectorStore) BatchUpsert(ctx context.Context, chunks []domain.CodeChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Validierung
	for i, chunk := range chunks {
		if chunk.RepoID == uuid.Nil {
			return fmt.Errorf("chunk at index %d has no repo_id", i)
		}
		if len(chunk.Embedding) == 0 {
			return fmt.Errorf("chunk at index %d has no embedding", i)
		}
	}

	// In Batches aufteilen für bessere Performance
	batchSize := DefaultBatchSize
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		if err := s.upsertBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to upsert batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// upsertBatch führt den eigentlichen Batch-Upsert durch
func (s *WeaviateVectorStore) upsertBatch(ctx context.Context, chunks []domain.CodeChunk) error {
	objects := make([]*models.Object, len(chunks))

	for i, chunk := range chunks {
		// Deterministische ID aus Chunk ID generieren (falls vorhanden)
		var objID string
		if chunk.ID != "" {
			// Versuche UUID zu parsen, sonst generiere neue
			if id, err := uuid.Parse(chunk.ID); err == nil {
				objID = id.String()
			} else {
				objID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(chunk.ID)).String()
			}
		}

		objects[i] = &models.Object{
			Class: s.indexName,
			ID:    strfmt.UUID(objID),
			Properties: map[string]interface{}{
				"content":    chunk.Content,
				"file_path":  chunk.FilePath,
				"repo_id":    chunk.RepoID.String(),
				"start_line": chunk.StartLine,
				"end_line":   chunk.EndLine,
				"language":   chunk.Language,
				"signature":  chunk.Signature,
				"chunk_type": string(chunk.ChunkType),
			},
			Vector: chunk.Embedding,
		}
	}

	// Batch Operation
	batchRes, err := s.client.Batch().ObjectsBatcher().
		WithObjects(objects...).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("batch operation failed: %w", err)
	}

	// Fehlerprüfung
	var errors []string
	for i, res := range batchRes {
		if res.Result.Errors != nil && res.Result.Errors.Error != nil {
			for _, e := range res.Result.Errors.Error {
				errors = append(errors, fmt.Sprintf("item %d: %s", i, e.Message))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch upsert errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// SearchSimilar sucht ähnliche Code Chunks basierend auf Vektor-Ähnlichkeit
func (s *WeaviateVectorStore) SearchSimilar(
	ctx context.Context,
	queryVector []float32,
	limit int,
	minScore float64,
	repoIDs []uuid.UUID,
) ([]domain.SearchResult, error) {
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector cannot be empty")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 1000 {
		limit = 1000 // Sicherheitsgrenze
	}

	fields := []graphql.Field{
		{Name: "content"},
		{Name: "file_path"},
		{Name: "start_line"},
		{Name: "end_line"},
		{Name: "signature"},
		{Name: "repo_id"},
		{Name: "language"},
		{Name: "chunk_type"},
		{Name: "_additional", Fields: []graphql.Field{
			{Name: "distance"},
			{Name: "id"},
		}},
	}

	// Query Builder
	builder := s.client.GraphQL().Get().
		WithClassName(s.indexName).
		WithFields(fields...).
		WithLimit(limit)

	// Vector Search mit Distance Threshold
	// Weaviate Distance: 0 (identisch) bis 2 (maximal unterschiedlich)
	// minScore 0.7 -> Distance 0.3
	maxDistance := float32(1.0 - minScore)
	if maxDistance > 2.0 {
		maxDistance = 2.0
	}
	if maxDistance < 0.0 {
		maxDistance = 0.0
	}

	builder = builder.WithNearVector(
		s.client.GraphQL().NearVectorArgBuilder().
			WithVector(queryVector).
			WithDistance(maxDistance),
	)

	// Repository Filter
	if len(repoIDs) > 0 {
		whereFilter := s.buildRepoFilter(repoIDs)
		builder = builder.WithWhere(whereFilter)
	}

	// Query ausführen
	result, err := builder.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("graphql errors: %v", result.Errors)
	}

	results, err := s.parseGraphQLResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse results: %w", err)
	}

	// Post-Filter für exakte Score-Grenze (optional, da Distance schon filtert)
	filtered := make([]domain.SearchResult, 0, len(results))
	for _, r := range results {
		if r.SemanticScore >= minScore {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

// buildRepoFilter erstellt einen Weaviate Filter für Repository IDs
func (s *WeaviateVectorStore) buildRepoFilter(repoIDs []uuid.UUID) *filters.WhereBuilder {
	if len(repoIDs) == 0 {
		return nil
	}

	if len(repoIDs) == 1 {
		return filters.Where().
			WithPath([]string{"repo_id"}).
			WithOperator(filters.Equal).
			WithValueString(repoIDs[0].String())
	}

	// Mehrere Repos: OR Verknüpfung
	operands := make([]*filters.WhereBuilder, len(repoIDs))
	for i, id := range repoIDs {
		operands[i] = filters.Where().
			WithPath([]string{"repo_id"}).
			WithOperator(filters.Equal).
			WithValueString(id.String())
	}

	return filters.Where().
		WithOperator(filters.Or).
		WithOperands(operands)
}

// DeleteByRepoID löscht alle Chunks eines Repositories
func (s *WeaviateVectorStore) DeleteByRepoID(ctx context.Context, repoID uuid.UUID) error {
	if repoID == uuid.Nil {
		return fmt.Errorf("repo_id cannot be nil")
	}

	result, err := s.client.Batch().ObjectsBatchDeleter().
		WithClassName(s.indexName).
		WithOutput("verbose").
		WithWhere(filters.Where().
			WithPath([]string{"repo_id"}).
			WithOperator(filters.Equal).
			WithValueString(repoID.String())).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("delete operation failed: %w", err)
	}

	// Optional: Log wie viele Objekte gelöscht wurden
	if result != nil && result.Results != nil {
		// Weaviate gibt Details über gelöschte Objekte zurück
	}

	return nil
}

// parseGraphQLResult parsed die GraphQL Response in SearchResults
func (s *WeaviateVectorStore) parseGraphQLResult(result *models.GraphQLResponse) ([]domain.SearchResult, error) {
	data, ok := result.Data["Get"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response structure: missing 'Get'")
	}

	classData, ok := data[s.indexName].([]any)
	if !ok {
		// Leere Ergebnisse sind ok
		return []domain.SearchResult{}, nil
	}

	results := make([]domain.SearchResult, 0, len(classData))

	for _, item := range classData {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}

		searchResult, err := s.parseSearchResultObject(obj)
		if err != nil {
			// Logging wäre hier sinnvoll, aber nicht abbrechen
			continue
		}

		results = append(results, searchResult)
	}

	return results, nil
}

// parseSearchResultObject parsed ein einzelnes Suchergebnis-Objekt
func (s *WeaviateVectorStore) parseSearchResultObject(obj map[string]any) (domain.SearchResult, error) {
	chunk := domain.CodeChunk{}

	// String Felder
	if v, ok := obj["content"].(string); ok {
		chunk.Content = v
	}
	if v, ok := obj["file_path"].(string); ok {
		chunk.FilePath = v
	}
	if v, ok := obj["language"].(string); ok {
		chunk.Language = v
	}
	if v, ok := obj["signature"].(string); ok {
		chunk.Signature = v
	}
	if v, ok := obj["chunk_type"].(string); ok {
		chunk.ChunkType = domain.ChunkType(v)
	}

	// Repository ID
	if repoIDStr, ok := obj["repo_id"].(string); ok {
		if uid, err := uuid.Parse(repoIDStr); err == nil {
			chunk.RepoID = uid
		}
	}

	// Integer Felder (JSON numbers sind float64)
	if v, ok := obj["start_line"].(float64); ok {
		chunk.StartLine = int(v)
	}
	if v, ok := obj["end_line"].(float64); ok {
		chunk.EndLine = int(v)
	}

	// Additional Fields (Distance & ID)
	var distance float64

	if additional, ok := obj["_additional"].(map[string]any); ok {
		if dist, ok := additional["distance"].(float64); ok {
			distance = dist
		}
		if id, ok := additional["id"].(string); ok {
			chunk.ID = id
		}
	}

	// Score berechnen aus Distance
	// Weaviate Cosine Distance: 0 (identisch) bis 2 (maximal unterschiedlich)
	// Score: 1.0 - distance (normalized to 0..1)
	similarity := 1.0 - distance
	if similarity < 0 {
		similarity = 0
	}
	if similarity > 1 {
		similarity = 1
	}

	return domain.SearchResult{
		Chunk:         chunk,
		SemanticScore: similarity,
		CombinedScore: similarity, // Keyword Score könnte später hinzugefügt werden
	}, nil
}

// GetStats gibt Statistiken über den Index zurück (optional, für Debugging)
func (s *WeaviateVectorStore) GetStats(ctx context.Context) (map[string]interface{}, error) {
	// Aggregation Query für Statistiken
	fields := []graphql.Field{
		{Name: "meta", Fields: []graphql.Field{
			{Name: "count"},
		}},
	}

	resp, err := s.client.GraphQL().Aggregate().
		WithClassName(s.indexName).
		WithFields(fields...).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// KORREKTUR: Explizite Konvertierung von map[string]models.JSONObject zu map[string]interface{}
	result := make(map[string]any)
	for k, v := range resp.Data {
		result[k] = v
	}

	return result, nil
}
