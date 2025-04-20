package service

import (
	"context"
	"errors"
	"testing"
	"time" // Нужен для тестов с датами

	"github.com/Artem0405/pvz-service/internal/domain"
	// Нужен для кастомных ошибок репозитория
	// --- ИСПРАВЛЕНО: Импорт моков ---
	"github.com/Artem0405/pvz-service/internal/repository/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestPVZService_CreatePVZ остается без изменений
func TestPVZService_CreatePVZ(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Используем правильные типы моков
		mockPVZRepo := new(mocks.PVZRepository)             // ИСПРАВЛЕНО
		mockReceptionRepo := new(mocks.ReceptionRepository) // ИСПРАВЛЕНО
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		inputPVZ := domain.PVZ{City: "Москва"}
		expectedID := uuid.New()

		mockPVZRepo.On("CreatePVZ", mock.Anything, inputPVZ).Return(expectedID, nil).Once()

		createdPVZ, err := pvzService.CreatePVZ(ctx, inputPVZ)

		require.NoError(t, err)
		assert.Equal(t, expectedID, createdPVZ.ID)
		assert.Equal(t, "Москва", createdPVZ.City)
		assert.False(t, createdPVZ.RegistrationDate.IsZero())
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t)
	})

	t.Run("Invalid City", func(t *testing.T) {
		mockPVZRepo := new(mocks.PVZRepository)             // ИСПРАВЛЕНО
		mockReceptionRepo := new(mocks.ReceptionRepository) // ИСПРАВЛЕНО
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		inputPVZ := domain.PVZ{City: "Рязань"}
		_, err := pvzService.CreatePVZ(ctx, inputPVZ)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "создание ПВЗ возможно только в городах")
		mockPVZRepo.AssertNotCalled(t, "CreatePVZ", mock.Anything, mock.Anything)
		mockReceptionRepo.AssertExpectations(t)
	})

	t.Run("Repository Error on Create", func(t *testing.T) {
		mockPVZRepo := new(mocks.PVZRepository)             // ИСПРАВЛЕНО
		mockReceptionRepo := new(mocks.ReceptionRepository) // ИСПРАВЛЕНО
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		inputPVZ := domain.PVZ{City: "Казань"}
		repoError := errors.New("database connection lost")

		mockPVZRepo.On("CreatePVZ", mock.Anything, inputPVZ).Return(uuid.Nil, repoError).Once()
		_, err := pvzService.CreatePVZ(ctx, inputPVZ)

		assert.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		assert.Contains(t, err.Error(), "не удалось сохранить ПВЗ")
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t)
	})
}

