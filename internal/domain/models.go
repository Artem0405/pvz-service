package domain

import (
	"time"

	"github.com/google/uuid"
)

// --- Структуры для новых эндпоинтов ---
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"` // Оставляем возможность задать роль при регистрации
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// User - структура пользователя
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"` // Поле добавлено
	PasswordHash string    `json:"-"`     // Хэш пароля, НЕ отдаем в JSON!
	Role         string    `json:"role"`
	// Добавим поля created_at/updated_at, если их нет и они нужны
	// CreatedAt    time.Time `json:"-"`
	// UpdatedAt    time.Time `json:"-"`
}

// PVZ ... (остальные структуры без изменений) ...
type PVZ struct {
	ID               uuid.UUID `json:"id"`
	RegistrationDate time.Time `json:"registrationDate"`
	City             string    `json:"city"`
}

type ReceptionStatus string

const (
	StatusInProgress ReceptionStatus = "in_progress"
	StatusClosed     ReceptionStatus = "closed"
	RoleEmployee                     = "employee"
	RoleModerator                    = "moderator"
)

type Reception struct {
	ID       uuid.UUID       `json:"id"`
	PVZID    uuid.UUID       `json:"pvzId"`
	DateTime time.Time       `json:"dateTime"`
	Status   ReceptionStatus `json:"status"`
}
type ProductType string

const (
	TypeElectronics ProductType = "электроника"
	TypeClothes     ProductType = "одежда"
	TypeShoes       ProductType = "обувь"
)

type Product struct {
	ID            uuid.UUID   `json:"id"`
	ReceptionID   uuid.UUID   `json:"receptionId"`
	DateTimeAdded time.Time   `json:"dateTimeAdded"`
	Type          ProductType `json:"type"`
}

// GetPVZListResult ...
type GetPVZListResult struct {
	PVZs       []PVZ                     `json:"-"` // Не отдаем напрямую, собираем items
	Receptions map[uuid.UUID][]Reception `json:"-"`
	Products   map[uuid.UUID][]Product   `json:"-"`
	TotalPVZs  int                       `json:"totalCount"` // Отдаем общее количество
}
