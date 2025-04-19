package service

import (
	"context"
	"encoding/base64" // Needed for manipulating JWT header/payload manually
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository"
	mocks "github.com/Artem0405/pvz-service/internal/repository/mocks" // Correct import for mocks
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const testSecret = "test-secret-key-1234567890-for-testing-purpose" // Use a constant secret for tests

// Helper function to set up AuthService with a mock repository
func setupAuthServiceTest(t *testing.T) (*AuthServiceImpl, *mocks.UserRepoMock) {
	t.Helper()
	mockUserRepo := new(mocks.UserRepoMock) // Use correct mock type name if generated differently
	authService := NewAuthService(testSecret, mockUserRepo)
	require.NotNil(t, authService)
	return authService, mockUserRepo
}

// --- Tests for NewAuthService ---
func TestNewAuthService(t *testing.T) {
	mockUserRepo := new(mocks.UserRepoMock)

	t.Run("Success with valid secret", func(t *testing.T) {
		assert.NotPanics(t, func() {
			service := NewAuthService(testSecret, mockUserRepo)
			assert.NotNil(t, service)
			assert.NotNil(t, service.userRepo, "userRepo should be initialized")
			assert.Equal(t, mockUserRepo, service.userRepo)
		})
	})

	t.Run("Panic on empty secret", func(t *testing.T) {
		assert.PanicsWithValue(t, "JWT_SECRET не может быть пустым", func() {
			NewAuthService("", mockUserRepo)
		}, "Should panic when JWT secret is empty")
	})
}

// --- Tests for Register ---
func TestAuthService_Register(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)
		email := "register.success@example.com"
		password := "password123"
		role := domain.RoleEmployee
		expectedUserID := uuid.New()

		mockUserRepo.On("CreateUser", mock.Anything, mock.MatchedBy(func(user domain.User) bool {
			if user.Email != email || user.Role != role {
				return false
			}
			err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
			return err == nil
		})).Return(expectedUserID, nil).Once()

		createdUser, err := authService.Register(ctx, email, password, role)

		require.NoError(t, err)
		assert.Equal(t, expectedUserID, createdUser.ID)
		assert.Equal(t, email, createdUser.Email)
		assert.Equal(t, role, createdUser.Role)
		assert.Empty(t, createdUser.PasswordHash, "Password hash should not be returned")

		mockUserRepo.AssertExpectations(t)
	})

	// --- Validation Error Tests ---
	validationTestCases := []struct {
		name        string
		email       string
		password    string
		role        string
		expectedErr error // Use error variable for checking
	}{
		{"Fail - Empty Email", "", "password123", domain.RoleEmployee, domain.ErrAuthValidation},
		{"Fail - Empty Password", "test@example.com", "", domain.RoleEmployee, domain.ErrAuthValidation},
		{"Fail - Empty Email and Password", "", "", domain.RoleEmployee, domain.ErrAuthValidation},
		{"Fail - Invalid Role", "test@example.com", "password123", "admin", errors.New("недопустимая роль пользователя")}, // Specific error message for role
	}

	for _, tc := range validationTestCases {
		t.Run(tc.name, func(t *testing.T) {
			authService, mockUserRepo := setupAuthServiceTest(t)
			_, err := authService.Register(ctx, tc.email, tc.password, tc.role)
			require.Error(t, err)
			if errors.Is(tc.expectedErr, domain.ErrAuthValidation) { // Check specifically for validation error
				assert.ErrorIs(t, err, domain.ErrAuthValidation)
			} else {
				assert.Contains(t, err.Error(), tc.expectedErr.Error()) // Check for contains for other messages
			}
			mockUserRepo.AssertNotCalled(t, "CreateUser", mock.Anything, mock.Anything)
		})
	}

	// --- Repository Error Tests ---
	t.Run("Fail - Duplicate Email", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)
		email := "duplicate@example.com"

		mockUserRepo.On("CreateUser", mock.Anything, mock.AnythingOfType("domain.User")).
			Return(uuid.Nil, repository.ErrUserDuplicateEmail).Once()

		_, err := authService.Register(ctx, email, "password123", domain.RoleModerator)

		require.Error(t, err)
		assert.ErrorIs(t, err, repository.ErrUserDuplicateEmail, "Should return specific duplicate email error")

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Fail - Repository Generic Error on CreateUser", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)
		repoErr := errors.New("DB connection failed during insert")

		mockUserRepo.On("CreateUser", mock.Anything, mock.AnythingOfType("domain.User")).
			Return(uuid.Nil, repoErr).Once()

		_, err := authService.Register(ctx, "test.repo.fail@example.com", "password123", domain.RoleEmployee)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "не удалось зарегистрировать пользователя", "Should return wrapped generic error")
		// !!! ИСПРАВЛЕНО: Проверяем, что ошибка обернута !!!
		assert.ErrorIs(t, err, repoErr, "Original repo error should be wrapped") // Check if wrapped

		mockUserRepo.AssertExpectations(t)
	})
}

