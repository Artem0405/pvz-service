package mocks

import (
	"context"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// ReceptionRepoMock - мок-структура для ReceptionRepository
type ReceptionRepoMock struct {
	mock.Mock
}

func (m *ReceptionRepoMock) CreateReception(ctx context.Context, reception domain.Reception) (uuid.UUID, error) {
	args := m.Called(ctx, reception)
	var id uuid.UUID
	if args.Get(0) != nil {
		id = args.Get(0).(uuid.UUID)
	}
	return id, args.Error(1)
}

func (m *ReceptionRepoMock) GetLastOpenReceptionByPVZ(ctx context.Context, pvzID uuid.UUID) (domain.Reception, error) {
	args := m.Called(ctx, pvzID)
	var reception domain.Reception
	// Обрабатываем случай, когда возвращается nil вместо структуры
	if args.Get(0) != nil {
		reception = args.Get(0).(domain.Reception)
	}
	return reception, args.Error(1) // Ошибка может быть repository.ErrReceptionNotFound
}

func (m *ReceptionRepoMock) AddProductToReception(ctx context.Context, product domain.Product) (uuid.UUID, error) {
	args := m.Called(ctx, product)
	var id uuid.UUID
	if args.Get(0) != nil {
		id = args.Get(0).(uuid.UUID)
	}
	return id, args.Error(1)
}

func (m *ReceptionRepoMock) GetLastProductFromReception(ctx context.Context, receptionID uuid.UUID) (domain.Product, error) {
	args := m.Called(ctx, receptionID)
	var product domain.Product
	if args.Get(0) != nil {
		product = args.Get(0).(domain.Product)
	}
	return product, args.Error(1) // Ошибка может быть repository.ErrProductNotFound
}

func (m *ReceptionRepoMock) DeleteProductByID(ctx context.Context, productID uuid.UUID) error {
	args := m.Called(ctx, productID)
	return args.Error(0)
}

func (m *ReceptionRepoMock) CloseReceptionByID(ctx context.Context, receptionID uuid.UUID) error {
	args := m.Called(ctx, receptionID)
	return args.Error(0)
}

// --- Моки для методов получения списков (если они будут использоваться в тестах сервиса) ---
func (m *ReceptionRepoMock) ListReceptionsByPVZIDs(ctx context.Context, pvzIDs []uuid.UUID, startDate, endDate *time.Time) ([]domain.Reception, error) {
	args := m.Called(ctx, pvzIDs, startDate, endDate)
	var receptions []domain.Reception
	if args.Get(0) != nil {
		receptions = args.Get(0).([]domain.Reception)
	}
	return receptions, args.Error(1)
}

func (m *ReceptionRepoMock) ListProductsByReceptionIDs(ctx context.Context, receptionIDs []uuid.UUID) ([]domain.Product, error) {
	args := m.Called(ctx, receptionIDs)
	var products []domain.Product
	if args.Get(0) != nil {
		products = args.Get(0).([]domain.Product)
	}
	return products, args.Error(1)
}
