package indexer

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"coderefinery/internal/config"
	"coderefinery/internal/domain"
	"coderefinery/internal/embedding"
	"coderefinery/internal/indexer/parser"

	"github.com/fsnotify/fsnotify"
)

type Indexer struct {
	cfg      config.IndexerConfig
	embedder embedding.Embedder
	db       *DB
	chunks   map[string]domain.CodeChunk
	mu       sync.RWMutex
	watcher  *fsnotify.Watcher
	stats    domain.IndexStats
	ignorePatterns []string // Cache für .gitignore Patterns
}

func NewIndexer(cfg config.IndexerConfig, embedder embedding.Embedder, dbPath string) (*Indexer, error) {
	db, err := NewDB(dbPath)
	if err != nil {
		return nil, err
	}

	log.Println("Loading chunks from database...")
	chunks, err := db.LoadAllChunks()
	if err != nil {
		log.Printf("Warning: Could not load chunks: %v", err)
		chunks = make(map[string]domain.CodeChunk)
	} else {
		log.Printf("Loaded %d chunks from history", len(chunks))
	}

	return &Indexer{
		cfg:      cfg,
		embedder: embedder,
		db:       db,
		chunks:   chunks,
		stats: domain.IndexStats{
			TotalChunks: len(chunks),
			Languages:   make(map[string]int),
		},
	}, nil
}

// loadGitIgnore liest einfache Patterns aus der .gitignore
func (idx *Indexer) loadGitIgnore(rootPath string) {
	idx.ignorePatterns = []string{}
	file, err := os.Open(filepath.Join(rootPath, ".gitignore"))
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
		// Einfache Normalisierung: Entferne trailing slashes
		idx.ignorePatterns = append(idx.ignorePatterns, strings.TrimSuffix(line, "/"))
	}
	log.Printf("Loaded %d patterns from .gitignore", len(idx.ignorePatterns))
}