// --- Tests for Login ---
func TestAuthService_Login(t *testing.T) {
	ctx := context.Background()
	email := "login.test@example.com"
	correctPassword := "correctPassword123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(correctPassword), bcrypt.DefaultCost)
	userID := uuid.New()
	userRole := domain.RoleModerator

	mockUser := domain.User{
		ID:           userID,
		Email:        email,
		PasswordHash: string(hashedPassword),
		Role:         userRole,
	}

	t.Run("Success", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)

		mockUserRepo.On("GetUserByEmail", mock.Anything, email).Return(mockUser, nil).Once()

		tokenString, err := authService.Login(ctx, email, correctPassword)

		require.NoError(t, err)
		assert.NotEmpty(t, tokenString)

		claims, err := authService.ValidateToken(tokenString)
		require.NoError(t, err)
		assert.Equal(t, userRole, claims.Role)

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Fail - User Not Found", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)

		mockUserRepo.On("GetUserByEmail", mock.Anything, email).
			Return(domain.User{}, repository.ErrUserNotFound).Once()

		_, err := authService.Login(ctx, email, correctPassword)

		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrAuthInvalidCredentials)

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Fail - Repository Generic Error on GetUser", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)
		repoErr := errors.New("DB connection failed during select")

		mockUserRepo.On("GetUserByEmail", mock.Anything, email).
			Return(domain.User{}, repoErr).Once()

		_, err := authService.Login(ctx, email, correctPassword)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "ошибка входа")
		// !!! ИСПРАВЛЕНО: Проверяем, что ошибка обернута !!!
		assert.ErrorIs(t, err, repoErr) // Ensure original error is wrapped
		assert.NotErrorIs(t, err, domain.ErrAuthInvalidCredentials)

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Fail - Incorrect Password", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)

		mockUserRepo.On("GetUserByEmail", mock.Anything, email).Return(mockUser, nil).Once()

		_, err := authService.Login(ctx, email, "wrongPassword")

		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrAuthInvalidCredentials)

		mockUserRepo.AssertExpectations(t)
	})
}

// --- Tests for GenerateToken ---
func TestAuthService_GenerateToken(t *testing.T) {
	authService, _ := setupAuthServiceTest(t)

	t.Run("Success", func(t *testing.T) {
		role := domain.RoleEmployee
		tokenString, err := authService.GenerateToken(role)
		require.NoError(t, err)
		require.NotEmpty(t, tokenString)

		claims, err := authService.ValidateToken(tokenString)
		require.NoError(t, err)
		assert.Equal(t, role, claims.Role)
		assert.WithinDuration(t, time.Now().Add(24*time.Hour), claims.ExpiresAt.Time, 10*time.Second)
		assert.Equal(t, "pvz-service", claims.Issuer)
	})
}

// --- Tests for ValidateToken ---
func TestAuthService_ValidateToken(t *testing.T) {
	authService, _ := setupAuthServiceTest(t)
	validRole := domain.RoleModerator

	validToken, err := authService.GenerateToken(validRole)
	require.NoError(t, err)
	require.NotEmpty(t, validToken)

	t.Run("Success - Valid Token", func(t *testing.T) {
		claims, err := authService.ValidateToken(validToken)
		require.NoError(t, err)
		require.NotNil(t, claims)
		assert.Equal(t, validRole, claims.Role)
	})

	t.Run("Fail - Expired Token", func(t *testing.T) {
		claimsExpired := &Claims{
			Role: validRole,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				Issuer:    "pvz-service",
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsExpired)
		expiredTokenString, _ := token.SignedString(jwtKey)

		_, err := authService.ValidateToken(expiredTokenString)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrAuthTokenExpired)
	})

	t.Run("Fail - Malformed Token (Not JWT)", func(t *testing.T) {
		_, err := authService.ValidateToken("not.a.jwt.token")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrAuthTokenMalformed)
	})

	t.Run("Fail - Malformed Token (Bad Segment Encoding)", func(t *testing.T) {
		parts := strings.Split(validToken, ".")
		require.Len(t, parts, 3)
		malformedToken := parts[0] + ".%%%%%%%." + parts[2]
		_, err := authService.ValidateToken(malformedToken)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrAuthTokenMalformed)
	})

	t.Run("Fail - Invalid Signature", func(t *testing.T) {
		tamperedToken := validToken[:len(validToken)-3] + "abc"

		_, err := authService.ValidateToken(tamperedToken)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrAuthTokenInvalidSignature)
	})

	t.Run("Fail - Wrong Signing Method (alg mismatch)", func(t *testing.T) {
		// Create parts manually to ensure 'none' algorithm in header
		header := `{"alg":"none","typ":"JWT"}`
		payloadClaims := &Claims{
			Role: validRole,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				Issuer:    "pvz-service",
			},
		}
		payloadBytes, _ := json.Marshal(payloadClaims)
		// Manually construct token with alg:none (no signature)
		tokenStringWrongAlg := base64.RawURLEncoding.EncodeToString([]byte(header)) + "." +
			base64.RawURLEncoding.EncodeToString(payloadBytes) + "." // Empty signature part

		_, err := authService.ValidateToken(tokenStringWrongAlg)
		require.Error(t, err)
		// !!! ИСПРАВЛЕНО: Проверяем, что ошибка обернута !!!
		assert.ErrorIs(t, err, domain.ErrAuthTokenInvalid, "Should return wrapped invalid token error")
		assert.Contains(t, err.Error(), "неожиданный метод подписи", "Wrapped error should contain original message")
	})

	t.Run("Fail - Token Valid Flag is False (Hard to trigger)", func(t *testing.T) {
		t.Skip("Skipping test for !token.Valid case as it's hard to trigger reliably without other parsing errors")
	})
}
