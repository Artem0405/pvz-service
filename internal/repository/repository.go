package repository

import (
	"context" // Используем context для передачи сигналов отмены/таймаутов
	"database/sql"
	"errors"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/google/uuid"
	// --- Добавьте этот импорт, если его нет, он может понадобиться мокам ---
	// "github.com/stretchr/testify/mock" // Не обязательно для самого интерфейса, но часто нужен рядом
)

// --- Стандартные ошибки репозитория ---
var ErrReceptionNotFound = sql.ErrNoRows                                      // Используем стандартную ошибку для "не найдено" для приемки
var ErrProductNotFound = sql.ErrNoRows                                        // Используем стандартную ошибку для "не найдено" для товара
var ErrUserNotFound = errors.New("user not found")                            // Кастомная ошибка для пользователя
var ErrUserDuplicateEmail = errors.New("user with this email already exists") // Кастомная ошибка дубликата email

// --- Интерфейсы Репозиториев ---

// PVZRepository определяет методы для работы с хранилищем ПВЗ
//
//go:generate mockery --name PVZRepository --output ./mocks --outpkg mocks --case underscore --filename pvz_repo_mock.go
type PVZRepository interface {
	// CreatePVZ сохраняет новый ПВЗ в хранилище.
	// Возвращает ID созданного ПВЗ или ошибку.
	CreatePVZ(ctx context.Context, pvz domain.PVZ) (uuid.UUID, error)

	// ListPVZs возвращает срез ПВЗ для текущей "страницы", определенной лимитом и курсором.
	// afterRegistrationDate и afterID используются для keyset pagination (должны быть оба nil или оба не nil).
	// Возвращает срез ПВЗ и ошибку.
	ListPVZs(ctx context.Context, limit int, afterRegistrationDate *time.Time, afterID *uuid.UUID) ([]domain.PVZ, error)

	// GetAllPVZs возвращает *все* ПВЗ из хранилища.
	// ВНИМАНИЕ: Может быть неэффективно при больших объемах данных.
	// Используется, например, для gRPC эндпоинта, где пагинация не реализована.
	GetAllPVZs(ctx context.Context) ([]domain.PVZ, error)

	// GetPVZByID возвращает ПВЗ по ID.
	// Может понадобиться для проверки существования ПВЗ перед созданием приемки.
	// Возвращает domain.PVZ и ошибку (например, sql.ErrNoRows, если не найден).
	// GetPVZByID(ctx context.Context, id uuid.UUID) (domain.PVZ, error) // Раскомментируйте, если будете реализовывать
}

// ReceptionRepository определяет методы для работы с приемками и товарами в рамках приемок.
//
//go:generate mockery --name ReceptionRepository --output ./mocks --outpkg mocks --case underscore --filename reception_repo_mock.go
type ReceptionRepository interface {
	// CreateReception создает новую запись о приемке.
	// Возвращает ID созданной приемки или ошибку.
	CreateReception(ctx context.Context, reception domain.Reception) (uuid.UUID, error)

	// GetLastOpenReceptionByPVZ ищет последнюю незакрытую приемку (статус 'in_progress') для данного ПВЗ.
	// Возвращает domain.Reception и nil, если найдена.
	// Возвращает пустую структуру и ErrReceptionNotFound, если не найдена (или закрыта).
	// Возвращает пустую структуру и другую ошибку при проблемах с БД.
	GetLastOpenReceptionByPVZ(ctx context.Context, pvzID uuid.UUID) (domain.Reception, error)

	// AddProductToReception добавляет товар к существующей приемке.
	// Возвращает ID добавленного товара или ошибку.
	AddProductToReception(ctx context.Context, product domain.Product) (uuid.UUID, error)

	// GetLastProductFromReception находит последний (по времени добавления) товар в указанной приемке.
	// Возвращает domain.Product и nil, если найден.
	// Возвращает пустую структуру и ErrProductNotFound, если товаров в приемке нет.
	// Возвращает пустую структуру и другую ошибку при проблемах с БД.
	GetLastProductFromReception(ctx context.Context, receptionID uuid.UUID) (domain.Product, error)

	// DeleteProductByID удаляет товар по его ID.
	// Возвращает ErrProductNotFound, если товар с таким ID не найден.
	// Возвращает nil при успехе или другую ошибку при проблемах с БД.
	DeleteProductByID(ctx context.Context, productID uuid.UUID) error

	// CloseReceptionByID изменяет статус приемки на 'closed'.
	// Обновляет только приемку со статусом 'in_progress'.
	// Возвращает ErrReceptionNotFound, если приемка не найдена или уже закрыта.
	// Возвращает nil при успехе или другую ошибку при проблемах с БД.
	CloseReceptionByID(ctx context.Context, receptionID uuid.UUID) error

	// ListReceptionsByPVZIDs возвращает все приемки для указанного списка ID ПВЗ,
	// опционально фильтруя по диапазону дат (startDate, endDate).
	ListReceptionsByPVZIDs(ctx context.Context, pvzIDs []uuid.UUID, startDate, endDate *time.Time) ([]domain.Reception, error)

	// ListProductsByReceptionIDs возвращает все товары для указанного списка ID приемок.
	ListProductsByReceptionIDs(ctx context.Context, receptionIDs []uuid.UUID) ([]domain.Product, error)
}

// UserRepository определяет методы для работы с пользователями в БД.
//
//go:generate mockery --name UserRepository --output ./mocks --outpkg mocks --case underscore --filename user_repo_mock.go
type UserRepository interface {
	// CreateUser сохраняет нового пользователя.
	// Возвращает ID созданного пользователя или ошибку (включая ErrUserDuplicateEmail).
	CreateUser(ctx context.Context, user domain.User) (uuid.UUID, error)

	// GetUserByEmail ищет пользователя по email.
	// Возвращает domain.User и nil, если найден.
	// Возвращает пустую структуру и ErrUserNotFound, если не найден.
	// Возвращает пустую структуру и другую ошибку при проблемах с БД.
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
}
