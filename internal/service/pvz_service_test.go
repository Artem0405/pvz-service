package service

import (
	"context"
	"errors"
	"testing"
	"time" // Нужен для тестов с датами

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"  // Для утверждений
	"github.com/stretchr/testify/mock"    // Для работы с моками
	"github.com/stretchr/testify/require" // Для критичных утверждений

	"github.com/Artem0405/pvz-service/internal/domain" // Ваши доменные модели
	// Нужен для repository.ErrReceptionNotFound и т.п.
	"github.com/Artem0405/pvz-service/internal/repository/mocks" // <<<--- ИМПОРТ МОКОВ
)

// Тесты для CreatePVZ
func TestPVZService_CreatePVZ(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Arrange: Создаем моки и сервис
		mockPVZRepo := new(mocks.PVZRepoMock) // <<<--- Используем мок
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		// Предполагаем, что NewPVZService принимает оба репозитория
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		inputPVZ := domain.PVZ{City: "Москва"}
		expectedID := uuid.New()

		// Настраиваем мок PVZRepo: Ожидаем вызов CreatePVZ с любым контекстом и нашим inputPVZ.
		// Возвращаем заранее заданный ID и nil ошибку. Вызов ожидается один раз.
		mockPVZRepo.On("CreatePVZ", mock.Anything, inputPVZ).Return(expectedID, nil).Once()

		// Act: Вызываем метод сервиса
		createdPVZ, err := pvzService.CreatePVZ(ctx, inputPVZ)

		// Assert: Проверяем результат
		require.NoError(t, err) // Используем require, т.к. без успеха дальше нет смысла
		assert.Equal(t, expectedID, createdPVZ.ID)
		assert.Equal(t, "Москва", createdPVZ.City) // Проверяем, что город сохранился

		// Убеждаемся, что ожидаемые вызовы моков произошли
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t) // ReceptionRepo не должен был вызываться
	})

	t.Run("Invalid City", func(t *testing.T) {
		// Arrange
		mockPVZRepo := new(mocks.PVZRepoMock)
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		inputPVZ := domain.PVZ{City: "Рязань"} // Невалидный город

		// Act
		_, err := pvzService.CreatePVZ(ctx, inputPVZ)

		// Assert
		require.Error(t, err)                                                     // Ожидаем ошибку
		assert.Contains(t, err.Error(), "создание ПВЗ возможно только в городах") // Проверяем текст ошибки валидации

		// Убеждаемся, что метод репозитория НЕ вызывался
		mockPVZRepo.AssertNotCalled(t, "CreatePVZ", mock.Anything, mock.Anything)
		mockReceptionRepo.AssertExpectations(t)
	})

	t.Run("Repository Error on Create", func(t *testing.T) {
		// Arrange
		mockPVZRepo := new(mocks.PVZRepoMock)
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		inputPVZ := domain.PVZ{City: "Казань"} // Валидный город
		repoError := errors.New("database connection lost")

		// Настраиваем мок на возврат ошибки
		mockPVZRepo.On("CreatePVZ", mock.Anything, inputPVZ).Return(uuid.Nil, repoError).Once()

		// Act
		_, err := pvzService.CreatePVZ(ctx, inputPVZ)

		// Assert
		require.Error(t, err)             // Ожидаем ошибку
		assert.ErrorIs(t, err, repoError) // Убеждаемся, что это именно ошибка репозитория

		// Проверяем моки
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t)
	})
}

