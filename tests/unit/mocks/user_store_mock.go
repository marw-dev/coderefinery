package mocks

import (
	"coderefinery/internal/core/domain"
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// UserStoreMock simuliert die Datenbank
type UserStoreMock struct {
	mock.Mock
}

func (m *UserStoreMock) Save(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *UserStoreMock) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	args := m.Called(ctx, username)
	// Typ-Assertion prüfen, da Return auch nil sein kann
	if user, ok := args.Get(0).(*domain.User); ok {
		return user, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *UserStoreMock) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if user, ok := args.Get(0).(*domain.User); ok {
		return user, args.Error(1)
	}
	return nil, args.Error(1)
}
