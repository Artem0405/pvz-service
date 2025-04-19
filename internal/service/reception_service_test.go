package service

import (
	"context"
	"errors"
	"testing"
	"time" // Для проверки времени в AddProduct

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository" // Нужен для ErrReceptionNotFound, ErrProductNotFound
	"github.com/Artem0405/pvz-service/internal/repository/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestReceptionService_InitiateReception остается без изменений...
func TestReceptionService_InitiateReception(t *testing.T) {
	ctx := context.Background()
	testPVZID := uuid.New()

	t.Run("Success - No open reception", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		expectedNewID := uuid.New()
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(domain.Reception{}, repository.ErrReceptionNotFound).Once()
		mockReceptionRepo.On("CreateReception", mock.Anything, mock.MatchedBy(func(r domain.Reception) bool { return r.PVZID == testPVZID })).Return(expectedNewID, nil).Once()
		createdReception, err := receptionService.InitiateReception(ctx, testPVZID)
		assert.NoError(t, err)
		assert.Equal(t, expectedNewID, createdReception.ID)
		mockReceptionRepo.AssertExpectations(t)
	})

	t.Run("Fail - Already open reception", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		existingReception := domain.Reception{ID: uuid.New(), PVZID: testPVZID, Status: domain.StatusInProgress}
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(existingReception, nil).Once()
		_, err := receptionService.InitiateReception(ctx, testPVZID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "предыдущая приемка для этого ПВЗ еще не закрыта")
		mockReceptionRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "CreateReception", mock.Anything, mock.Anything)
	})

	t.Run("Fail - Error checking existing reception", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		repoError := errors.New("DB connection error")
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(domain.Reception{}, repoError).Once()
		_, err := receptionService.InitiateReception(ctx, testPVZID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		mockReceptionRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "CreateReception", mock.Anything, mock.Anything)
	})

	t.Run("Fail - Error creating reception", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		repoError := errors.New("Failed to insert")
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(domain.Reception{}, repository.ErrReceptionNotFound).Once()
		mockReceptionRepo.On("CreateReception", mock.Anything, mock.AnythingOfType("domain.Reception")).Return(uuid.Nil, repoError).Once()
		_, err := receptionService.InitiateReception(ctx, testPVZID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		mockReceptionRepo.AssertExpectations(t)
	})
}

// Тесты для AddProduct
func TestReceptionService_AddProduct(t *testing.T) {
	ctx := context.Background()
	testPVZID := uuid.New()
	testReceptionID := uuid.New()
	testProductID := uuid.New()
	openReception := domain.Reception{ID: testReceptionID, PVZID: testPVZID, Status: domain.StatusInProgress}

	t.Run("Success", func(t *testing.T) {
		// Arrange
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		productType := domain.TypeClothes

		// Моки: найти открытую приемку -> успех; добавить товар -> успех
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(openReception, nil).Once()
		mockReceptionRepo.On("AddProductToReception", mock.Anything, mock.MatchedBy(func(p domain.Product) bool {
			return p.ReceptionID == testReceptionID && p.Type == productType
		})).Return(testProductID, nil).Once()

		// Act
		addedProduct, err := receptionService.AddProduct(ctx, testPVZID, productType)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, testProductID, addedProduct.ID)
		assert.Equal(t, testReceptionID, addedProduct.ReceptionID)
		assert.Equal(t, productType, addedProduct.Type)
		mockReceptionRepo.AssertExpectations(t)
	})

	t.Run("Fail - Invalid Product Type", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		_, err := receptionService.AddProduct(ctx, testPVZID, "invalid_type") // Невалидный тип
		assert.Error(t, err)
		assert.EqualError(t, err, "недопустимый тип товара")
		mockReceptionRepo.AssertNotCalled(t, "GetLastOpenReceptionByPVZ", mock.Anything, mock.Anything)
	})

	t.Run("Fail - No Open Reception", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(domain.Reception{}, repository.ErrReceptionNotFound).Once()

		_, err := receptionService.AddProduct(ctx, testPVZID, domain.TypeShoes)

		assert.Error(t, err)
		assert.EqualError(t, err, "нет открытой приемки для данного ПВЗ, чтобы добавить товар")
		mockReceptionRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "AddProductToReception", mock.Anything, mock.Anything)
	})

	t.Run("Fail - Error Finding Reception", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		repoError := errors.New("DB error find reception")
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(domain.Reception{}, repoError).Once()

		_, err := receptionService.AddProduct(ctx, testPVZID, domain.TypeElectronics)

		assert.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		mockReceptionRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "AddProductToReception", mock.Anything, mock.Anything)
	})

	t.Run("Fail - Error Adding Product", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		productType := domain.TypeClothes
		repoError := errors.New("DB error add product")

		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(openReception, nil).Once()
		mockReceptionRepo.On("AddProductToReception", mock.Anything, mock.AnythingOfType("domain.Product")).Return(uuid.Nil, repoError).Once()

		_, err := receptionService.AddProduct(ctx, testPVZID, productType)

		assert.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		mockReceptionRepo.AssertExpectations(t)
	})
}