// --- ИСПРАВЛЕНО: Тесты для метода GetPVZList с Keyset Pagination ---
func TestPVZService_GetPVZList(t *testing.T) {
	ctx := context.Background()
	// Общие тестовые данные
	pvzID1 := uuid.New()
	pvzID2 := uuid.New()
	receptionID1 := uuid.New()
	receptionID2 := uuid.New()
	productID1 := uuid.New()
	productID2 := uuid.New()
	now := time.Now()
	mockPVZs := []domain.PVZ{
		{ID: pvzID1, City: "Москва", RegistrationDate: now.Add(-2 * time.Hour)},
		{ID: pvzID2, City: "Казань", RegistrationDate: now.Add(-1 * time.Hour)},
	}
	mockReceptions := []domain.Reception{
		{ID: receptionID1, PVZID: pvzID1, DateTime: now.Add(-30 * time.Minute), Status: domain.StatusInProgress},
		{ID: receptionID2, PVZID: pvzID2, DateTime: now.Add(-15 * time.Minute), Status: domain.StatusClosed},
	}
	mockProducts := []domain.Product{
		{ID: productID1, ReceptionID: receptionID1, Type: domain.TypeElectronics, DateTimeAdded: now.Add(-25 * time.Minute)},
		{ID: productID2, ReceptionID: receptionID2, Type: domain.TypeClothes, DateTimeAdded: now.Add(-10 * time.Minute)},
	}

	// --- Тест 1: Успешный базовый список (первая страница, без фильтров) ---
	t.Run("Success - Basic List No Filters First Page", func(t *testing.T) {
		mockPVZRepo := new(mocks.PVZRepository)             // ИСПРАВЛЕНО
		mockReceptionRepo := new(mocks.ReceptionRepository) // ИСПРАВЛЕНО
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		limit := 10
		var startDate, endDate *time.Time
		var cursorDate *time.Time = nil
		var cursorID *uuid.UUID = nil

		// --- ИСПРАВЛЕНО: Настройка мока ListPVZs ---
		mockPVZRepo.On(
			"ListPVZs",
			mock.Anything, // ctx
			limit,         // limit (int)
			cursorDate,    // afterRegistrationDate (*time.Time)
			cursorID,      // afterID (*uuid.UUID)
		).Return(mockPVZs, nil).Once() // Возвращает []domain.PVZ, error

		mockReceptionRepo.On("ListReceptionsByPVZIDs", mock.Anything, []uuid.UUID{pvzID1, pvzID2}, startDate, endDate).Return(mockReceptions, nil).Once()
		mockReceptionRepo.On("ListProductsByReceptionIDs", mock.Anything, []uuid.UUID{receptionID1, receptionID2}).Return(mockProducts, nil).Once()

		// --- ИСПРАВЛЕНО: Вызов GetPVZList ---
		result, err := pvzService.GetPVZList(ctx, startDate, endDate, limit, cursorDate, cursorID)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, result.PVZs, 2)
		assert.NotNil(t, result.Receptions)
		assert.NotNil(t, result.Products)
		// --- УДАЛЕНО: Проверка TotalPVZs ---
		assert.Nil(t, result.NextAfterID) // Курсор nil, т.к. len < limit
		assert.Nil(t, result.NextAfterRegistrationDate)
		assert.Len(t, result.Receptions[pvzID1], 1)
		// ... остальные assert'ы ...
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t)
	})

	// --- Тест 2: Запрос "следующей" страницы с курсором ---
	t.Run("Success - Keyset Pagination Second Page", func(t *testing.T) {
		mockPVZRepo := new(mocks.PVZRepository)             // ИСПРАВЛЕНО
		mockReceptionRepo := new(mocks.ReceptionRepository) // ИСПРАВЛЕНО
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		limit := 1
		cursorDate := mockPVZs[1].RegistrationDate // Курсор на второй (последний в mockPVZs)
		cursorID := mockPVZs[1].ID

		expectedPVZsPage2 := []domain.PVZ{mockPVZs[0]} // Ожидаем первый элемент
		expectedReceptionIDsForProducts := []uuid.UUID{receptionID1}

		// --- ИСПРАВЛЕНО: Настройка мока ListPVZs с курсором ---
		mockPVZRepo.On("ListPVZs", mock.Anything, limit, &cursorDate, &cursorID).Return(expectedPVZsPage2, nil).Once()
		mockReceptionRepo.On("ListReceptionsByPVZIDs", mock.Anything, []uuid.UUID{pvzID1}, (*time.Time)(nil), (*time.Time)(nil)).Return([]domain.Reception{mockReceptions[0]}, nil).Once()
		mockReceptionRepo.On("ListProductsByReceptionIDs", mock.Anything, expectedReceptionIDsForProducts).Return([]domain.Product{mockProducts[0]}, nil).Once()

		// --- ИСПРАВЛЕНО: Вызов GetPVZList с курсором ---
		result, err := pvzService.GetPVZList(ctx, nil, nil, limit, &cursorDate, &cursorID)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, result.PVZs, 1)
		assert.Equal(t, pvzID1, result.PVZs[0].ID)
		require.NotNil(t, result.NextAfterID)
		require.NotNil(t, result.NextAfterRegistrationDate)
		assert.Equal(t, pvzID1, *result.NextAfterID)
		assert.Equal(t, mockPVZs[0].RegistrationDate, *result.NextAfterRegistrationDate)
		// ... остальные assert'ы ...
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t)
	})

	// --- Тест 3: Нет ПВЗ ---
	t.Run("Success - No PVZs Found", func(t *testing.T) {
		mockPVZRepo := new(mocks.PVZRepository)             // ИСПРАВЛЕНО
		mockReceptionRepo := new(mocks.ReceptionRepository) // ИСПРАВЛЕНО
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		limit := 10
		var cursorDate *time.Time = nil
		var cursorID *uuid.UUID = nil

		// --- ИСПРАВЛЕНО: Настройка мока ListPVZs ---
		mockPVZRepo.On("ListPVZs", mock.Anything, limit, cursorDate, cursorID).Return([]domain.PVZ{}, nil).Once()

		// --- ИСПРАВЛЕНО: Вызов GetPVZList ---
		result, err := pvzService.GetPVZList(ctx, nil, nil, limit, cursorDate, cursorID)

		// Assert
		assert.NoError(t, err)
		assert.Empty(t, result.PVZs)
		// --- УДАЛЕНО: Проверка TotalPVZs ---
		assert.Nil(t, result.Receptions)
		assert.Nil(t, result.Products)
		assert.Nil(t, result.NextAfterID)
		assert.Nil(t, result.NextAfterRegistrationDate)
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "ListReceptionsByPVZIDs", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		mockReceptionRepo.AssertNotCalled(t, "ListProductsByReceptionIDs", mock.Anything, mock.Anything)
	})

	// --- Тест 4: Ошибка от PVZ репозитория ---
	t.Run("Fail - PVZ Repo Error", func(t *testing.T) {
		mockPVZRepo := new(mocks.PVZRepository)             // ИСПРАВЛЕНО
		mockReceptionRepo := new(mocks.ReceptionRepository) // ИСПРАВЛЕНО
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		limit := 10
		repoError := errors.New("pvz repo failed")

		// --- ИСПРАВЛЕНО: Настройка мока ListPVZs ---
		mockPVZRepo.On("ListPVZs", mock.Anything, limit, (*time.Time)(nil), (*uuid.UUID)(nil)).Return(nil, repoError).Once()

		// --- ИСПРАВЛЕНО: Вызов GetPVZList ---
		_, err := pvzService.GetPVZList(ctx, nil, nil, limit, nil, nil)

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		assert.Contains(t, err.Error(), "не удалось получить список ПВЗ")
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "ListReceptionsByPVZIDs", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		mockReceptionRepo.AssertNotCalled(t, "ListProductsByReceptionIDs", mock.Anything, mock.Anything)
	})

	// --- Тест 5: Ошибка от Reception репозитория (при получении приемок) ---
	t.Run("Fail - Reception Repo Error on Receptions", func(t *testing.T) {
		mockPVZRepo := new(mocks.PVZRepository)             // ИСПРАВЛЕНО
		mockReceptionRepo := new(mocks.ReceptionRepository) // ИСПРАВЛЕНО
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		limit := 10
		repoError := errors.New("reception repo failed on list")

		// --- ИСПРАВЛЕНО: Настройка мока ListPVZs ---
		mockPVZRepo.On("ListPVZs", mock.Anything, limit, (*time.Time)(nil), (*uuid.UUID)(nil)).Return(mockPVZs, nil).Once()
		mockReceptionRepo.On("ListReceptionsByPVZIDs", mock.Anything, []uuid.UUID{pvzID1, pvzID2}, (*time.Time)(nil), (*time.Time)(nil)).Return(nil, repoError).Once()

		// --- ИСПРАВЛЕНО: Вызов GetPVZList ---
		_, err := pvzService.GetPVZList(ctx, nil, nil, limit, nil, nil)

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		assert.Contains(t, err.Error(), "не удалось получить приемки")
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "ListProductsByReceptionIDs", mock.Anything, mock.Anything)
	})

	// --- Тест 6: Ошибка от Reception репозитория (при получении товаров) ---
	t.Run("Fail - Reception Repo Error on Products", func(t *testing.T) {
		mockPVZRepo := new(mocks.PVZRepository)             // ИСПРАВЛЕНО
		mockReceptionRepo := new(mocks.ReceptionRepository) // ИСПРАВЛЕНО
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		limit := 10
		repoError := errors.New("reception repo failed on products")

		// --- ИСПРАВЛЕНО: Настройка мока ListPVZs ---
		mockPVZRepo.On("ListPVZs", mock.Anything, limit, (*time.Time)(nil), (*uuid.UUID)(nil)).Return(mockPVZs, nil).Once()
		mockReceptionRepo.On("ListReceptionsByPVZIDs", mock.Anything, []uuid.UUID{pvzID1, pvzID2}, (*time.Time)(nil), (*time.Time)(nil)).Return(mockReceptions, nil).Once()
		mockReceptionRepo.On("ListProductsByReceptionIDs", mock.Anything, []uuid.UUID{receptionID1, receptionID2}).Return(nil, repoError).Once()

		// --- ИСПРАВЛЕНО: Вызов GetPVZList ---
		_, err := pvzService.GetPVZList(ctx, nil, nil, limit, nil, nil)

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		assert.Contains(t, err.Error(), "не удалось получить товары")
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t)
	})
}
