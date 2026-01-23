package sqlite

import (
	"context"
	"database/sql"

	"coderefinery/internal/core/domain"

	_ "github.com/glebarez/go-sqlite"
	"github.com/google/uuid"
)

type RepoStore struct {
	db *sql.DB
}

func NewRepoStore(db *sql.DB) *RepoStore {
	return &RepoStore{db: db}
}

// InitSchema erstellt die notwendige Tabelle, falls sie fehlt
func (s *RepoStore) InitSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS repositories (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		path TEXT NOT NULL,
		status TEXT NOT NULL,
		last_indexed DATETIME,
		created_at DATETIME,
		updated_at DATETIME,
		file_count INTEGER DEFAULT 0,
		chunk_count INTEGER DEFAULT 0,
		error_msg TEXT
	);
	`
	_, err := s.db.Exec(query)
	return err
}

func (s *RepoStore) Save(ctx context.Context, repo *domain.Repository) error {
	query := `
	INSERT INTO repositories (id, name, path, status, last_indexed, created_at, updated_at, file_count, chunk_count, error_msg)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		status = excluded.status,
		last_indexed = excluded.last_indexed,
		updated_at = excluded.updated_at,
		file_count = excluded.file_count,
		chunk_count = excluded.chunk_count,
		error_msg = excluded.error_msg;
	`

	_, err := s.db.ExecContext(ctx, query,
		repo.ID,
		repo.Name,
		repo.Path,
		repo.Status,
		repo.LastIndexed,
		repo.CreatedAt,
		repo.UpdatedAt,
		repo.FileCount,
		repo.ChunkCount,
		repo.ErrorMsg,
	)
	return err
}

func (s *RepoStore) FindByID(ctx context.Context, id uuid.UUID) (*domain.Repository, error) {
	query := `SELECT id, name, path, status, last_indexed, created_at, updated_at, file_count, chunk_count, error_msg FROM repositories WHERE id = ?`

	row := s.db.QueryRowContext(ctx, query, id)
	return s.scanRow(row)
}

func (s *RepoStore) FindAll(ctx context.Context) ([]*domain.Repository, error) {
	query := `SELECT id, name, path, status, last_indexed, created_at, updated_at, file_count, chunk_count, error_msg FROM repositories`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []*domain.Repository
	for rows.Next() {
		repo, err := s.scanRows(rows)
		if err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, nil
}

func (s *RepoStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM repositories WHERE id = ?", id)
	return err
}

// Helper für Scan (DRY)
func (s *RepoStore) scanRow(row *sql.Row) (*domain.Repository, error) {
	var repo domain.Repository
	var lastIndexed, createdAt, updatedAt sql.NullTime
	var errorMsg sql.NullString

	err := row.Scan(
		&repo.ID, &repo.Name, &repo.Path, &repo.Status,
		&lastIndexed, &createdAt, &updatedAt,
		&repo.FileCount, &repo.ChunkCount, &errorMsg,
	)
	if err != nil {
		return nil, err
	}

	repo.LastIndexed = lastIndexed.Time
	repo.CreatedAt = createdAt.Time
	repo.UpdatedAt = updatedAt.Time
	repo.ErrorMsg = errorMsg.String

	return &repo, nil
}

func (s *RepoStore) scanRows(rows *sql.Rows) (*domain.Repository, error) {
	var repo domain.Repository
	var lastIndexed, createdAt, updatedAt sql.NullTime
	var errorMsg sql.NullString

	err := rows.Scan(
		&repo.ID, &repo.Name, &repo.Path, &repo.Status,
		&lastIndexed, &createdAt, &updatedAt,
		&repo.FileCount, &repo.ChunkCount, &errorMsg,
	)
	if err != nil {
		return nil, err
	}

	repo.LastIndexed = lastIndexed.Time
	repo.CreatedAt = createdAt.Time
	repo.UpdatedAt = updatedAt.Time
	repo.ErrorMsg = errorMsg.String

	return &repo, nil
}
