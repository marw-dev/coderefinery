package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Searcher struct {
	vectorStore ports.VectorStore
	embedder    ports.Embedder
	cache       ports.Cache
}

// NewSearcher akzeptiert nun ports.Cache
func NewSearcher(store ports.VectorStore, embedder ports.Embedder, cache ports.Cache) *Searcher {
	return &Searcher{
		vectorStore: store,
		embedder:    embedder,
		cache:       cache,
	}
}

func (s *Searcher) Search(ctx context.Context, req domain.SearchRequest) ([]domain.SearchResult, error) {
	// 1. Cache Check
	cacheKey := s.generateCacheKey(req)
	if s.cache != nil {
		var cachedResults []domain.SearchResult
		found, err := s.cache.Get(ctx, cacheKey, &cachedResults)
		if err == nil && found {
			log.Debug().Str("query", req.Query).Msg("Cache hit L1/L2")
			return cachedResults, nil
		}
	}

	// 2. Embedding generieren
	queryVector, err := s.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// 3. Vektorsuche durchführen
	var repoFilter []uuid.UUID
	if req.RepoID != uuid.Nil {
		repoFilter = []uuid.UUID{req.RepoID}
	}

	results, err := s.vectorStore.SearchSimilar(ctx, queryVector, req.Limit, req.MinScore, repoFilter)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// 4. Cache Update
	if len(results) > 0 && s.cache != nil {
		_ = s.cache.Set(ctx, cacheKey, results, 1*time.Hour)
	}

	return results, nil
}

func (s *Searcher) generateCacheKey(req domain.SearchRequest) string {
	data := fmt.Sprintf("%s|%d|%f|%s", req.Query, req.Limit, req.MinScore, req.RepoID.String())
	hash := sha256.Sum256([]byte(data))
	return "search:" + hex.EncodeToString(hash[:])
}
