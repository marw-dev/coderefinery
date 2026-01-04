package indexer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"coderefinery/internal/domain"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func NewDB(path string) (*DB, error) {
    conn, err := sql.Open("sqlite3", path+"?cache=shared&mode=rwc")
    if err != nil {
        return nil, err
    }

    // WAL Mode aktivieren
    if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
        log.Printf("Warning: Could not enable WAL: %v", err)
    }

    db := &DB{conn: conn}

    // AUTOMATISCHE MIGRATION STARTEN
    if err := db.migrate(); err != nil {
        conn.Close()
        return nil, fmt.Errorf("database migration failed: %w", err)
    }

    return db, nil
}

func (db *DB) migrate() error {
    var currentVersion int
    // Aktuelle Version lesen
    err := db.conn.QueryRow("PRAGMA user_version").Scan(&currentVersion)
    if err != nil {
        return err
    }

    log.Printf("Current DB Version: %d, Target: %d", currentVersion, TargetVersion)

    if currentVersion >= TargetVersion {
        return nil // Alles aktuell
    }

    for _, m := range migrations {
        // Führe nur Migrationen aus, die neuer sind als die aktuelle DB-Version
        if m.Version > currentVersion {
            tx, err := db.conn.Begin()
            if err != nil {
                return err
            }

            if err := m.Apply(tx); err != nil {
                tx.Rollback()
                return err
            }

            // Version hochsetzen
            // WICHTIG: user_version kann nicht in Transaktion gesetzt werden in älteren SQLite Versionen,
            // aber wir machen es hier als Teil des Flows.
            if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.Version)); err != nil {
                tx.Rollback()
                return err
            }

            if err := tx.Commit(); err != nil {
                return err
            }
            log.Printf("✓ Migrated to version %d", m.Version)
            currentVersion = m.Version
        }
    }
    return nil
}

func (db *DB) SaveFileChunks(filePath string, modTime time.Time, chunks []domain.CodeChunk) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM chunks WHERE file_path = ?", filePath); err != nil {
		return err
	}

	// Statement mit 'imports' Spalte
	stmt, err := tx.Prepare(`
		INSERT INTO chunks (id, file_path, content, signature, comments,
			start_line, end_line, chunk_type, language, embedding, imports, last_modified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		embeddingJSON, _ := json.Marshal(chunk.Embedding)
		importsJSON, _ := json.Marshal(chunk.Imports) // Imports serialisieren

		if _, err := stmt.Exec(
			chunk.ID, chunk.FilePath, chunk.Content, chunk.Signature, chunk.Comments,
			chunk.StartLine, chunk.EndLine, chunk.ChunkType, chunk.Language,
			embeddingJSON, importsJSON, chunk.LastModified.Unix(),
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`
		INSERT OR REPLACE INTO file_metadata (file_path, last_modified, last_indexed, chunk_count)
		VALUES (?, ?, ?, ?)
	`, filePath, modTime.Unix(), time.Now().Unix(), len(chunks)); err != nil {
		return err
	}

	return tx.Commit()
}

func (db *DB) LoadAllChunks() (map[string]domain.CodeChunk, error) {
	// Select Query mit 'imports'
	rows, err := db.conn.Query(`
		SELECT id, file_path, content, signature, comments,
			start_line, end_line, chunk_type, language, embedding, imports, last_modified
		FROM chunks ORDER BY file_path, start_line
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make(map[string]domain.CodeChunk)
	for rows.Next() {
		var chunk domain.CodeChunk
		var embeddingJSON []byte
		var importsJSON []byte // Helper für SQL NULL/Text Handling
		var lastModUnix int64

		// Scan inklusive imports
		if err := rows.Scan(&chunk.ID, &chunk.FilePath, &chunk.Content, &chunk.Signature,
			&chunk.Comments, &chunk.StartLine, &chunk.EndLine, &chunk.ChunkType,
			&chunk.Language, &embeddingJSON, &importsJSON, &lastModUnix); err != nil {
			return nil, err
		}

		json.Unmarshal(embeddingJSON, &chunk.Embedding)
		if len(importsJSON) > 0 {
			json.Unmarshal(importsJSON, &chunk.Imports)
		}

		chunk.LastModified = time.Unix(lastModUnix, 0)
		chunks[chunk.ID] = chunk
	}

	return chunks, nil
}

func (db *DB) GetFileModTime(filePath string) (time.Time, bool) {
	var modTimeUnix int64
	err := db.conn.QueryRow(
		"SELECT last_modified FROM file_metadata WHERE file_path = ?", filePath,
	).Scan(&modTimeUnix)

	if err == sql.ErrNoRows {
		return time.Time{}, false
	}
	if err != nil {
		return time.Time{}, false
	}

	return time.Unix(modTimeUnix, 0), true
}

func (db *DB) GetAllFilePaths() ([]string, error) {
	rows, err := db.conn.Query("SELECT DISTINCT file_path FROM chunks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func (db *DB) DeleteFile(filePath string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM chunks WHERE file_path = ?", filePath); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM file_metadata WHERE file_path = ?", filePath); err != nil {
		return err
	}

	return tx.Commit()
}

func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}
