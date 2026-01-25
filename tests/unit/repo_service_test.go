package unit

import (
	"context"
	"errors"
	"testing"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/services"
	"coderefinery/tests/unit/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRepoService_Create_Success(t *testing.T) {
	mockStore := new(mocks.MockRepositoryStore)
	mockIndexer := new(mocks.MockIndexer)
	service := services.NewRepositoryService(mockStore, mockIndexer)

	ctx := context.Background()
	realTmpDir := t.TempDir()

	// 1. Synchroner Save (Status: Pending)
	mockStore.On("Save", ctx, mock.MatchedBy(func(r *domain.Repository) bool {
		return r.Name == "test-repo" && r.Status == domain.StatusPending
	})).Return(nil)

	// 2. Asynchroner Save (Status: Indexing)
	mockStore.On("Save", mock.Anything, mock.MatchedBy(func(r *domain.Repository) bool {
		return r.Status == domain.StatusIndexing
	})).Return(nil).Maybe()

	// 3. FIX: Asynchroner Save (Status: Ready)
	// Da der Indexer im Test "sofort" fertig ist (weil er gemocked ist),
	// ruft der Service am Ende Save mit StatusReady auf. Das müssen wir erlauben.
	mockStore.On("Save", mock.Anything, mock.MatchedBy(func(r *domain.Repository) bool {
		return r.Status == domain.StatusReady
	})).Return(nil).Maybe()

	// Indexer Trigger
	mockIndexer.On("Index", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Execute
	// KORREKTUR: 4. Argument 'isManaged' hinzugefügt (false für lokale Tests)
	repo, err := service.Create(ctx, "test-repo", realTmpDir, false)

	require.NoError(t, err)
	require.NotNil(t, repo)

	assert.Equal(t, "test-repo", repo.Name)
	assert.Equal(t, realTmpDir, repo.Path)
}

func TestRepoService_Delete_Success(t *testing.T) {
	mockStore := new(mocks.MockRepositoryStore)
	mockIndexer := new(mocks.MockIndexer)
	service := services.NewRepositoryService(mockStore, mockIndexer)
	ctx := context.Background()
	id := uuid.New()

	repo := &domain.Repository{ID: id, Name: "to-delete"}

	// 1. Find Repo
	mockStore.On("FindByID", ctx, id).Return(repo, nil)

	// 2. Delete from Vector DB
	mockIndexer.On("DeleteIndex", ctx, repo).Return(nil)

	// 3. Delete from SQL
	mockStore.On("Delete", ctx, id).Return(nil)

	err := service.Delete(ctx, id)
	assert.NoError(t, err)

	mockStore.AssertExpectations(t)
	mockIndexer.AssertExpectations(t)
}

func TestRepoService_Delete_NotFound(t *testing.T) {
	mockStore := new(mocks.MockRepositoryStore)
	mockIndexer := new(mocks.MockIndexer)
	service := services.NewRepositoryService(mockStore, mockIndexer)
	ctx := context.Background()
	id := uuid.New()

	mockStore.On("FindByID", ctx, id).Return(nil, errors.New("not found"))

	err := service.Delete(ctx, id)
	assert.Error(t, err)

	mockIndexer.AssertNotCalled(t, "DeleteIndex")
	mockStore.AssertNotCalled(t, "Delete", ctx, id)
}

func TestRepoService_Reindex_Success(t *testing.T) {
	mockStore := new(mocks.MockRepositoryStore)
	mockIndexer := new(mocks.MockIndexer)
	service := services.NewRepositoryService(mockStore, mockIndexer)
	ctx := context.Background()
	id := uuid.New()

	repo := &domain.Repository{ID: id, Status: domain.StatusReady}

	// 1. Find Repo
	mockStore.On("FindByID", ctx, id).Return(repo, nil)

	// 2. Synchroner Update (Status: Indexing)
	mockStore.On("Save", ctx, mock.MatchedBy(func(r *domain.Repository) bool {
		return r.ID == id && r.Status == domain.StatusIndexing
	})).Return(nil)

	// 3. Asynchroner Update durch 'runIndexing' (Status: Indexing)
	// Dieser Aufruf passiert in der Goroutine, evtl. parallel.
	mockStore.On("Save", mock.Anything, mock.MatchedBy(func(r *domain.Repository) bool {
		return r.Status == domain.StatusIndexing
	})).Return(nil).Maybe()

	// 4. Asynchroner Update am Ende (Status: Ready)
	mockStore.On("Save", mock.Anything, mock.MatchedBy(func(r *domain.Repository) bool {
		return r.Status == domain.StatusReady
	})).Return(nil).Maybe()

	// Trigger Index Async
	mockIndexer.On("Index", mock.Anything, repo).Return(nil).Maybe()

	err := service.Reindex(ctx, id)
	assert.NoError(t, err)

	// Wir prüfen hier nicht AssertExpectations, da asynchrone Calls "Maybe" sind
	// und der Test zu Ende sein könnte, bevor sie eintreffen.
	// Wichtig ist, dass keine unerwarteten Calls den Test crashen lassen.
}
