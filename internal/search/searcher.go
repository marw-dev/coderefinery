package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type Searcher struct {
	chunkRepo ports.ChunkRepository
	embedder  ports.Embedder
	cache	  ports.Cache
}

func NewSearcher(chunkRepo ports.ChunkRepository, embedder ports.Embedder, cache ports.Cache) *Searcher {
	return &Searcher{
		chunkRepo: chunkRepo,
		embedder:  embedder,
		cache: cache,
	}
}

// Search führt die semantische Suche durch.
func (s *Searcher) Search(ctx context.Context, req domain.SearchRequest) ([]domain.SearchResult, error) {
	tracer := otel.Tracer("search-service")
	ctx, span := tracer.Start(ctx, "SearchOperation")
	defer span.End()

	span.SetAttributes(
		attribute.String("query", req.Query),
		attribute.Int("limit", req.Limit),
	)

	// --- 1. CACHE CHECK ---
	cacheKey := s.generateCacheKey(req)
	var cachedResults []domain.SearchResult

	found, err := s.cache.Get(ctx, cacheKey, &cachedResults)
	if err == nil && found {
		span.SetAttributes(attribute.Bool("cache_hit", true))
		return cachedResults, nil
	}
	span.SetAttributes(attribute.Bool("cache_hit", false))

	// 2. Embedding
	_, embedSpan := tracer.Start(ctx, "EmbedQuery")
	queryEmbed, err := s.embedder.Embed(ctx, req.Query)
	embedSpan.End()
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// 3. Vector Search
	_, dbSpan := tracer.Start(ctx, "VectorSearchDB")
	results, err := s.chunkRepo.VectorSearch(ctx, queryEmbed, req.Limit, req.MinScore)
	dbSpan.End()
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// 4. Mapping
	domainResults := make([]domain.SearchResult, len(results))
	for i, r := range results {
		domainResults[i] = domain.SearchResult{
			Chunk:         r.CodeChunk,
			SemanticScore: r.Similarity,
			CombinedScore: r.Similarity,
			Rank:          i + 1,
		}
	}

	// --- 5. CACHE SET ---
	// Wir speichern das Ergebnis asynchron (oder synchron, hier kurz gehalten)
	_ = s.cache.Set(ctx, cacheKey, domainResults, 0) // 0 = Default TTL

	return domainResults, nil
}

func (s *Searcher) generateCacheKey(req domain.SearchRequest) string {
	// Key muss eindeutig für die Anfrage sein
	data := fmt.Sprintf("%s|%d|%f", req.Query, req.Limit, req.MinScore)
	hash := sha256.Sum256([]byte(data))
	return "search:" + hex.EncodeToString(hash[:])
}
