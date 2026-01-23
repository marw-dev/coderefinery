package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/pgvector/pgvector-go"
)

type chunkRepository struct {
	db *sqlx.DB
}

func NewChunkRepository(db *sqlx.DB) ports.ChunkRepository {
	return &chunkRepository{db: db}
}

// sqlChunk ist ein Hilfs-Struct für den Scan.
type sqlChunk struct {
	ID               string          `db:"id"`
	Content          string          `db:"content"`
	Signature        string          `db:"signature"`
	Comments         string          `db:"comments"`
	StartLine        int             `db:"start_line"`
	EndLine          int             `db:"end_line"`
	ChunkType        string          `db:"chunk_type"`
	Embedding        pgvector.Vector `db:"embedding"` // pgvector Struct
	ImportsJSON      []byte          `db:"imports"`   // JSON als []byte
	FilePath         string          `db:"file_path"`
	FileLanguage     string          `db:"file_language"`
	CosineSimilarity float64         `db:"cosine_similarity"`
}

func (r *chunkRepository) VectorSearch(ctx context.Context, embedding []float32, limit int, threshold float64) ([]*ports.ChunkSearchResult, error) {
	query := `
		SELECT
			c.id,
			c.content,
			COALESCE(c.signature, '') as signature,
			COALESCE(c.comments, '') as comments,
			c.start_line,
			c.end_line,
			c.chunk_type,
			c.embedding,
			c.imports,
			f.path as file_path,
			COALESCE(f.language, '') as file_language,
			1 - (c.embedding <=> $1) as cosine_similarity
		FROM code_chunks c
		JOIN files f ON c.file_id = f.id
		WHERE 1 - (c.embedding <=> $1) >= $2
		ORDER BY c.embedding <=> $1
		LIMIT $3
	`

	vec := pgvector.NewVector(embedding)

	var rawResults []sqlChunk

	err := r.db.SelectContext(ctx, &rawResults, query, vec, threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Mapping auf das Domain-Modell
	var results []*ports.ChunkSearchResult
	for _, row := range rawResults {
		// Imports parsen
		var imports []string
		if len(row.ImportsJSON) > 0 {
			_ = json.Unmarshal(row.ImportsJSON, &imports)
		}

		// Domain Objekt bauen
		chunk := domain.CodeChunk{
			ID:        row.ID,
			FilePath:  row.FilePath,
			Content:   row.Content,
			Signature: row.Signature,
			Comments:  row.Comments,
			StartLine: row.StartLine,
			EndLine:   row.EndLine,
			ChunkType: domain.ChunkType(row.ChunkType),
			Language:  row.FileLanguage,
			Embedding: row.Embedding.Slice(), // Extrahiert []float32
			Imports:   imports,
		}

		results = append(results, &ports.ChunkSearchResult{
			CodeChunk:  chunk,
			Similarity: row.CosineSimilarity,
			File:       row.FilePath,
			Language:   row.FileLanguage,
		})
	}

	return results, nil
}

func (r *chunkRepository) HybridSearch(ctx context.Context, embedding []float32, textQuery string, limit int) ([]*ports.ChunkSearchResult, error) {
	return r.VectorSearch(ctx, embedding, limit, 0.0)
}
