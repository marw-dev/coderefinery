package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/services"
	"coderefinery/internal/infrastructure/auth"
	"coderefinery/tests/unit/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthService_Register(t *testing.T) {
	// Setup
	mockStore := new(mocks.UserStoreMock)
	jwtService := auth.NewJWTService("secret_key_for_testing", time.Hour)

	service := services.NewAuthService(mockStore, jwtService)
	ctx := context.Background()

	// WICHTIG: Das hat gefehlt!
	// Register prüft erst, ob der User schon existiert.
	// Wir müssen simulieren, dass er NICHT gefunden wurde (error != nil), damit es weitergeht.
	mockStore.On("FindByUsername", ctx, "newuser").Return(nil, errors.New("user not found"))

	// Erwartung: Save wird danach aufgerufen
	mockStore.On("Save", ctx, mock.MatchedBy(func(u *domain.User) bool {
		// Wir prüfen, ob das Passwort gehasht wurde (ungleich Klartext)
		return u.Username == "newuser" && u.PasswordHash != "password123"
	})).Return(nil)

	// Test
	user, err := service.Register(ctx, "newuser", "password123")

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "newuser", user.Username)

	// Sicherstellen, dass alle Mocks aufgerufen wurden
	mockStore.AssertExpectations(t)
}

func TestAuthService_Login_Success(t *testing.T) {
	mockStore := new(mocks.UserStoreMock)
	jwtService := auth.NewJWTService("secret_key_for_testing", time.Hour)
	service := services.NewAuthService(mockStore, jwtService)
	ctx := context.Background()

	hashed, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	dbUser := domain.NewUser("existing", string(hashed), domain.RoleViewer)

	mockStore.On("FindByUsername", ctx, "existing").Return(dbUser, nil)

	// Update: Erwarten jetzt Token UND User
	token, user, err := service.Login(ctx, "existing", "secret")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.NotNil(t, user)
	assert.Equal(t, "existing", user.Username)
	mockStore.AssertExpectations(t)
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	mockStore := new(mocks.UserStoreMock)
	jwtService := auth.NewJWTService("secret", time.Hour)
	service := services.NewAuthService(mockStore, jwtService)
	ctx := context.Background()

	hashed, _ := bcrypt.GenerateFromPassword([]byte("correct_password"), bcrypt.DefaultCost)
	dbUser := domain.NewUser("hacker", string(hashed), domain.RoleViewer)

	mockStore.On("FindByUsername", ctx, "hacker").Return(dbUser, nil)

	// Update: Rückgabewerte angepasst
	token, user, err := service.Login(ctx, "hacker", "wrong_guess")

	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "invalid credentials")

	mockStore.AssertExpectations(t)
}
