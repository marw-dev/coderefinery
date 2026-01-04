package search

import (
	"context"
	"math"
	"sort"
	"strings"

	"coderefinery/internal/domain"
	"coderefinery/internal/indexer"
	"coderefinery/pkg/mathutil"
)

type Searcher struct {
	indexer *indexer.Indexer
}

func NewSearcher(idx *indexer.Indexer) *Searcher {
	return &Searcher{indexer: idx}
}

// SearchResult erweitert um Rank-Informationen für RRF
type rankInfo struct {
	result        domain.SearchResult
	semanticRank  int
	keywordRank   int
	combinedScore float64
}

func (s *Searcher) Search(ctx context.Context, req domain.SearchRequest, queryEmbed []float32, limit int) []domain.SearchResult {
	queryLower := strings.ToLower(req.Query)
	queryTerms := s.extractRelevantTerms(queryLower)

	// Phase 1: Separate Retrieval (Semantic + Keyword)
	semanticResults := make([]rankInfo, 0, 500)
	keywordResults := make([]rankInfo, 0, 500)

	s.indexer.IterateChunks(func(chunk domain.CodeChunk) bool {
		// Hard Filters zuerst (schnell)
		if !s.matchesFilters(chunk, req) {
			return true
		}

		// Semantic Score
		semanticScore := mathutil.CosineSimilarity(queryEmbed, chunk.Embedding)

		// Keyword Score (verbessert)
		keywordScore := s.calculateEnhancedKeywordScore(chunk, queryTerms, queryLower)

		result := domain.SearchResult{
			Chunk:         chunk,
			SemanticScore: semanticScore,
			KeywordScore:  keywordScore,
		}

		// Sammle Top-K für beide Methoden
		if semanticScore > 0.25 {
			semanticResults = append(semanticResults, rankInfo{result: result})
		}
		if keywordScore > 0.15 {
			keywordResults = append(keywordResults, rankInfo{result: result})
		}

		return true
	})

	// Phase 2: Separate Ranking
	sort.Slice(semanticResults, func(i, j int) bool {
		return semanticResults[i].result.SemanticScore > semanticResults[j].result.SemanticScore
	})
	sort.Slice(keywordResults, func(i, j int) bool {
		return keywordResults[i].result.KeywordScore > keywordResults[j].result.KeywordScore
	})

	// Assign Ranks
	for i := range semanticResults {
		semanticResults[i].semanticRank = i + 1
	}
	for i := range keywordResults {
		keywordResults[i].keywordRank = i + 1
	}

	// Phase 3: Reciprocal Rank Fusion (RRF)
	// Best Practice: RRF ist robuster als gewichtete Addition
	fusedResults := s.reciprocalRankFusion(semanticResults, keywordResults, 60)

	// Phase 4: Adaptive Filtering mit Elbow Detection
	finalResults := s.adaptiveFilter(fusedResults, req.MinScore, limit)

	// Ranks neu zuweisen
	for i := range finalResults {
		finalResults[i].Rank = i + 1
	}

	return finalResults
}

// reciprocalRankFusion kombiniert Rankings mit RRF
// RRF Score = sum(1 / (k + rank)) für jeden Retriever
// k=60 ist der Standardwert (aus Forschung)
func (s *Searcher) reciprocalRankFusion(semanticResults, keywordResults []rankInfo, k float64) []domain.SearchResult {
	// Merge beide Listen in Map (dedup by chunk ID)
	scoreMap := make(map[string]*rankInfo)

	for _, r := range semanticResults {
		id := r.result.Chunk.ID
		if _, exists := scoreMap[id]; !exists {
			scoreMap[id] = &rankInfo{result: r.result}
		}
		scoreMap[id].semanticRank = r.semanticRank
	}

	for _, r := range keywordResults {
		id := r.result.Chunk.ID
		if _, exists := scoreMap[id]; !exists {
			scoreMap[id] = &rankInfo{result: r.result}
		}
		scoreMap[id].keywordRank = r.keywordRank
	}

	// Calculate RRF Score
	results := make([]domain.SearchResult, 0, len(scoreMap))
	for _, info := range scoreMap {
		rrfScore := 0.0

		// Semantic contribution
		if info.semanticRank > 0 {
			rrfScore += 1.0 / (k + float64(info.semanticRank))
		}

		// Keyword contribution
		if info.keywordRank > 0 {
			rrfScore += 1.0 / (k + float64(info.keywordRank))
		}

		info.result.CombinedScore = rrfScore
		results = append(results, info.result)
	}

	// Sort by RRF score
	sort.Slice(results, func(i, j int) bool {
		return results[i].CombinedScore > results[j].CombinedScore
	})

	return results
}

