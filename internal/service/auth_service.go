package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository" // Убедитесь, что интерфейс UserRepository и константы ошибок здесь
	"github.com/golang-jwt/jwt/v5"                         // Импорт пакета JWT v5
	"golang.org/x/crypto/bcrypt"                           // Импорт пакета bcrypt
)

// Claims определяет структуру полезной нагрузки (payload) JWT токена.
type Claims struct {
	Role string `json:"role"`
	// Можно добавить другие поля, например UserID
	jwt.RegisteredClaims // Встраиваем стандартные RegisteredClaims (exp, iat, iss, etc.)
}

// jwtKey хранит секретный ключ для подписи и проверки JWT токенов.
// Инициализируется в конструкторе NewAuthService.
var jwtKey []byte

// AuthServiceImpl реализует логику сервиса аутентификации.
type AuthServiceImpl struct {
	userRepo repository.UserRepository // Зависимость от репозитория пользователей
}

// AuthService определяет интерфейс для сервиса аутентификации (если он нужен).
// Хорошая практика - определить интерфейс, но для исправления текущих ошибок это не обязательно.
// type AuthService interface {
//  Register(ctx context.Context, email, password, role string) (domain.User, error)
//  Login(ctx context.Context, email, password string) (string, error)
//  GenerateToken(role string) (string, error)
//  ValidateToken(tokenString string) (*Claims, error)
// }

// NewAuthService - конструктор для AuthServiceImpl.
// Принимает секретный ключ JWT и репозиторий пользователей.
func NewAuthService(secret string, userRepo repository.UserRepository) *AuthServiceImpl {
	if secret == "" {
		// Паника при старте, если не задан секрет - это критическая ошибка конфигурации.
		panic("JWT_SECRET не может быть пустым")
	}
	jwtKey = []byte(secret) // Инициализируем глобальный ключ
	return &AuthServiceImpl{
		userRepo: userRepo,
	}
}

// Register обрабатывает регистрацию нового пользователя.
func (s *AuthServiceImpl) Register(ctx context.Context, email, password, role string) (domain.User, error) {
	// 1. Валидация входных данных
	if email == "" || password == "" {
		// Возвращаем конкретную ошибку для невалидного ввода
		return domain.User{}, domain.ErrAuthValidation // Пример использования доменной ошибки
	}
	if role != domain.RoleEmployee && role != domain.RoleModerator {
		return domain.User{}, fmt.Errorf("недопустимая роль пользователя: %s", role)
	}
	// TODO: Добавить более строгую валидацию формата email и сложности пароля.

	// 2. Хеширование пароля
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка хеширования пароля при регистрации", "email", email, "error", err)
		// Не раскрываем внутренние детали ошибки пользователю
		return domain.User{}, fmt.Errorf("внутренняя ошибка сервера")
	}

	// 3. Создание пользователя в репозитории
	newUser := domain.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
		Role:         role,
		// ID будет присвоен базой данных или сгенерирован в CreateUser
	}
	userID, err := s.userRepo.CreateUser(ctx, newUser)
	if err != nil {
		// Проверяем на конкретную ошибку дубликата
		if errors.Is(err, repository.ErrUserDuplicateEmail) {
			slog.WarnContext(ctx, "Попытка регистрации с существующим email", "email", email)
			// Возвращаем специфическую ошибку, которую может обработать хендлер (например, для статуса 409)
			return domain.User{}, repository.ErrUserDuplicateEmail
		}
		// Логируем любую другую ошибку репозитория
		slog.ErrorContext(ctx, "Ошибка создания пользователя в репозитории", "email", email, "error", err)
		// Возвращаем обернутую ошибку
		return domain.User{}, fmt.Errorf("не удалось зарегистрировать пользователя: %w", err)
	}

	// 4. Успешная регистрация - возвращаем данные пользователя (без хеша пароля!)
	createdUser := domain.User{
		ID:    userID,
		Email: email,
		Role:  role,
	}
	slog.InfoContext(ctx, "Пользователь успешно зарегистрирован", "user_id", userID, "email", email)
	return createdUser, nil
}

