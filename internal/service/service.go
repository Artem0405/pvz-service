package service

import (
	"context"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain" // Только доменные модели
	"github.com/google/uuid"
)

// AuthService определяет методы для сервиса аутентификации.
// Эти методы будут использоваться в API слое (хендлеры, middleware).
type AuthService interface {
	Register(ctx context.Context, email, password, role string) (domain.User, error)
	Login(ctx context.Context, email, password string) (string, error) // Возвращает токен или ошибку
	// GenerateToken создает новый JWT для указанной роли.
	GenerateToken(role string) (string, error)
	// ValidateToken проверяет токен. Возвращает роль или другую информацию,
	// если токен валиден, и ошибку в противном случае.
	// Вместо *Claims можно вернуть просто роль (string) или кастомную структуру UserInfo.
	// Давайте вернем *Claims, как было в реализации.
	ValidateToken(tokenString string) (*Claims, error) // Возвращаем *Claims (структура из auth_service.go)
}

// ReceptionService определяет методы для управления приемками товаров.
type ReceptionService interface {
	// InitiateReception начинает новую приемку для указанного ПВЗ
	InitiateReception(ctx context.Context, pvzID uuid.UUID) (domain.Reception, error)
	// Добавляем метод добавления товара
	// Принимает ID ПВЗ (чтобы найти нужную приемку) и данные товара
	AddProduct(ctx context.Context, pvzID uuid.UUID, productType domain.ProductType) (domain.Product, error)
	// Добавляем метод удаления последнего товара
	DeleteLastProduct(ctx context.Context, pvzID uuid.UUID) error
	// Возвращает данные закрытой приемки или ошибку
	CloseLastReception(ctx context.Context, pvzID uuid.UUID) (domain.Reception, error)
}

// Claims определяет структуру полезной нагрузки токена (переносим сюда для видимости в интерфейсе)
// Либо можно оставить его в auth_service.go и не возвращать из ValidateToken в интерфейсе.
// Оставим пока в auth_service.go, а интерфейс ValidateToken вернет просто роль.
// Давайте переделаем интерфейс ValidateToken, чтобы он не зависел от Claims напрямую.

/*
// --- ПЕРЕДЕЛАННЫЙ ИНТЕРФЕЙС AuthService ---
type AuthService interface {
	GenerateToken(role string) (string, error)
	// ValidateToken проверяет токен и возвращает роль пользователя, если он валиден.
	ValidateToken(tokenString string) (role string, err error)
}
// В этом случае нужно будет изменить и реализацию ValidateToken в auth_service.go,
// чтобы она возвращала string, error, и middleware, чтобы она получала роль.
// Оставим пока как было, с возвратом *Claims, т.к. структура Claims используется в auth_service.go.
// Убедитесь, что структура Claims также доступна для пакета service, если она нужна в интерфейсе.
// Проще всего оставить Claims в auth_service.go и интерфейс ValidateToken тоже вернет *Claims.
*/

// --- Убедимся, что тип Claims видим ---
// Поскольку Claims используется и в auth_service.go и возвращается из интерфейса,
// его либо нужно вынести в отдельный пакет (например, domain), либо оставить в service
// и убедиться, что он не приватный.
// Давайте пока оставим его в auth_service.go, но убедимся, что он экспортируемый (с большой буквы).

// В файле auth_service.go: Убедитесь, что структура Claims называется с большой буквы.
// PVZService определяет методы бизнес-логики для ПВЗ
type PVZService interface {
	CreatePVZ(ctx context.Context, input domain.PVZ) (domain.PVZ, error)
	GetPVZList(ctx context.Context, startDate, endDate *time.Time, limit int, afterRegistrationDate *time.Time, afterID *uuid.UUID) (GetPVZListResult, error)
	// Другие методы, если есть...
}

// GetPVZListResult - структура для возврата результата из сервиса GetPVZList
type GetPVZListResult struct {
	PVZs       []domain.PVZ
	Receptions map[uuid.UUID][]domain.Reception
	Products   map[uuid.UUID][]domain.Product
	// Убрали TotalPVZs
	NextAfterRegistrationDate *time.Time // Добавили
	NextAfterID               *uuid.UUID // Добавили
}
