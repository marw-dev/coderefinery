package domain

import "time"

type SearchRequest struct {
	Query      string   `json:"query"`
	Limit      int      `json:"limit"`
	MinScore   float64  `json:"min_score,omitempty"`
	Languages  []string `json:"languages,omitempty"`
	PathFilter string   `json:"path_filter,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
	ChunkTypes []string `json:"chunk_types,omitempty"`
}

type CodeChunk struct {
	ID           string
	FilePath     string
	Content      string
	Signature    string
	Comments     string
	StartLine    int
	EndLine      int
	ChunkType    ChunkType
	Language     string
	Embedding    []float32
	Imports      []string
	LastModified time.Time
}

type ChunkType string

const (
	ChunkTypeFunction  ChunkType = "function"
	ChunkTypeMethod    ChunkType = "method"
	ChunkTypeClass     ChunkType = "class"
	ChunkTypeStruct    ChunkType = "struct"
	ChunkTypeInterface ChunkType = "interface"
	ChunkTypeGeneric   ChunkType = "generic"
)

type SearchResult struct {
	Chunk         CodeChunk
	SemanticScore float64
	KeywordScore  float64
	CombinedScore float64
	Rank          int
}

type IndexStats struct {
	TotalChunks  int
	TotalFiles   int
	Languages    map[string]int
	LastIndexed  time.Time
}
