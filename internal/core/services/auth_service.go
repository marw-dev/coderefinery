package services

import (
	"context"
	"errors"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"
	"coderefinery/internal/infrastructure/auth"

	"github.com/google/uuid"
)

type AuthService struct {
	store ports.UserStore
	jwt   *auth.JWTService
}

func NewAuthService(store ports.UserStore, jwt *auth.JWTService) *AuthService {
	return &AuthService{store: store, jwt: jwt}
}

// Login gibt jetzt auch das User-Objekt zurück
func (s *AuthService) Login(ctx context.Context, username, password string) (string, *domain.User, error) {
	user, err := s.store.FindByUsername(ctx, username)
	if err != nil {
		return "", nil, errors.New("invalid credentials")
	}

	if !auth.CheckPasswordHash(password, user.PasswordHash) {
		return "", nil, errors.New("invalid credentials")
	}

	token, err := s.jwt.GenerateToken(user)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

func (s *AuthService) Register(ctx context.Context, username, password string) (*domain.User, error) {
	// Prüfen ob User existiert
	if _, err := s.store.FindByUsername(ctx, username); err == nil {
		return nil, errors.New("username already taken")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Neuer User (Rolle Viewer als Default)
	user := domain.NewUser(username, hash, domain.RoleViewer)

	if err := s.store.Save(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Me(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.store.FindByID(ctx, id)
}
