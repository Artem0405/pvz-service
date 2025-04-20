package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors" // Добавили fmt для обертки ошибок
	"strings"
	"testing"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository"
	mocks "github.com/Artem0405/pvz-service/internal/repository/mocks" // Правильный импорт моков
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const testSecret = "test-secret-key-1234567890-for-testing-purpose" // Используем константу для тестов

// --- ДОБАВЛЕНО: Кастомные ошибки уровня сервиса ---
var (
	ErrAuthValidation            = errors.New("ошибка валидации данных аутентификации")
	ErrAuthInvalidCredentials    = errors.New("неверный email или пароль")
	ErrAuthTokenGeneration       = errors.New("ошибка генерации токена")
	ErrAuthTokenExpired          = errors.New("токен истек")
	ErrAuthTokenMalformed        = errors.New("некорректный формат токена")
	ErrAuthTokenInvalidSignature = errors.New("неверная подпись токена")
	ErrAuthTokenInvalid          = errors.New("невалидный токен")
)

// Helper function to set up AuthService with a mock repository
func setupAuthServiceTest(t *testing.T) (*AuthServiceImpl, *mocks.UserRepository) { // Возвращаем *mocks.UserRepository
	t.Helper()
	mockUserRepo := new(mocks.UserRepository) // Используем правильный тип мока
	// NewAuthService принимает UserRepository, а не UserRepoMock
	authService := NewAuthService(testSecret, mockUserRepo).(*AuthServiceImpl) // Приводим к *AuthServiceImpl, если нужно обращаться к неэкспортируемым полям (не нужно здесь)
	require.NotNil(t, authService)
	return authService, mockUserRepo
}

// --- Tests for NewAuthService ---
func TestNewAuthService(t *testing.T) {
	mockUserRepo := new(mocks.UserRepository) // Используем правильный тип мока

	t.Run("Success with valid secret", func(t *testing.T) {
		assert.NotPanics(t, func() {
			service := NewAuthService(testSecret, mockUserRepo) // Передаем мок UserRepository
			assert.NotNil(t, service)
			// Проверяем, что поле userRepo установлено (если нужно)
			// Для этого может потребоваться привести тип service.(type) или сделать поле экспортируемым
			// Проще проверить, что NewAuthService не возвращает nil и не падает
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

		// Настройка мока CreateUser
		mockUserRepo.On("CreateUser", mock.Anything, mock.MatchedBy(func(user domain.User) bool {
			if user.Email != email || user.Role != role {
				t.Logf("Mock Matcher: Email or Role mismatch. Got Email: %s, Role: %s. Want Email: %s, Role: %s", user.Email, user.Role, email, role)
				return false
			}
			// Проверяем, что пароль захэширован (достаточно проверить, что он не пустой и не равен исходному)
			if user.PasswordHash == "" || user.PasswordHash == password {
				t.Logf("Mock Matcher: PasswordHash is empty or equals original password")
				return false
			}
			// Можно добавить проверку самого хэша, но это излишне
			// err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
			// return err == nil
			return true
		})).Return(expectedUserID, nil).Once()

		createdUser, err := authService.Register(ctx, email, password, role)

		require.NoError(t, err)
		assert.Equal(t, expectedUserID, createdUser.ID)
		assert.Equal(t, email, createdUser.Email)
		assert.Equal(t, role, createdUser.Role)
		assert.Empty(t, createdUser.PasswordHash, "Password hash should not be returned by Register")

		mockUserRepo.AssertExpectations(t)
	})

	// --- Validation Error Tests ---
	validationTestCases := []struct {
		name        string
		email       string
		password    string
		role        string
		expectedErr error // Используем ошибки уровня сервиса
	}{
		{"Fail - Empty Email", "", "password123", domain.RoleEmployee, ErrAuthValidation},
		{"Fail - Empty Password", "test@example.com", "", domain.RoleEmployee, ErrAuthValidation},
		{"Fail - Invalid Role", "test@example.com", "password123", "admin", ErrAuthValidation}, // Ошибка валидации роли тоже ErrAuthValidation
	}

	for _, tc := range validationTestCases {
		t.Run(tc.name, func(t *testing.T) {
			authService, mockUserRepo := setupAuthServiceTest(t)
			_, err := authService.Register(ctx, tc.email, tc.password, tc.role)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.expectedErr) // Проверяем конкретную ошибку сервиса
			mockUserRepo.AssertNotCalled(t, "CreateUser", mock.Anything, mock.Anything)
		})
	}

	// --- Repository Error Tests ---
	t.Run("Fail - Duplicate Email", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)
		email := "duplicate@example.com"

		// Мок репозитория возвращает ошибку дубликата
		mockUserRepo.On("CreateUser", mock.Anything, mock.AnythingOfType("domain.User")).
			Return(uuid.Nil, repository.ErrUserDuplicateEmail).Once()

		_, err := authService.Register(ctx, email, "password123", domain.RoleModerator)

		require.Error(t, err)
		assert.ErrorIs(t, err, repository.ErrUserDuplicateEmail, "Should return specific duplicate email error") // Сервис должен пробрасывать эту ошибку репозитория

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
		assert.ErrorIs(t, err, repoErr, "Original repo error should be wrapped") // Проверяем, что оригинальная ошибка обернута

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

		// Мок репозитория возвращает пользователя
		mockUserRepo.On("GetUserByEmail", mock.Anything, email).Return(mockUser, nil).Once()

		tokenString, err := authService.Login(ctx, email, correctPassword)

		require.NoError(t, err)
		assert.NotEmpty(t, tokenString)

		// Проверяем валидность сгенерированного токена
		claims, err := authService.ValidateToken(tokenString) // Используем метод самого сервиса для валидации
		require.NoError(t, err)
		assert.Equal(t, userRole, claims.Role)

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Fail - User Not Found", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)

		// Мок репозитория возвращает ошибку "не найдено"
		mockUserRepo.On("GetUserByEmail", mock.Anything, email).
			Return(domain.User{}, repository.ErrUserNotFound).Once()

		_, err := authService.Login(ctx, email, correctPassword)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthInvalidCredentials) // Сервис должен вернуть ошибку неверных данных

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Fail - Repository Generic Error on GetUser", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)
		repoErr := errors.New("DB connection failed during select")

		mockUserRepo.On("GetUserByEmail", mock.Anything, email).
			Return(domain.User{}, repoErr).Once()

		_, err := authService.Login(ctx, email, correctPassword)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "ошибка входа")      // Проверяем общее сообщение
		assert.ErrorIs(t, err, repoErr)                      // Проверяем, что оригинальная ошибка обернута
		assert.NotErrorIs(t, err, ErrAuthInvalidCredentials) // Убедимся, что это не ошибка неверных данных

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Fail - Incorrect Password", func(t *testing.T) {
		authService, mockUserRepo := setupAuthServiceTest(t)

		// Мок репозитория находит пользователя
		mockUserRepo.On("GetUserByEmail", mock.Anything, email).Return(mockUser, nil).Once()

		// Пытаемся войти с неверным паролем
		_, err := authService.Login(ctx, email, "wrongPassword")

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthInvalidCredentials) // Сервис должен вернуть ошибку неверных данных

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

		// Парсим и проверяем клеймы
		claims := &Claims{}
		_, err = jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil // Используем тот же ключ, что и сервис
		})

		require.NoError(t, err)
		assert.Equal(t, role, claims.Role)
		// Проверяем стандартные клеймы
		assert.WithinDuration(t, time.Now().Add(24*time.Hour), claims.ExpiresAt.Time, 10*time.Second, "Expiration time is incorrect")
		assert.WithinDuration(t, time.Now(), claims.IssuedAt.Time, 10*time.Second, "IssuedAt time is incorrect")
		assert.Equal(t, "pvz-service", claims.Issuer) // Проверяем издателя
	})

	// Тест на ошибку генерации (сложно симулировать без изменения jwtKey)
	// Обычно покрывается тестами NewAuthService на пустой секрет
}

