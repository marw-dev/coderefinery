package domain

import "time"

type ChunkType string

const (
	ChunkTypeFunction  ChunkType = "function"
	ChunkTypeMethod    ChunkType = "method"
	ChunkTypeClass     ChunkType = "class"
	ChunkTypeInterface ChunkType = "interface"
	ChunkTypeStruct    ChunkType = "struct"
	ChunkTypeGeneric   ChunkType = "generic"
	ChunkTypeOther     ChunkType = "other"
)

// CodeChunk repräsentiert einen Teil einer Datei
type CodeChunk struct {
	ID           string    `json:"id" db:"id"`

	FilePath     string    `json:"file_path" db:"-"`

	Content      string    `json:"content" db:"content"`
	Signature    string    `json:"signature" db:"signature"`
	Comments     string    `json:"comments" db:"comments"`
	StartLine    int       `json:"start_line" db:"start_line"`
	EndLine      int       `json:"end_line" db:"end_line"`
	ChunkType    ChunkType `json:"chunk_type" db:"chunk_type"`

	Language     string    `json:"language" db:"-"`

	Embedding    []float32 `json:"embedding" db:"embedding"`

	Imports      []string  `json:"imports" db:"imports"`

	// Mapping auf updated_at oder created_at der DB
	LastModified time.Time `json:"last_modified" db:"updated_at"`
}

type IndexStats struct {
	TotalFiles  int
	TotalChunks int
	Languages   map[string]int
	LastIndexed time.Time
}

// SearchRequest definiert alle Filter und Parameter für eine Suche
type SearchRequest struct {
	Query      string   `json:"query"`
	Limit      int      `json:"limit"`
	MinScore   float64  `json:"min_score"`
	Languages  []string `json:"languages"`
	ChunkTypes []string `json:"chunk_types"`
	PathFilter string   `json:"path_filter"` // Substring Match
	PathPrefix string   `json:"path_prefix"` // Prefix Match
}

// SearchResult ist ein einzelnes Suchergebnis mit Scores
type SearchResult struct {
	Chunk         CodeChunk `json:"chunk"`
	SemanticScore float64   `json:"semantic_score"`
	KeywordScore  float64   `json:"keyword_score"`
	CombinedScore float64   `json:"combined_score"`
	Rank          int       `json:"rank"`
}
