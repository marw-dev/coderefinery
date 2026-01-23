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

type UserStore struct {
	db *sqlx.DB
}

func NewUserStore(db *sqlx.DB) ports.UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) Save(ctx context.Context, user *domain.User) error {
	query := `
	INSERT INTO users (id, username, password_hash, role, is_active, last_login, created_at, updated_at)
	VALUES (:id, :username, :password_hash, :role, :is_active, :last_login, :created_at, NOW())
	ON CONFLICT(username) DO UPDATE SET
		password_hash = :password_hash,
		role = :role,
		last_login = :last_login,
		updated_at = NOW()
	RETURNING id
	`

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	rows, err := s.db.NamedQueryContext(ctx, query, user)
	if err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		var newID uuid.UUID
		if err := rows.Scan(&newID); err == nil {
			user.ID = newID
		}
	}
	return nil
}

func (s *UserStore) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := `SELECT * FROM users WHERE username = $1`
	var user domain.User
	err := s.db.GetContext(ctx, &user, query, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `SELECT * FROM users WHERE id = $1`
	var user domain.User
	err := s.db.GetContext(ctx, &user, query, id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
