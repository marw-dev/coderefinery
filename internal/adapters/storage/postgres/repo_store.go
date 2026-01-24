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

func NewRepoStore(db *sqlx.DB) ports.RepositoryStore {
	return &RepoStore{db: db}
}

func (s *RepoStore) Save(ctx context.Context, repo *domain.Repository) error {
	query := `
	INSERT INTO repositories (id, name, repository_url, owner_id, settings, status, updated_at)
	VALUES (:id, :name, :path, :owner_id, '{}', :status, NOW())
	ON CONFLICT(owner_id, name) DO UPDATE SET
		repository_url = :path,
		status = :status,
		updated_at = NOW()
	RETURNING id
	`

	type RepoRow struct {
		ID      uuid.UUID `db:"id"`
		Name    string    `db:"name"`
		Path    string    `db:"path"`
		OwnerID uuid.UUID `db:"owner_id"`
		Status  string    `db:"status"`
	}

	ownerID := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	if repo.ID == uuid.Nil {
		repo.ID = uuid.New()
	}

	// Status Fallback
	status := string(repo.Status)
	if status == "" {
		status = string(domain.StatusPending)
	}

	row := RepoRow{
		ID:      repo.ID,
		Name:    repo.Name,
		Path:    repo.Path,
		OwnerID: ownerID,
		Status:  status,
	}

	rows, err := s.db.NamedQueryContext(ctx, query, row)
	if err != nil {
		return fmt.Errorf("failed to save repository: %w", err)
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
	query := `SELECT id, name, repository_url as path, status FROM repositories WHERE id = $1`

	var repo domain.Repository
	err := s.db.GetContext(ctx, &repo, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found")
		}
		return nil, err
	}
	return &repo, nil
}

func (s *RepoStore) FindAll(ctx context.Context) ([]*domain.Repository, error) {
	query := `SELECT id, name, repository_url as path, status FROM repositories`

	var repos []*domain.Repository
	err := s.db.SelectContext(ctx, &repos, query)
	if err != nil {
		return nil, err
	}
	return repos, nil
}

func (s *RepoStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM repositories WHERE id = $1", id)
	return err
}
