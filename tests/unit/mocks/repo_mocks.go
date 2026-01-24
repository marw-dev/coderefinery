package mocks

import (
	"coderefinery/internal/core/domain"
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockRepositoryStore simuliert die SQL-Datenbank für Repos
type MockRepositoryStore struct {
	mock.Mock
}

func (m *MockRepositoryStore) Save(ctx context.Context, repo *domain.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockRepositoryStore) FindByID(ctx context.Context, id uuid.UUID) (*domain.Repository, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Repository), args.Error(1)
}

func (m *MockRepositoryStore) FindAll(ctx context.Context) ([]*domain.Repository, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Repository), args.Error(1)
}

func (m *MockRepositoryStore) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockIndexer simuliert den Indexierungsprozess
type MockIndexer struct {
	mock.Mock
}

func (m *MockIndexer) Index(ctx context.Context, repo *domain.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockIndexer) DeleteIndex(ctx context.Context, repo *domain.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockIndexer) DeleteAllIndices(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