// adaptiveFilter implementiert intelligente Filterung mit Elbow Detection
func (s *Searcher) adaptiveFilter(results []domain.SearchResult, minScore float64, limit int) []domain.SearchResult {
	if len(results) == 0 {
		return results
	}

	if minScore <= 0 {
		minScore = 0.01 // RRF-Scores sind viel niedriger
	}

	finalResults := make([]domain.SearchResult, 0)
	highestScore := results[0].CombinedScore

	for i, res := range results {
		// Hard Limit
		if limit > 0 && i >= limit {
			break
		}

		// Absolute Minimum
		if res.CombinedScore < minScore {
			break
		}

		// Relative Quality: Cut bei 40% vom Top (für RRF)
		if res.CombinedScore < (highestScore * 0.4) {
			break
		}

		// Elbow Detection: Großer Score-Drop
		if i > 0 {
			prevScore := results[i-1].CombinedScore
			drop := prevScore - res.CombinedScore
			dropPercent := drop / prevScore

			// Wenn Score plötzlich >50% fällt -> Cut
			if dropPercent > 0.5 {
				break
			}
		}

		finalResults = append(finalResults, res)
	}

	return finalResults
}

// calculateEnhancedKeywordScore mit TF-IDF-inspiriertem Scoring
func (s *Searcher) calculateEnhancedKeywordScore(chunk domain.CodeChunk, terms []string, fullQuery string) float64 {
	if len(terms) == 0 {
		return 0
	}

	score := 0.0
	contentLower := strings.ToLower(chunk.Content)
	pathLower := strings.ToLower(chunk.FilePath)
	sigLower := strings.ToLower(chunk.Signature)

	// 1. Exakte Phrase Match (höchste Priorität)
	if strings.Contains(contentLower, fullQuery) {
		score += 5.0
	}
	if strings.Contains(sigLower, fullQuery) {
		score += 8.0
	}
	if strings.Contains(pathLower, fullQuery) {
		score += 6.0
	}

	// 2. Individual Term Matches mit Gewichtung
	for _, term := range terms {
		termLen := len(term)
		if termLen < 3 {
			continue // Ignoriere Stopwords
		}

		// Gewicht basierend auf Term-Länge (längere = spezifischer)
		termWeight := math.Min(float64(termLen)/10.0, 1.5)

		// Path Match
		if strings.Contains(pathLower, term) {
			score += 3.0 * termWeight
		}

		// Signature Match (Funktions-/Klassennamen)
		if strings.Contains(sigLower, term) {
			score += 2.5 * termWeight
		}

		// Content Match mit TF-Komponente
		count := float64(strings.Count(contentLower, term))
		if count > 0 {
			// Logarithmisches TF (vermeidet Spam-Effekt)
			tf := 1.0 + math.Log(count)
			score += tf * termWeight
		}

		// Comments Match (mittlere Priorität)
		if strings.Contains(strings.ToLower(chunk.Comments), term) {
			score += 1.5 * termWeight
		}
	}

	// 3. Chunk-Type Boost für relevante Code-Strukturen
	switch chunk.ChunkType {
	case domain.ChunkTypeFunction, domain.ChunkTypeMethod:
		score *= 1.2
	case domain.ChunkTypeClass, domain.ChunkTypeInterface:
		score *= 1.15
	}

	// Normalisierung: 0-1 Skala (mit Soft Cap)
	normalizedScore := score / (score + 10.0)

	return normalizedScore
}

// extractRelevantTerms entfernt Stopwords und extrahiert wichtige Begriffe
func (s *Searcher) extractRelevantTerms(query string) []string {
	stopwords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
		"be": true, "by": true, "for": true, "from": true, "has": true, "he": true,
		"in": true, "is": true, "it": true, "its": true, "of": true, "on": true,
		"that": true, "the": true, "to": true, "was": true, "will": true, "with": true,
		// Code-spezifische low-value words
		"get": true, "set": true, "new": true, "old": true, "this": true,
	}

	words := strings.Fields(query)
	relevant := make([]string, 0, len(words))

	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) >= 2 && !stopwords[word] {
			relevant = append(relevant, word)
		}
	}

	return relevant
}

func (s *Searcher) matchesFilters(chunk domain.CodeChunk, req domain.SearchRequest) bool {
	// Language Filter
	if len(req.Languages) > 0 {
		match := false
		for _, l := range req.Languages {
			if strings.EqualFold(chunk.Language, l) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// Path Filter
	if req.PathFilter != "" && !strings.Contains(strings.ToLower(chunk.FilePath), strings.ToLower(req.PathFilter)) {
		return false
	}

	if req.PathPrefix != "" && !strings.HasPrefix(chunk.FilePath, req.PathPrefix) {
		return false
	}

	// Chunk Type Filter
	if len(req.ChunkTypes) > 0 {
		match := false
		for _, ct := range req.ChunkTypes {
			if strings.EqualFold(string(chunk.ChunkType), ct) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	return true
}
