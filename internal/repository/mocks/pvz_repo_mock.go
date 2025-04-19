package mocks

import (
	"context"

	"github.com/Artem0405/pvz-service/internal/domain" // Путь к вашим доменным моделям
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock" // Импортируем testify mock
)

// PVZRepoMock - это мок-структура для PVZRepository
type PVZRepoMock struct {
	mock.Mock // Встраиваем mock.Mock
}

// CreatePVZ - мок-реализация метода CreatePVZ
func (m *PVZRepoMock) CreatePVZ(ctx context.Context, pvz domain.PVZ) (uuid.UUID, error) {
	// Регистрируем вызов метода с переданными аргументами
	args := m.Called(ctx, pvz)
	// Возвращаем то, что было настроено в тесте через .Return(...)
	// Первый возвращаемый аргумент - uuid.UUID, второй - error
	// Используем args.Get(0).(uuid.UUID) для получения первого аргумента
	// Используем args.Error(1) для получения второго аргумента (ошибки)

	// Безопасное получение UUID
	var id uuid.UUID
	if args.Get(0) != nil {
		id = args.Get(0).(uuid.UUID)
	}

	return id, args.Error(1)
}

// --- Моки для других методов PVZRepository (ListPVZs и т.д.) ---
// ListPVZs - мок-реализация
func (m *PVZRepoMock) ListPVZs(ctx context.Context, page, limit int) ([]domain.PVZ, int, error) {
	args := m.Called(ctx, page, limit)
	var pvzList []domain.PVZ
	if args.Get(0) != nil {
		pvzList = args.Get(0).([]domain.PVZ)
	}
	return pvzList, args.Int(1), args.Error(2)
}

// GetAllPVZs мок-реализация метода GetAllPVZs
func (m *PVZRepoMock) GetAllPVZs(ctx context.Context) ([]domain.PVZ, error) {
	args := m.Called(ctx) // Регистрируем вызов

	var resultPVZs []domain.PVZ
	if args.Get(0) != nil { // Безопасно получаем первый аргумент возврата
		resultPVZs = args.Get(0).([]domain.PVZ)
	}
	// Получаем второй аргумент возврата (ошибку)
	return resultPVZs, args.Error(1)
}
