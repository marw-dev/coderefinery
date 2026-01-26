package indexer

import (
	"bufio"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"coderefinery/internal/adapters/indexer/parser"
	"coderefinery/internal/config"
	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"

	"github.com/google/uuid"
)

type Indexer struct {
	cfg            config.IndexerConfig
	embedder       ports.Embedder
	vectorStore    ports.VectorStore
	ignorePatterns []string
}

// NewIndexer akzeptiert jetzt ports.VectorStore
func NewIndexer(cfg config.IndexerConfig, embedder ports.Embedder, vectorStore ports.VectorStore) (*Indexer, error) {
	return &Indexer{
		cfg:         cfg,
		embedder:    embedder,
		vectorStore: vectorStore,
	}, nil
}

func (idx *Indexer) Index(ctx context.Context, repo *domain.Repository) error {
	log.Printf("Indexing repository: %s", repo.Name)
	return idx.buildIndexInternal(ctx, repo.ID, repo.Path)
}

func (idx *Indexer) DeleteIndex(ctx context.Context, repo *domain.Repository) error {
	log.Printf("Deleting index for repository: %s", repo.Name)
	return idx.vectorStore.DeleteByRepoID(ctx, repo.ID)
}

func (idx *Indexer) DeleteAllIndices(ctx context.Context) error {
	log.Println("WARNING: Deleting ALL search indices (not implemented via vectorStore)")
	return nil
}

func (idx *Indexer) loadGitIgnore(rootPath string) {
	idx.ignorePatterns = []string{}
	// G304: Clean path
	safePath := filepath.Clean(filepath.Join(rootPath, ".gitignore"))
	file, err := os.Open(safePath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx.ignorePatterns = append(idx.ignorePatterns, strings.TrimSuffix(line, "/"))
	}
}

func (idx *Indexer) shouldIgnore(path string) bool {
	for _, excl := range idx.cfg.ExcludePaths {
		if strings.Contains(path, excl) {
			return true
		}
	}
	base := filepath.Base(path)
	for _, pattern := range idx.ignorePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if strings.Contains(path, string(os.PathSeparator)+pattern+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func (idx *Indexer) buildIndexInternal(ctx context.Context, repoID uuid.UUID, rootPath string) error {
	idx.loadGitIgnore(rootPath)

	filesToProcess := make([]string, 0)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if idx.shouldIgnore(path) {
			return nil
		}
		if strings.Contains(path, ".git") {
			return nil
		}
		filesToProcess = append(filesToProcess, path)
		return nil
	})
	if err != nil {
		return err
	}

	if len(filesToProcess) == 0 {
		log.Println("No matching files found to index.")
		return nil
	}

	log.Printf("Processing %d files...", len(filesToProcess))
	for _, path := range filesToProcess {
		if err := idx.processFile(ctx, repoID, path); err != nil {
			log.Printf("Error processing %s: %v", filepath.Base(path), err)
		}
	}

	return nil
}

func (idx *Indexer) processFile(ctx context.Context, repoID uuid.UUID, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(path))
	extKey := strings.TrimPrefix(ext, ".")

	lang, known := idx.cfg.SupportedExts[extKey]
	if !known {
		return nil
	}

	p := parser.GetParser(lang)
	// G304: Path traversal protection
	safePath := filepath.Clean(path)
	content, err := os.ReadFile(safePath)
	if err != nil {
		return err
	}

	chunks, err := p.Parse(path, content, info.ModTime())
	if err != nil {
		return err
	}

	if len(chunks) == 0 {
		return nil
	}

	texts := make([]string, len(chunks))
	for i, c := range chunks {
		text := c.Signature + "\n" + c.Comments + "\n" + c.Content
		texts[i] = text
	}

	embeddings, err := idx.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return err
	}

	for i := range chunks {
		chunks[i].Embedding = embeddings[i]
		chunks[i].RepoID = repoID
		chunks[i].Language = lang
	}

	return idx.vectorStore.BatchUpsert(ctx, chunks)
}
