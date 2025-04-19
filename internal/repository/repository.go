package repository

import (
	"context" // Используем context для передачи сигналов отмены/таймаутов
	"database/sql"
	"errors"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"

	"github.com/google/uuid"
)

var ErrReceptionNotFound = sql.ErrNoRows // Используем стандартную ошибку для "не найдено"
var ErrProductNotFound = sql.ErrNoRows   // Ошибка, если нет товаров для удаления

// PVZRepository определяет методы для работы с хранилищем ПВЗ
type PVZRepository interface {
	// CreatePVZ сохраняет новый ПВЗ в хранилище
	CreatePVZ(ctx context.Context, pvz domain.PVZ) (uuid.UUID, error)
	// GetPVZByID возвращает ПВЗ по ID (понадобится позже)
	// GetPVZByID(ctx context.Context, id uuid.UUID) (domain.PVZ, error)
	// ListPVZs возвращает список ПВЗ и общее количество для пагинации
	ListPVZs(ctx context.Context, page, limit int) ([]domain.PVZ, int, error)
	GetAllPVZs(ctx context.Context) ([]domain.PVZ, error)
}

type ReceptionRepository interface {
	// CreateReception создает новую запись о приемке
	CreateReception(ctx context.Context, reception domain.Reception) (uuid.UUID, error)
	// GetLastOpenReceptionByPVZ ищет последнюю незакрытую приемку для данного ПВЗ
	// Возвращает domain.Reception и nil, если найдена.
	// Возвращает пустую структуру и ErrReceptionNotFound, если не найдена.
	// Возвращает пустую структуру и другую ошибку при проблемах с БД.
	GetLastOpenReceptionByPVZ(ctx context.Context, pvzID uuid.UUID) (domain.Reception, error)
	// Добавляем метод для сохранения товара
	AddProductToReception(ctx context.Context, product domain.Product) (uuid.UUID, error)
	// GetLastProductFromReception находит последний добавленный товар в приемке
	GetLastProductFromReception(ctx context.Context, receptionID uuid.UUID) (domain.Product, error)
	// DeleteProductByID удаляет товар по ID
	DeleteProductByID(ctx context.Context, productID uuid.UUID) error
	// CloseReceptionByID изменяет статус приемки на 'closed'
	CloseReceptionByID(ctx context.Context, receptionID uuid.UUID) error
	// ListReceptionsByPVZIDs возвращает приемки для списка ПВЗ с фильтром по дате
	ListReceptionsByPVZIDs(ctx context.Context, pvzIDs []uuid.UUID, startDate, endDate *time.Time) ([]domain.Reception, error)
	// ListProductsByReceptionIDs возвращает товары для списка приемок
	ListProductsByReceptionIDs(ctx context.Context, receptionIDs []uuid.UUID) ([]domain.Product, error)
}

// UserRepository определяет методы для работы с пользователями в БД
type UserRepository interface {
	CreateUser(ctx context.Context, user domain.User) (uuid.UUID, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
}

// Можно определить кастомные ошибки
var ErrUserNotFound = errors.New("user not found")
var ErrUserDuplicateEmail = errors.New("user with this email already exists")
