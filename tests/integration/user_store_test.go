package integration

import (
	"context"
	"testing"
	"time"

	"coderefinery/internal/adapters/storage/postgres"
	"coderefinery/internal/core/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserStore_SaveAndFind(t *testing.T) {
	cleanup()
	store := postgres.NewUserStore(testDB)
	ctx := context.Background()

	// 1. User erstellen (Ohne Email)
	newUser := &domain.User{
		ID:           uuid.New(),
		Username:     "tester",
		PasswordHash: "hashed_secret",
		Role:         domain.RoleViewer,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 2. Speichern
	err := store.Save(ctx, newUser)
	require.NoError(t, err, "Save should not fail")

	// 3. Finden per ID
	foundUser, err := store.FindByID(ctx, newUser.ID)
	require.NoError(t, err, "FindByID should not fail")

	assert.Equal(t, newUser.ID, foundUser.ID)
	assert.Equal(t, newUser.Username, foundUser.Username)
	assert.Equal(t, newUser.Role, foundUser.Role)

	// 4. Finden per Username
	foundByName, err := store.FindByUsername(ctx, "tester")
	require.NoError(t, err)
	assert.Equal(t, newUser.ID, foundByName.ID)
}

func TestUserStore_UpdateOnConflict(t *testing.T) {
	cleanup()
	store := postgres.NewUserStore(testDB)
	ctx := context.Background()

	user1 := &domain.User{
		ID:           uuid.New(),
		Username:     "duplicate_user",
		PasswordHash: "pass1",
		Role:         domain.RoleViewer,
		IsActive:     true,
	}

	err := store.Save(ctx, user1)
	require.NoError(t, err)

	// User 2 mit gleichem Username aber neuem Passwort/Rolle
	user2 := &domain.User{
		ID:           uuid.New(),
		Username:     "duplicate_user", // Konflikt!
		PasswordHash: "pass2_updated",
		Role:         domain.RoleAdmin, // Rolle geändert
		IsActive:     true,
	}

	// Save sollte Upsert machen (Username conflict)
	err = store.Save(ctx, user2)
	require.NoError(t, err, "Upsert should succeed")

	// Prüfen ob Daten aktualisiert wurden
	updatedUser, err := store.FindByUsername(ctx, "duplicate_user")
	require.NoError(t, err)
	assert.Equal(t, "pass2_updated", updatedUser.PasswordHash)
	assert.Equal(t, domain.RoleAdmin, updatedUser.Role)
}
