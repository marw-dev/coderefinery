package mocks

import (
	"context"

	"coderefinery/internal/core/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockUserStore simuliert die Datenbank für Benutzer
type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) Save(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserStore) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserStore) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