// Тесты для GetPVZList
func TestPVZService_GetPVZList(t *testing.T) {
	ctx := context.Background()

	// --- Общие тестовые данные ---
	pvzID1 := uuid.New()
	pvzID2 := uuid.New()
	receptionID1 := uuid.New()
	receptionID2 := uuid.New()
	productID1 := uuid.New()
	productID2 := uuid.New()

	// Данные, которые *могут* вернуть моки
	mockPVZs := []domain.PVZ{
		{ID: pvzID1, City: "Москва", RegistrationDate: time.Now().Add(-2 * time.Hour)},
		{ID: pvzID2, City: "Казань", RegistrationDate: time.Now().Add(-1 * time.Hour)},
	}
	mockTotalCount := 2

	// --- Тест 1: Успешный базовый список (без фильтров) ---
	t.Run("Success - Basic List No Filters", func(t *testing.T) {
		// Arrange
		mockPVZRepo := new(mocks.PVZRepoMock) // <<<--- Используем мок
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		page, limit := 1, 10
		var startDate, endDate *time.Time // Без фильтров

		// Данные для моков Reception и Product
		mockReceptions := []domain.Reception{
			{ID: receptionID1, PVZID: pvzID1, DateTime: time.Now().Add(-30 * time.Minute), Status: domain.StatusInProgress},
			{ID: receptionID2, PVZID: pvzID2, DateTime: time.Now().Add(-15 * time.Minute), Status: domain.StatusClosed},
		}
		mockProducts := []domain.Product{
			{ID: productID1, ReceptionID: receptionID1, Type: domain.TypeElectronics, DateTimeAdded: time.Now().Add(-25 * time.Minute)},
			{ID: productID2, ReceptionID: receptionID2, Type: domain.TypeClothes, DateTimeAdded: time.Now().Add(-10 * time.Minute)},
		}
		// Списки ID, которые ожидаются в вызовах моков
		expectedPvzIDs := []uuid.UUID{pvzID1, pvzID2}
		expectedReceptionIDs := []uuid.UUID{receptionID1, receptionID2}

		// Настройка моков
		mockPVZRepo.On("ListPVZs", mock.Anything, page, limit).Return(mockPVZs, mockTotalCount, nil).Once()
		mockReceptionRepo.On("ListReceptionsByPVZIDs", mock.Anything, expectedPvzIDs, startDate, endDate).Return(mockReceptions, nil).Once()
		mockReceptionRepo.On("ListProductsByReceptionIDs", mock.Anything, expectedReceptionIDs).Return(mockProducts, nil).Once()

		// Act
		result, err := pvzService.GetPVZList(ctx, startDate, endDate, page, limit)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, mockTotalCount, result.TotalPVZs)
		require.Len(t, result.PVZs, 2)       // Ожидаем 2 ПВЗ
		require.NotNil(t, result.Receptions) // Карта приемок не должна быть nil
		require.NotNil(t, result.Products)   // Карта товаров не должна быть nil

		// Проверяем, что приемки и товары правильно сгруппированы
		assert.Len(t, result.Receptions[pvzID1], 1)
		assert.Len(t, result.Receptions[pvzID2], 1)
		assert.Len(t, result.Products[receptionID1], 1)
		assert.Len(t, result.Products[receptionID2], 1)

		// Проверяем конкретные ID для уверенности
		assert.Equal(t, receptionID1, result.Receptions[pvzID1][0].ID)
		assert.Equal(t, receptionID2, result.Receptions[pvzID2][0].ID)
		assert.Equal(t, productID1, result.Products[receptionID1][0].ID)
		assert.Equal(t, productID2, result.Products[receptionID2][0].ID)

		// Проверяем вызовы моков
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t)
	})

	// --- Тест 2: Успешно с фильтром по дате ---
	t.Run("Success - With Date Filter", func(t *testing.T) {
		// Arrange
		mockPVZRepo := new(mocks.PVZRepoMock)
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)

		page, limit := 1, 10
		// Фильтр, который включает только вторую приемку
		startTime := time.Now().Add(-20 * time.Minute)
		endTime := time.Now().Add(-10 * time.Minute)
		startDate, endDate := &startTime, &endTime

		// Данные для моков (только вторая приемка и ее товар)
		mockReceptionsFiltered := []domain.Reception{
			{ID: receptionID2, PVZID: pvzID2, DateTime: time.Now().Add(-15 * time.Minute), Status: domain.StatusClosed},
		}
		mockProductsFiltered := []domain.Product{
			{ID: productID2, ReceptionID: receptionID2, Type: domain.TypeClothes, DateTimeAdded: time.Now().Add(-10 * time.Minute)},
		}
		expectedPvzIDs := []uuid.UUID{pvzID1, pvzID2}             // ListPVZs все равно вернет оба
		expectedFilteredReceptionIDs := []uuid.UUID{receptionID2} // Только ID отфильтрованной приемки

		// Настройка моков
		mockPVZRepo.On("ListPVZs", mock.Anything, page, limit).Return(mockPVZs, mockTotalCount, nil).Once()
		// Ожидаем вызов с датами!
		mockReceptionRepo.On("ListReceptionsByPVZIDs", mock.Anything, expectedPvzIDs, startDate, endDate).Return(mockReceptionsFiltered, nil).Once()
		// Ожидаем вызов только с ID второй приемки
		mockReceptionRepo.On("ListProductsByReceptionIDs", mock.Anything, expectedFilteredReceptionIDs).Return(mockProductsFiltered, nil).Once()

		// Act
		result, err := pvzService.GetPVZList(ctx, startDate, endDate, page, limit)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, mockTotalCount, result.TotalPVZs) // Общее кол-во ПВЗ не меняется
		require.Len(t, result.PVZs, 2)

		// Проверяем, что у PVZ1 нет приемок (из-за фильтра), а у PVZ2 есть одна
		assert.Empty(t, result.Receptions[pvzID1]) // Пусто из-за фильтра
		require.Len(t, result.Receptions[pvzID2], 1)
		assert.Equal(t, receptionID2, result.Receptions[pvzID2][0].ID)

		// Проверяем, что есть товары только для второй приемки
		assert.Empty(t, result.Products[receptionID1])
		require.Len(t, result.Products[receptionID2], 1)
		assert.Equal(t, productID2, result.Products[receptionID2][0].ID)

		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t)
	})

	// --- Тест 3: Нет ПВЗ ---
	t.Run("Success - No PVZs Found", func(t *testing.T) {
		// Arrange
		mockPVZRepo := new(mocks.PVZRepoMock)
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)
		page, limit := 1, 10
		var startDate, endDate *time.Time

		// Мок PVZRepo возвращает пустой список и 0 total
		mockPVZRepo.On("ListPVZs", mock.Anything, page, limit).Return([]domain.PVZ{}, 0, nil).Once()

		// Act
		result, err := pvzService.GetPVZList(ctx, startDate, endDate, page, limit)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 0, result.TotalPVZs) // Общее кол-во 0
		assert.Empty(t, result.PVZs)         // Список ПВЗ пуст
		assert.Empty(t, result.Receptions)   // Проверяем, что карта пуста
		assert.Empty(t, result.Products)     // Проверяем, что карта пуста

		// Методы ReceptionRepo не должны были вызываться
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "ListReceptionsByPVZIDs", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		mockReceptionRepo.AssertNotCalled(t, "ListProductsByReceptionIDs", mock.Anything, mock.Anything)
	})

	// --- Тест 4: Ошибка от PVZ репозитория ---
	t.Run("Fail - PVZ Repo Error", func(t *testing.T) {
		// Arrange
		mockPVZRepo := new(mocks.PVZRepoMock)
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)
		page, limit := 1, 10
		repoError := errors.New("pvz repo failed")

		mockPVZRepo.On("ListPVZs", mock.Anything, page, limit).Return(nil, 0, repoError).Once()

		// Act
		_, err := pvzService.GetPVZList(ctx, nil, nil, page, limit)

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, repoError)

		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "ListReceptionsByPVZIDs", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	// --- Тест 5: Ошибка от Reception репозитория (при получении приемок) ---
	t.Run("Fail - Reception Repo Error on Receptions", func(t *testing.T) {
		// Arrange
		mockPVZRepo := new(mocks.PVZRepoMock)
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)
		page, limit := 1, 10
		repoError := errors.New("reception repo failed on list")
		expectedPvzIDs := []uuid.UUID{pvzID1, pvzID2} // ID из успешного ответа ListPVZs

		// Мок PVZRepo успешен
		mockPVZRepo.On("ListPVZs", mock.Anything, page, limit).Return(mockPVZs, mockTotalCount, nil).Once()
		// Мок ReceptionRepo падает на первом вызове
		mockReceptionRepo.On("ListReceptionsByPVZIDs", mock.Anything, expectedPvzIDs, (*time.Time)(nil), (*time.Time)(nil)).Return(nil, repoError).Once()

		// Act
		_, err := pvzService.GetPVZList(ctx, nil, nil, page, limit)

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t) // Проверяем вызов ListReceptions...
		// Вызов товаров не должен произойти
		mockReceptionRepo.AssertNotCalled(t, "ListProductsByReceptionIDs", mock.Anything, mock.Anything)
	})

	// --- Тест 6: Ошибка от Reception репозитория (при получении товаров) ---
	t.Run("Fail - Reception Repo Error on Products", func(t *testing.T) {
		// Arrange
		mockPVZRepo := new(mocks.PVZRepoMock)
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		pvzService := NewPVZService(mockPVZRepo, mockReceptionRepo)
		page, limit := 1, 10
		repoError := errors.New("reception repo failed on products")

		// Mock данные для успешных предыдущих шагов
		mockReceptions := []domain.Reception{
			{ID: receptionID1, PVZID: pvzID1}, // Упрощенно для этого теста
		}
		expectedPvzIDs := []uuid.UUID{pvzID1, pvzID2}
		expectedReceptionIDs := []uuid.UUID{receptionID1} // ID из успешного ответа ListReceptions...

		// Моки PVZRepo и ListReceptions... успешны
		mockPVZRepo.On("ListPVZs", mock.Anything, page, limit).Return(mockPVZs, mockTotalCount, nil).Once()
		mockReceptionRepo.On("ListReceptionsByPVZIDs", mock.Anything, expectedPvzIDs, (*time.Time)(nil), (*time.Time)(nil)).Return(mockReceptions, nil).Once()
		// Мок ListProducts... падает
		mockReceptionRepo.On("ListProductsByReceptionIDs", mock.Anything, expectedReceptionIDs).Return(nil, repoError).Once()

		// Act
		_, err := pvzService.GetPVZList(ctx, nil, nil, page, limit)

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		mockPVZRepo.AssertExpectations(t)
		mockReceptionRepo.AssertExpectations(t) // Оба вызова ReceptionRepo должны были быть
	})

}