// --- Tests for ValidateToken ---
func TestAuthService_ValidateToken(t *testing.T) {
	authService, _ := setupAuthServiceTest(t)
	validRole := domain.RoleModerator

	// Генерируем валидный токен для тестов
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
		// Создаем токен с истекшим временем
		claimsExpired := &Claims{
			Role: validRole,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // Истек час назад
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				Issuer:    "pvz-service",
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsExpired)
		expiredTokenString, _ := token.SignedString(jwtKey) // Подписываем тем же ключом

		_, err := authService.ValidateToken(expiredTokenString)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthTokenExpired) // Проверяем кастомную ошибку
	})

	t.Run("Fail - Malformed Token (Not JWT)", func(t *testing.T) {
		_, err := authService.ValidateToken("not.a.jwt.token")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthTokenMalformed)
	})

	t.Run("Fail - Malformed Token (Bad Segment Encoding)", func(t *testing.T) {
		parts := strings.Split(validToken, ".")
		require.Len(t, parts, 3)
		malformedToken := parts[0] + ".%%%%%%%." + parts[2] // Невалидная средняя часть
		_, err := authService.ValidateToken(malformedToken)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthTokenMalformed)
	})

	t.Run("Fail - Invalid Signature", func(t *testing.T) {
		// Генерируем токен с другим секретом
		otherSecret := []byte("different-secret-key-0987654321-for-test")
		tokenWrongSig := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
			Role: validRole,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				Issuer:    "pvz-service",
			},
		})
		tokenStringWrongSig, _ := tokenWrongSig.SignedString(otherSecret)

		_, err := authService.ValidateToken(tokenStringWrongSig) // Валидируем с правильным ключом
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthTokenInvalidSignature)
	})

	t.Run("Fail - Wrong Signing Method (alg mismatch)", func(t *testing.T) {
		// Создаем токен с заголовком "alg":"none"
		header := `{"alg":"none","typ":"JWT"}`
		payloadClaims := &Claims{
			Role: validRole,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				Issuer:    "pvz-service",
			},
		}
		payloadBytes, _ := json.Marshal(payloadClaims)
		tokenStringWrongAlg := base64.RawURLEncoding.EncodeToString([]byte(header)) + "." +
			base64.RawURLEncoding.EncodeToString(payloadBytes) + "." // Пустая подпись

		_, err := authService.ValidateToken(tokenStringWrongAlg)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAuthTokenInvalid, "Should return invalid token error") // Общая ошибка невалидности
		assert.Contains(t, err.Error(), "неожиданный метод подписи", "Should contain specific message")
	})

	t.Run("Fail - Token Valid Flag is False (Difficult to simulate)", func(t *testing.T) {
		// Этот случай сложно воспроизвести изолированно, так как библиотека jwt
		// обычно возвращает более конкретные ошибки парсинга или валидации клеймов до этой проверки.
		// Если другие проверки проходят, токен обычно считается валидным.
		t.Skip("Skipping test for !token.Valid case as it's hard to trigger reliably without other parsing errors")
	})
}
