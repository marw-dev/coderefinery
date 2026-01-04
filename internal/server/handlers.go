package server

import (
	"net/http"
	"time"

	"coderefinery/internal/domain"

	"github.com/gin-gonic/gin"
)

type SearchRequest struct {
	Query      string   `json:"query" binding:"required"`
	Limit      int      `json:"limit"`
	MinScore   float64  `json:"min_score"`
	Languages  []string `json:"languages"`
	PathFilter string   `json:"path_filter"`
	PathPrefix string   `json:"path_prefix"`
	ChunkTypes []string `json:"chunk_types"`
}

type SearchResponse struct {
	Results []domain.SearchResult `json:"results"`
	Count   int                   `json:"count"`
	Query   string                `json:"query"`
	Took    string                `json:"took"`
}

type HealthResponse struct {
	Status string `json:"status"`
	Chunks int    `json:"chunks"`
	Files  int    `json:"files"`
}

func (s *Server) handleSearch(c *gin.Context) {
	startTime := time.Now()

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.MinScore == 0 {
		req.MinScore = 0.01 // Default für RRF
	}

	// 1. Embedder holen
	embedder := s.indexer.GetEmbedder()

	// 2. Query einbetten
	queryEmbed, err := embedder.Embed(c.Request.Context(), req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process query embedding"})
		return
	}

	// 3. Domain-Request erstellen
	domainReq := domain.SearchRequest{
		Query:      req.Query,
		Limit:      req.Limit,
		MinScore:   req.MinScore,
		Languages:  req.Languages,
		PathFilter: req.PathFilter,
		PathPrefix: req.PathPrefix,
		ChunkTypes: req.ChunkTypes,
	}

	// 4. Suchen (mit optimiertem RRF)
	results := s.searcher.Search(c.Request.Context(), domainReq, queryEmbed, req.Limit)

	// Timing berechnen
	took := time.Since(startTime)

	// 5. Antworten
	c.JSON(http.StatusOK, SearchResponse{
		Results: results,
		Count:   len(results),
		Query:   req.Query,
		Took:    took.String(),
	})
}

func (s *Server) handleHealth(c *gin.Context) {
	stats := s.indexer.Stats()
	c.JSON(http.StatusOK, HealthResponse{
		Status: "ok",
		Chunks: stats.TotalChunks,
		Files:  stats.TotalFiles,
	})
}

func (s *Server) handleStats(c *gin.Context) {
	stats := s.indexer.Stats()
	c.JSON(http.StatusOK, stats)
}