func (idx *Indexer) shouldIgnore(path string) bool {
	// 1. Config Excludes (Substring Match)
	for _, excl := range idx.cfg.ExcludePaths {
		if strings.Contains(path, excl) {
			return true
		}
	}

	// 2. Gitignore Patterns (Simple Match)
	base := filepath.Base(path)
	for _, pattern := range idx.ignorePatterns {
		// Match filename
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		// Match directories (sehr simpler check)
		if strings.Contains(path, string(os.PathSeparator)+pattern+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func isBinaryFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Lese die ersten 512 Bytes
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Wenn NUL-Byte (0x00) vorkommt -> Wahrscheinlich Binär
	if bytes.IndexByte(buf[:n], 0) != -1 {
		return true, nil
	}

	return false, nil
}

func (idx *Indexer) BuildIndex(ctx context.Context, rootPath string) error {
	log.Println("Scanning for files (Universal Mode)...")
	idx.loadGitIgnore(rootPath)

	filesToProcess := make([]string, 0)
	currentFiles := make(map[string]bool)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// 1. Excludes prüfen (node_modules, .git)
		if idx.shouldIgnore(path) {
			return nil
		}

		// 2. WICHTIG: Keine Whitelist mehr! Wir prüfen stattdessen auf Binärdaten.
		// Das öffnet das Tor für Rust, Assembly, Configs, etc.
		isBin, err := isBinaryFile(path)
		if err != nil || isBin {
			return nil // Skip Binary
		}

		currentFiles[path] = true

		lastModDB, exists := idx.db.GetFileModTime(path)
		if !exists || info.ModTime().After(lastModDB) {
			filesToProcess = append(filesToProcess, path)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Clean up deleted files
	deletedCount, _ := idx.cleanupDeletedFiles(currentFiles)
	if deletedCount > 0 {
		log.Printf("Cleaned up %d deleted files", deletedCount)
	}

	if len(filesToProcess) == 0 {
		log.Println("✓ Index is up to date.")
		return nil
	}

	log.Printf("Re-indexing %d files (Dynamic Language Detection)...", len(filesToProcess))

	// Batch Process
	for i, path := range filesToProcess {
		if err := idx.processFile(ctx, path); err != nil {
			log.Printf("Skipped %s: %v", filepath.Base(path), err)
		}
		if i > 0 && i%50 == 0 {
			log.Printf("Progress: %d/%d", i, len(filesToProcess))
		}
	}

	// Index Update im RAM
	newChunks, _ := idx.db.LoadAllChunks()
	idx.mu.Lock()
	idx.chunks = newChunks
	idx.stats.TotalChunks = len(newChunks)
	idx.stats.TotalFiles = len(currentFiles)
	idx.stats.LastIndexed = time.Now()
    // Stats update
    langStats := make(map[string]int)
	for _, chunk := range newChunks {
		langStats[chunk.Language]++
	}
	idx.stats.Languages = langStats
	idx.mu.Unlock()

	return nil
}

func (idx *Indexer) cleanupDeletedFiles(currentFiles map[string]bool) (int, error) {
	dbFiles, err := idx.db.GetAllFilePaths()
	if err != nil {
		return 0, err
	}

	deletedCount := 0
	for _, dbPath := range dbFiles {
		if !currentFiles[dbPath] {
			if err := idx.db.DeleteFile(dbPath); err != nil {
				log.Printf("   Failed to delete %s: %v", filepath.Base(dbPath), err)
			} else {
				deletedCount++
			}
		}
	}
	return deletedCount, nil
}

func (idx *Indexer) processFile(ctx context.Context, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Sprache bestimmen:
	// 1. Schau in die Config Map (für saubere Namen wie "rust" statt ".rs")
	// 2. Fallback: Nutze einfach die Extension (z.B. "asm")
	ext := strings.ToLower(filepath.Ext(path))
	lang, known := idx.cfg.SupportedExts[ext]
	if !known {
		lang = strings.TrimPrefix(ext, ".") // .asm -> asm
        if lang == "" { lang = "txt" }      // Makefile, Dockerfile -> txt
	}

	// Parser holen: Holt SmartGenericParser für alles Unbekannte
	p := parser.GetParser(lang)

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	chunks, err := p.Parse(path, content, info.ModTime())
	if err != nil {
		return err
	}

    // Leere Chunks speichern um Re-Indexing Loop zu verhindern
    if len(chunks) == 0 {
        return idx.db.SaveFileChunks(path, info.ModTime(), []domain.CodeChunk{})
    }

	// Embeddings generieren
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		text := ""
		if c.Signature != "" { text += c.Signature + "\n" }
		if c.Comments != "" { text += c.Comments + "\n" }
		text += c.Content
		texts[i] = text
	}

	embeddings, err := idx.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return err
	}

	for i := range chunks {
		chunks[i].Embedding = embeddings[i]
	}

	return idx.db.SaveFileChunks(path, info.ModTime(), chunks)
}

// IterateChunks ermöglicht speicherschonende Iteration über alle Chunks
// Callback return false stoppt die Iteration
func (idx *Indexer) IterateChunks(fn func(domain.CodeChunk) bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	for _, chunk := range idx.chunks {
		if !fn(chunk) {
			break
		}
	}
}

func (idx *Indexer) Stats() domain.IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.stats
}

func (idx *Indexer) Watch(ctx context.Context, rootPath string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	idx.watcher = watcher

	// Rekursives Adden ist in fsnotify tricky,
	// wir adden erstmal nur root und reagieren auf Reindex.
	// Für echten Recursive Watcher müsste man hier durch alle Dirs loopen.
	// MVP Lösung: Watch Root und reindex bei Change.
	if err := idx.addWatchRecursive(rootPath); err != nil {
		return err
	}

	go func() {
		debounce := time.NewTimer(idx.cfg.WatchDebounce)
		debounce.Stop()
		needsReindex := false

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Ignoriere Events in .git etc.
				if idx.shouldIgnore(event.Name) {
					continue
				}

				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					needsReindex = true
					debounce.Reset(idx.cfg.WatchDebounce)
				}
			case <-debounce.C:
				if needsReindex {
					log.Println("File changes detected, reindexing...")
					if err := idx.BuildIndex(ctx, rootPath); err != nil {
						log.Printf("❌ Reindex failed: %v", err)
					}
					needsReindex = false
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()

	return nil
}

func (idx *Indexer) addWatchRecursive(path string) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil { return nil }
		if info.IsDir() {
			if idx.shouldIgnore(p) {
				return filepath.SkipDir
			}
			return idx.watcher.Add(p)
		}
		return nil
	})
}

func (idx *Indexer) Close() {
	if idx.watcher != nil {
		idx.watcher.Close()
	}
	if idx.db != nil {
		idx.db.Close()
	}
}

func (idx *Indexer) GetEmbedder() embedding.Embedder {
	return idx.embedder
}
