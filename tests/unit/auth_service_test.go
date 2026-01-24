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

func TestAuthService_Login_Success(t *testing.T) {
	mockUserStore := new(mocks.MockUserStore) // Du musst diese Datei ggf. anpassen auf ports.UserStore
	jwtService := auth.NewJWTService("secret-key", 1*time.Hour)

	service := services.NewAuthService(mockUserStore, jwtService)
	ctx := context.Background()

	password := "secure123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	user := &domain.User{
		Username:     "testuser",
		PasswordHash: string(hash),
	}

	mockUserStore.On("FindByUsername", ctx, "testuser").Return(user, nil)

	token, returnedUser, err := service.Login(ctx, "testuser", password)

	assert.NoError(t, err)
	assert.NotNil(t, returnedUser)
	assert.NotEmpty(t, token)
	assert.Equal(t, "testuser", returnedUser.Username)
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	mockUserStore := new(mocks.MockUserStore)
	jwtService := auth.NewJWTService("secret", 1*time.Hour)
	service := services.NewAuthService(mockUserStore, jwtService)
	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.DefaultCost)
	user := &domain.User{Username: "user", PasswordHash: string(hash)}

	mockUserStore.On("FindByUsername", ctx, "user").Return(user, nil)

	_, _, err := service.Login(ctx, "user", "wrong")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials") // oder spezifische message
}

func TestAuthService_Register_Success(t *testing.T) {
	mockUserStore := new(mocks.MockUserStore)
	jwtService := auth.NewJWTService("secret", 1*time.Hour)
	service := services.NewAuthService(mockUserStore, jwtService)
	ctx := context.Background()

	// 1. Expect Check if User Exists
	mockUserStore.On("FindByUsername", ctx, "newuser").Return(nil, errors.New("user not found"))

	// 2. Expect Save with hashed password
	mockUserStore.On("Save", ctx, mock.MatchedBy(func(u *domain.User) bool {
		// Prüfung: Username stimmt, Passwort ist gehasht (nicht mehr "plain")
		return u.Username == "newuser" && u.PasswordHash != "plain" && len(u.PasswordHash) > 10
	})).Return(nil)

	user, err := service.Register(ctx, "newuser", "plain")

	assert.NoError(t, err)
	assert.Equal(t, "newuser", user.Username)

	// Stellt sicher, dass alle erwarteten Methoden aufgerufen wurden
	mockUserStore.AssertExpectations(t)
}
