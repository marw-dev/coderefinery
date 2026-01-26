package parser

import (
	"path/filepath"
	"strings"
	"time"

	"coderefinery/internal/core/domain"
)



func Parse(filePath string, content []byte, modTime time.Time) ([]domain.CodeChunk, error) {
	lines := strings.Split(string(content), "\n")
	var chunks []domain.CodeChunk

	var currentChunk strings.Builder
	chunkStart := 0

	for i, line := range lines {
		currentChunk.WriteString(line + "\n")

		// Verwendung von Literalen
		if currentChunk.Len() > 1500 || i == len(lines)-1 {
			if currentChunk.Len() > 50 {
				ext := filepath.Ext(filePath)
				lang := strings.TrimPrefix(ext, ".")

				chunks = append(chunks, domain.CodeChunk{
					ID:           generateID(filePath, chunkStart),
					FilePath:     filePath,
					Content:      strings.TrimSpace(currentChunk.String()),
					StartLine:    chunkStart + 1,
					EndLine:      i + 1,
					ChunkType:    domain.ChunkTypeGeneric,
					Language:     lang,
					LastModified: modTime,
				})
			}
			currentChunk.Reset()
			chunkStart = i + 1
		}
	}

	return chunks, nil
}

func SupportsLanguage(lang string) bool {
	return true
}
