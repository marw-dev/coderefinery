package vectordb

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"coderefinery/internal/config"
	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"

	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

type WeaviateVectorStore struct {
	client    *weaviate.Client
	indexName string // Name der Klasse in Weaviate, z.B. "CodeChunk"
	timeout   time.Duration
}

// NewWeaviateVectorStore erstellt eine neue Instanz und initialisiert das Schema falls nötig
func NewWeaviateVectorStore(cfg config.VectorDBConfig) (ports.VectorStore, error) {
	wCfg := weaviate.Config{
		Host:   cfg.Host,
		Scheme: cfg.Scheme,
		ConnectionClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}

	// Auth Config (falls API Key gesetzt)
	if cfg.APIKey != "" {
		wCfg.AuthConfig = auth.ApiKey{Value: cfg.APIKey}
	}

	client, err := weaviate.NewClient(wCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create weaviate client: %w", err)
	}

	store := &WeaviateVectorStore{
		client:    client,
		indexName: cfg.IndexName,
		timeout:   cfg.Timeout,
	}

	// Schema Check / Init beim Start
	if err := store.ensureSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure schema: %w", err)
	}

	return store, nil
}

// ensureSchema prüft ob die Klasse existiert, sonst legt er sie an
func (s *WeaviateVectorStore) ensureSchema(ctx context.Context) error {
	exists, err := s.client.Schema().ClassExistenceChecker().
		WithClassName(s.indexName).
		Do(ctx)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	// Klasse definieren
	classObj := &models.Class{
		Class:      s.indexName,
		Vectorizer: "none", // Wir bringen unsere eigenen Vektoren (von Ollama) mit
		Properties: []*models.Property{
			{
				Name:     "content",
				DataType: []string{"text"},
			},
			{
				Name:     "file_path",
				DataType: []string{"string"}, // "string" wird exakt gematched (filterbar)
			},
			{
				Name:     "repo_id",
				DataType: []string{"string"}, // UUID als String speichern für Filter
			},
			{
				Name:     "start_line",
				DataType: []string{"int"},
			},
			{
				Name:     "end_line",
				DataType: []string{"int"},
			},
			{
				Name:     "language",
				DataType: []string{"string"},
			},
			{
				Name:     "signature",
				DataType: []string{"text"},
			},
			{
				Name:     "chunk_type",
				DataType: []string{"string"},
			},
		},
	}

	return s.client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
}

func (s *WeaviateVectorStore) BatchUpsert(ctx context.Context, chunks []domain.CodeChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	objects := make([]*models.Object, len(chunks))
	for i, chunk := range chunks {
		// RepoID muss vorhanden sein!
		if chunk.RepoID == uuid.Nil {
			return fmt.Errorf("chunk %s has no repo_id", chunk.ID)
		}

		objects[i] = &models.Object{
			Class: s.indexName,
			// ID:    chunk.ID, // Optional: Weaviate kann auch eigene IDs generieren, aber deterministische IDs sind besser für Updates
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
			Vector: chunk.Embedding, // Hier kommt der Vektor von Ollama rein
		}
	}

	// Batch senden
	batchRes, err := s.client.Batch().ObjectsBatcher().
		WithObjects(objects...).
		Do(ctx)
	if err != nil {
		return err
	}

	// Fehler im Batch prüfen
	for _, res := range batchRes {
		if res.Result.Errors != nil {
			return fmt.Errorf("error in batch upsert: %v", res.Result.Errors)
		}
	}

	return nil
}

