package integration

import (
	"context"
	"testing"

	"coderefinery/internal/adapters/storage/postgres"
	"coderefinery/internal/core/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoStore_CRUD(t *testing.T) {
	cleanup()
	store := postgres.NewRepoStore(testDB)
	ctx := context.Background()

	// 1. Create
	repo := &domain.Repository{
		ID:   uuid.New(),
		Name: "Test Repo",
		Path: "/tmp/test-repo",
	}

	err := store.Save(ctx, repo)
	require.NoError(t, err)

	// 2. Read
	fetched, err := store.FindByID(ctx, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, repo.Name, fetched.Name)
	assert.Equal(t, repo.Path, fetched.Path)

	// 3. List
	repos, err := store.FindAll(ctx)
	require.NoError(t, err)
	assert.Len(t, repos, 1)

	// 4. Delete
	err = store.Delete(ctx, repo.ID)
	require.NoError(t, err)

	// Verify Delete
	_, err = store.FindByID(ctx, repo.ID)
	assert.Error(t, err)
}
