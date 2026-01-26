package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	storageWeaviate "coderefinery/internal/adapters/storage/weaviate"
	"coderefinery/internal/core/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoStore_CRUD(t *testing.T) {
	// Clean slate
	cleanup()

	store, err := storageWeaviate.NewRepoStore(testWeaviate)
	require.NoError(t, err, "Failed to create repo store")

	ctx := context.Background()

	// === 1. CREATE ===
	repo := &domain.Repository{
		ID:          uuid.New(),
		Name:        "Test Repo",
		Path:        "/tmp/test-repo",
		Status:      domain.StatusPending,
		IsManaged:   true,
		FileCount:   0,
		ChunkCount:  0,
		CreatedAt:   time.Now().Truncate(time.Second),
		IndexConfig: map[string]any{"max_depth": 5},
		DefaultPipeline: map[string]any{
			"steps": []string{"parse", "chunk", "embed"},
		},
	}

	err = store.Save(ctx, repo)
	require.NoError(t, err, "Failed to save repository")

	// Weaviate eventual consistency
	time.Sleep(150 * time.Millisecond)

	// === 2. READ ===
	fetched, err := store.FindByID(ctx, repo.ID)
	require.NoError(t, err, "Failed to find repository")
	require.NotNil(t, fetched)

	assert.Equal(t, repo.ID, fetched.ID)
	assert.Equal(t, repo.Name, fetched.Name)
	assert.Equal(t, repo.Path, fetched.Path)
	assert.Equal(t, repo.Status, fetched.Status)
	assert.Equal(t, repo.IsManaged, fetched.IsManaged)
	assert.Equal(t, repo.FileCount, fetched.FileCount)
	assert.Equal(t, repo.ChunkCount, fetched.ChunkCount)

	// IndexConfig prüfen
	assert.NotNil(t, fetched.IndexConfig)
	if maxDepth, ok := fetched.IndexConfig["max_depth"].(float64); ok {
		assert.Equal(t, float64(5), maxDepth)
	}

	// === 3. LIST ===
	repos, err := store.FindAll(ctx)
	require.NoError(t, err, "Failed to list repositories")
	assert.Len(t, repos, 1, "Should have exactly one repository")
	assert.Equal(t, repo.ID, repos[0].ID)

	// === 4. UPDATE ===
	repo.Status = domain.StatusReady
	repo.FileCount = 42
	repo.ChunkCount = 156
	repo.ErrorMsg = ""
	repo.IndexConfig["processed"] = true

	err = store.Save(ctx, repo)
	require.NoError(t, err, "Failed to update repository")

	time.Sleep(150 * time.Millisecond)

	fetchedUpdated, err := store.FindByID(ctx, repo.ID)
	require.NoError(t, err, "Failed to find updated repository")
	require.NotNil(t, fetchedUpdated)

	assert.Equal(t, domain.StatusReady, fetchedUpdated.Status)
	assert.Equal(t, 42, fetchedUpdated.FileCount)
	assert.Equal(t, 156, fetchedUpdated.ChunkCount)

	// Verify IndexConfig update
	if processed, ok := fetchedUpdated.IndexConfig["processed"].(bool); ok {
		assert.True(t, processed)
	}

	// === 5. DELETE ===
	err = store.Delete(ctx, repo.ID)
	require.NoError(t, err, "Failed to delete repository")

	time.Sleep(150 * time.Millisecond)

	// Verify deletion
	_, err = store.FindByID(ctx, repo.ID)
	assert.Error(t, err, "Should return error for deleted repository")
	assert.True(t, errors.Is(err, storageWeaviate.ErrRepositoryNotFound),
		"Error should be ErrRepositoryNotFound")
}

func TestRepoStore_NotFound(t *testing.T) {
	cleanup()

	store, err := storageWeaviate.NewRepoStore(testWeaviate)
	require.NoError(t, err)

	ctx := context.Background()
	nonExistentID := uuid.New()

	_, err = store.FindByID(ctx, nonExistentID)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, storageWeaviate.ErrRepositoryNotFound))
}

func TestRepoStore_MultipleRepositories(t *testing.T) {
	cleanup()

	store, err := storageWeaviate.NewRepoStore(testWeaviate)
	require.NoError(t, err)

	ctx := context.Background()

	// Create multiple repositories
	repos := []*domain.Repository{
		{
			ID:        uuid.New(),
			Name:      "Repo 1",
			Path:      "/tmp/repo1",
			Status:    domain.StatusReady,
			IsManaged: true,
			CreatedAt: time.Now(),
		},
		{
			ID:        uuid.New(),
			Name:      "Repo 2",
			Path:      "/tmp/repo2",
			Status:    domain.StatusPending,
			IsManaged: false,
			CreatedAt: time.Now(),
		},
		{
			ID:        uuid.New(),
			Name:      "Repo 3",
			Path:      "/tmp/repo3",
			Status:    domain.StatusIndexing,
			IsManaged: true,
			CreatedAt: time.Now(),
		},
	}

	// Save all
	for _, repo := range repos {
		err = store.Save(ctx, repo)
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)

	// Find all
	allRepos, err := store.FindAll(ctx)
	require.NoError(t, err)
	assert.Len(t, allRepos, 3)

	// Verify each can be found individually
	for _, repo := range repos {
		found, err := store.FindByID(ctx, repo.ID)
		require.NoError(t, err)
		assert.Equal(t, repo.Name, found.Name)
		assert.Equal(t, repo.Status, found.Status)
	}
}

func TestRepoStore_UpdateIdempotency(t *testing.T) {
	cleanup()

	store, err := storageWeaviate.NewRepoStore(testWeaviate)
	require.NoError(t, err)

	ctx := context.Background()

	repo := &domain.Repository{
		ID:        uuid.New(),
		Name:      "Test Repo",
		Path:      "/tmp/test",
		Status:    domain.StatusPending,
		IsManaged: true,
		CreatedAt: time.Now(),
	}

	// Save multiple times with same ID
	for i := 0; i < 3; i++ {
		repo.FileCount = i * 10
		err = store.Save(ctx, repo)
		require.NoError(t, err, "Save %d should succeed", i)
	}

	time.Sleep(150 * time.Millisecond)

	// Verify final state
	fetched, err := store.FindByID(ctx, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, 20, fetched.FileCount) // Last update
}

func TestRepoStore_EmptyList(t *testing.T) {
	cleanup()

	store, err := storageWeaviate.NewRepoStore(testWeaviate)
	require.NoError(t, err)

	ctx := context.Background()

	repos, err := store.FindAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, repos)
}