func (s *WeaviateVectorStore) SearchSimilar(ctx context.Context, queryVector []float32, limit int, minScore float64, repoIDs []uuid.UUID) ([]domain.SearchResult, error) {
	// Felder die wir zurückwollen
	fields := []graphql.Field{
		{Name: "content"},
		{Name: "file_path"},
		{Name: "start_line"},
		{Name: "end_line"},
		{Name: "signature"},
		{Name: "repo_id"},
		{Name: "_additional", Fields: []graphql.Field{
			{Name: "distance"}, // Weaviate gibt Distance (0..2), nicht Score
			{Name: "id"},
		}},
	}

	builder := s.client.GraphQL().Get().
		WithClassName(s.indexName).
		WithFields(fields...).
		WithNearVector(s.client.GraphQL().NearVectorArgBuilder().
			WithVector(queryVector).
			WithDistance(1.0 - float32(minScore))) // Distance Threshold (Ungefähr, Weaviate logic ist komplex)
			// Alternativ: Einfach Limit nehmen und Score später filtern

	builder = builder.WithLimit(limit)

	// Filter einbauen (WHERE repo_id IN [...])
	if len(repoIDs) > 0 {
		operands := make([]*filters.WhereBuilder, len(repoIDs))
		for i, id := range repoIDs {
			operands[i] = filters.Where().
				WithPath([]string{"repo_id"}).
				WithOperator(filters.Equal).
				WithValueString(id.String())
		}

		// Wenn mehr als 1 Repo, dann OR Verknüpfung
		if len(repoIDs) > 1 {
			builder = builder.WithWhere(filters.Where().
				WithOperator(filters.Or).
				WithOperands(operands))
		} else {
			// Bei einem Repo direkt
			builder = builder.WithWhere(operands[0])
		}
	}

	result, err := builder.Do(ctx)
	if err != nil {
		return nil, err
	}

	if result.Errors != nil {
		return nil, fmt.Errorf("graphql error: %v", result.Errors)
	}

	return s.parseGraphQLResult(result)
}

func (s *WeaviateVectorStore) DeleteByRepoID(ctx context.Context, repoID uuid.UUID) error {
	// Batch Delete API nutzen
	// DELETE FROM CodeChunk WHERE repo_id = "..."

	_, err := s.client.Batch().ObjectsBatchDeleter().
		WithClassName(s.indexName).
		WithOutput("verbose").
		WithWhere(filters.Where().
			WithPath([]string{"repo_id"}).
			WithOperator(filters.Equal).
			WithValueString(repoID.String())).
		Do(ctx)

	return err
}

func (s *WeaviateVectorStore) parseGraphQLResult(result *models.GraphQLResponse) ([]domain.SearchResult, error) {
	// Das Parsen von `models.GraphQLResponse` ist in Go etwas mühsam, da es map[string]any ist.
	// Hier muss man vorsichtig casten.

	data, ok := result.Data["Get"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response structure: 'Get' missing")
	}

	classData, ok := data[s.indexName].([]any)
	if !ok {
		// Kann passieren wenn leer
		return []domain.SearchResult{}, nil
	}

	results := make([]domain.SearchResult, 0, len(classData))

	for _, item := range classData {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}

		chunk := domain.CodeChunk{}

		// Content
		if content, ok := obj["content"].(string); ok {
			chunk.Content = content
		}
		if filePath, ok := obj["file_path"].(string); ok {
			chunk.FilePath = filePath
		}
		if repoIDStr, ok := obj["repo_id"].(string); ok {
			if uid, err := uuid.Parse(repoIDStr); err == nil {
				chunk.RepoID = uid
			}
		}
		if start, ok := obj["start_line"].(float64); ok { // JSON numbers sind float64
			chunk.StartLine = int(start)
		}
		if end, ok := obj["end_line"].(float64); ok {
			chunk.EndLine = int(end)
		}
		if sig, ok := obj["signature"].(string); ok {
			chunk.Signature = sig
		}

		// Additional Fields (Distance/Score)
		var distance float64
		if additional, ok := obj["_additional"].(map[string]interface{}); ok {
			if dist, ok := additional["distance"].(float64); ok {
				distance = dist
			}
		}

		// Score berechnen (1 - Distance)
		// Weaviate Cosine Distance geht von 0 (identisch) bis 2 (Gegenteil).
		// Einfache Heuristik: 1 - (distance / 2) oder ähnlich, je nach Metrik.
		// Für Cosine Distance ist Similarity = 1 - Distance oft passend für 0..1 Normalisierung.
		similarity := 1.0 - distance

		results = append(results, domain.SearchResult{
			Chunk:         chunk,
			SemanticScore: similarity,
			CombinedScore: similarity, // Keyword Score kommt später vielleicht dazu
		})
	}

	return results, nil
}
