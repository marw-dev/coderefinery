package sqlite

import (
	"context"
	"database/sql"

	"coderefinery/internal/core/domain"

	"github.com/google/uuid"
)

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) InitSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at DATETIME
	);
	`
	_, err := s.db.Exec(query)
	return err
}

func (s *UserStore) Save(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, user.ID, user.Username, user.PasswordHash, user.Role, user.CreatedAt)
	return err
}

func (s *UserStore) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?", username)
	return s.scanUser(row)
}

func (s *UserStore) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?", id)
	return s.scanUser(row)
}

func (s *UserStore) scanUser(row *sql.Row) (*domain.User, error) {
	var user domain.User
	var roleStr string
	err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &roleStr, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	user.Role = domain.Role(roleStr)
	return &user, nil
}