// Login обрабатывает вход пользователя и возвращает JWT токен.
func (s *AuthServiceImpl) Login(ctx context.Context, email, password string) (string, error) {
	// 1. Получаем пользователя из репозитория по email
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		// Если пользователь не найден, возвращаем общую ошибку (защита от перебора)
		if errors.Is(err, repository.ErrUserNotFound) {
			slog.WarnContext(ctx, "Попытка входа несуществующего пользователя", "email", email)
			return "", domain.ErrAuthInvalidCredentials
		}
		// Логируем любую другую ошибку репозитория
		slog.ErrorContext(ctx, "Ошибка получения пользователя по email при логине", "email", email, "error", err)
		// Возвращаем обернутую ошибку
		return "", fmt.Errorf("ошибка входа: %w", err)
	}

	// 2. Сравниваем предоставленный пароль с хешем из БД
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		// Если хеши не совпадают (bcrypt.ErrMismatchedHashAndPassword) или другая ошибка bcrypt
		slog.WarnContext(ctx, "Неудачная попытка входа (неверный пароль)", "email", email)
		// Возвращаем ту же общую ошибку (защита от перебора)
		return "", domain.ErrAuthInvalidCredentials
	}

	// 3. Пароль верный - генерируем JWT токен
	tokenString, err := s.GenerateToken(user.Role) // Используем роль пользователя из БД
	if err != nil {
		// Ошибка генерации токена уже логируется внутри GenerateToken
		// Оборачиваем ошибку для контекста
		return "", fmt.Errorf("не удалось сгенерировать токен: %w", err)
	}

	slog.InfoContext(ctx, "Пользователь успешно вошел в систему", "user_id", user.ID, "email", email)
	return tokenString, nil
}

// GenerateToken генерирует новый JWT токен для указанной роли.
func (s *AuthServiceImpl) GenerateToken(role string) (string, error) {
	// Устанавливаем срок действия токена (например, 24 часа)
	expirationTime := time.Now().Add(24 * time.Hour)
	// Создаем полезную нагрузку (claims)
	claims := &Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			// Устанавливаем стандартные claims
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "pvz-service", // Опционально: указываем издателя
		},
	}

	// Создаем новый токен с указанием метода подписи и claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Подписываем токен секретным ключом
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		slog.Error("Ошибка подписи JWT токена", "error", err)
		// Возвращаем общую ошибку сервера
		return "", fmt.Errorf("внутренняя ошибка сервера при генерации токена")
	}

	return tokenString, nil
}

// ValidateToken проверяет подпись и срок действия JWT токена.
// Возвращает claims токена в случае успеха.
func (s *AuthServiceImpl) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	// Парсим токен, проверяя подпись и claims
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Валидация метода подписи: убеждаемся, что это HMAC, а не 'none' или другой
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
		}
		// Возвращаем секретный ключ для проверки подписи
		return jwtKey, nil
	})

	// Обработка ошибок парсинга и валидации
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			slog.Debug("Ошибка валидации токена: токен истек")
			return nil, domain.ErrAuthTokenExpired // Возвращаем доменную ошибку
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			slog.Debug("Ошибка валидации токена: некорректный формат")
			return nil, domain.ErrAuthTokenMalformed
		}
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			slog.Warn("Ошибка валидации токена: неверная подпись")
			return nil, domain.ErrAuthTokenInvalidSignature
		}
		// Логируем другие, менее ожидаемые ошибки парсинга
		slog.Error("Неожиданная ошибка парсинга JWT", "error", err)
		// Возвращаем общую ошибку невалидного токена
		return nil, fmt.Errorf("%w: %v", domain.ErrAuthTokenInvalid, err)
	}

	// Дополнительная проверка флага Valid (хотя ParseWithClaims обычно это делает)
	if !token.Valid {
		slog.Warn("Токен прошел парсинг, но помечен как невалидный")
		return nil, domain.ErrAuthTokenInvalid
	}

	// Токен успешно прошел все проверки
	return claims, nil
}