// Тесты для DeleteLastProduct
func TestReceptionService_DeleteLastProduct(t *testing.T) {
	ctx := context.Background()
	testPVZID := uuid.New()
	testReceptionID := uuid.New()
	testProductID := uuid.New()
	openReception := domain.Reception{ID: testReceptionID, PVZID: testPVZID, Status: domain.StatusInProgress}
	lastProduct := domain.Product{ID: testProductID, ReceptionID: testReceptionID, Type: domain.TypeShoes}

	t.Run("Success", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)

		// Моки: найти приемку -> найти товар -> удалить товар
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(openReception, nil).Once()
		mockReceptionRepo.On("GetLastProductFromReception", mock.Anything, testReceptionID).Return(lastProduct, nil).Once()
		mockReceptionRepo.On("DeleteProductByID", mock.Anything, testProductID).Return(nil).Once()

		err := receptionService.DeleteLastProduct(ctx, testPVZID)

		assert.NoError(t, err)
		mockReceptionRepo.AssertExpectations(t)
	})

	t.Run("Fail - No Open Reception", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(domain.Reception{}, repository.ErrReceptionNotFound).Once()

		err := receptionService.DeleteLastProduct(ctx, testPVZID)

		assert.Error(t, err)
		assert.EqualError(t, err, "нет открытой приемки для данного ПВЗ, чтобы удалить товар")
		mockReceptionRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "GetLastProductFromReception", mock.Anything, mock.Anything)
		mockReceptionRepo.AssertNotCalled(t, "DeleteProductByID", mock.Anything, mock.Anything)
	})

	t.Run("Fail - No Products in Reception", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(openReception, nil).Once()
		mockReceptionRepo.On("GetLastProductFromReception", mock.Anything, testReceptionID).Return(domain.Product{}, repository.ErrProductNotFound).Once() // <-- Товар не найден

		err := receptionService.DeleteLastProduct(ctx, testPVZID)

		assert.Error(t, err)
		assert.EqualError(t, err, "в текущей открытой приемке нет товаров для удаления")
		mockReceptionRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "DeleteProductByID", mock.Anything, mock.Anything)
	})

	// TODO: Добавить тесты на ошибки репозитория при поиске приемки, поиске товара, удалении товара
}

// Тесты для CloseLastReception
func TestReceptionService_CloseLastReception(t *testing.T) {
	ctx := context.Background()
	testPVZID := uuid.New()
	testReceptionID := uuid.New()
	openReception := domain.Reception{ID: testReceptionID, PVZID: testPVZID, Status: domain.StatusInProgress, DateTime: time.Now()}

	t.Run("Success", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)

		// Моки: найти приемку -> закрыть приемку
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(openReception, nil).Once()
		mockReceptionRepo.On("CloseReceptionByID", mock.Anything, testReceptionID).Return(nil).Once()

		closedReception, err := receptionService.CloseLastReception(ctx, testPVZID)

		assert.NoError(t, err)
		assert.Equal(t, testReceptionID, closedReception.ID)
		assert.Equal(t, domain.StatusClosed, closedReception.Status) // Проверяем статус
		mockReceptionRepo.AssertExpectations(t)
	})

	t.Run("Fail - No Open Reception", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(domain.Reception{}, repository.ErrReceptionNotFound).Once()

		_, err := receptionService.CloseLastReception(ctx, testPVZID)

		assert.Error(t, err)
		assert.EqualError(t, err, "нет открытой приемки для данного ПВЗ для закрытия")
		mockReceptionRepo.AssertExpectations(t)
		mockReceptionRepo.AssertNotCalled(t, "CloseReceptionByID", mock.Anything, mock.Anything)
	})

	t.Run("Fail - Error Closing Reception", func(t *testing.T) {
		mockReceptionRepo := new(mocks.ReceptionRepoMock)
		receptionService := NewReceptionService(mockReceptionRepo)
		repoError := errors.New("DB error close reception")

		mockReceptionRepo.On("GetLastOpenReceptionByPVZ", mock.Anything, testPVZID).Return(openReception, nil).Once()
		mockReceptionRepo.On("CloseReceptionByID", mock.Anything, testReceptionID).Return(repoError).Once() // <-- Ошибка при закрытии

		_, err := receptionService.CloseLastReception(ctx, testPVZID)

		assert.Error(t, err)
		assert.ErrorIs(t, err, repoError)
		mockReceptionRepo.AssertExpectations(t)
	})

	// TODO: Добавить тест на ошибку поиска открытой приемки
}
