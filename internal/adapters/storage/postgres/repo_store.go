package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RepoStore struct {
	db *sqlx.DB
}

// NewRepoStore erstellt eine neue Instanz des Postgres-Stores
func NewRepoStore(db *sqlx.DB) ports.RepositoryStore {
	return &RepoStore{db: db}
}

// Save speichert ein Repository (Project) in der Datenbank
func (s *RepoStore) Save(ctx context.Context, repo *domain.Repository) error {
	// Wir mappen das 'domain.Repository' auf die 'projects' Tabelle
	// Hinweis: OwnerID ist hier hardcoded (Nil UUID), da das User-System noch nicht aktiv ist.
	query := `
	INSERT INTO projects (id, name, repository_url, owner_id, settings, updated_at)
	VALUES (:id, :name, :path, :owner_id, '{}', NOW())
	ON CONFLICT(owner_id, name) DO UPDATE SET
		repository_url = :path,
		updated_at = NOW()
	RETURNING id
	`

	// Temporäres Struct für Named Query
	type ProjectRow struct {
		ID      uuid.UUID `db:"id"`
		Name    string    `db:"name"`
		Path    string    `db:"path"`
		OwnerID uuid.UUID `db:"owner_id"`
	}

	// Default UUID für Owner (Single User Mode)
	ownerID := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	if repo.ID == uuid.Nil {
		repo.ID = uuid.New()
	}

	row := ProjectRow{
		ID:      repo.ID,
		Name:    repo.Name,
		Path:    repo.Path,
		OwnerID: ownerID,
	}

	rows, err := s.db.NamedQueryContext(ctx, query, row)
	if err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		var newID uuid.UUID
		if err := rows.Scan(&newID); err == nil {
			repo.ID = newID
		}
	}

	return nil
}

func (s *RepoStore) FindByID(ctx context.Context, id uuid.UUID) (*domain.Repository, error) {
	query := `SELECT id, name, repository_url as path FROM projects WHERE id = $1`

	var repo domain.Repository
	err := s.db.GetContext(ctx, &repo, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found")
		}
		return nil, err
	}
	// Status setzen (Feld existiert noch nicht in Projects Tabelle)
	repo.Status = domain.StatusReady
	return &repo, nil
}

func (s *RepoStore) FindAll(ctx context.Context) ([]*domain.Repository, error) {
	query := `SELECT id, name, repository_url as path FROM projects`

	var repos []*domain.Repository
	err := s.db.SelectContext(ctx, &repos, query)
	if err != nil {
		return nil, err
	}

	// Default-Werte setzen, um Nil-Pointer im Frontend zu vermeiden
	for _, r := range repos {
		r.Status = domain.StatusReady
	}

	return repos, nil
}

func (s *RepoStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM projects WHERE id = $1", id)
	return err
}
