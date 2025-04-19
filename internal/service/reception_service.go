package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	// import "log" // <-- Удалить
	// <-- Добавить

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository"
	"github.com/google/uuid"
)

// receptionService - реализация ReceptionService
type receptionService struct {
	repo repository.ReceptionRepository // Зависимость от репозитория приемок
	// Возможно, понадобится PVZ репозиторий для проверки существования PVZ ID
	// pvzRepo repository.PVZRepository
}

// NewReceptionService - конструктор
func NewReceptionService(repo repository.ReceptionRepository) *receptionService {
	return &receptionService{
		repo: repo,
	}
}

// InitiateReception - начинает новую приемку
func (s *receptionService) InitiateReception(ctx context.Context, pvzID uuid.UUID) (domain.Reception, error) {
	// Проверяем, нет ли уже открытой приемки для этого ПВЗ
	_, err := s.repo.GetLastOpenReceptionByPVZ(ctx, pvzID)

	// Обрабатываем результат проверки
	if err == nil {
		// Ошибки нет => Найдена открытая приемка! Нельзя начать новую.
		slog.WarnContext(ctx, "Попытка начать новую приемку при наличии открытой", "pvz_id", pvzID)
		return domain.Reception{}, errors.New("предыдущая приемка для этого ПВЗ еще не закрыта")
	}

	// Если ошибка - это НЕ "не найдено", значит, произошла другая проблема при проверке
	if !errors.Is(err, repository.ErrReceptionNotFound) {
		slog.ErrorContext(ctx, "Ошибка при проверке существующей открытой приемки", "pvz_id", pvzID, "error", err)
		return domain.Reception{}, fmt.Errorf("ошибка проверки существующей приемки: %w", err)
	}

	// Если мы здесь, значит err == repository.ErrReceptionNotFound - можно создавать новую

	// TODO: Опционально - проверить, существует ли сам pvzID, если добавить pvzRepo

	// Создаем новую запись о приемке
	newReception := domain.Reception{
		PVZID:  pvzID,
		Status: domain.StatusInProgress, // Устанавливается по умолчанию
		// DateTime установится в БД по умолчанию
	}

	createdID, err := s.repo.CreateReception(ctx, newReception)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка репозитория при создании приемки", "pvz_id", pvzID, "error", err)
		return domain.Reception{}, fmt.Errorf("не удалось создать приемку: %w", err)
	}

	// Формируем ответ API (ID и PVZID уже есть, добавим примерное время)
	createdReception := domain.Reception{
		ID:       createdID,
		PVZID:    pvzID,
		Status:   domain.StatusInProgress,
		DateTime: time.Now(), // Примерное время для ответа
	}
	slog.InfoContext(ctx, "Приемка успешно создана", "reception_id", createdID, "pvz_id", pvzID)
	return createdReception, nil
}

// AddProduct - добавляет товар в последнюю открытую приемку для указанного ПВЗ
func (s *receptionService) AddProduct(ctx context.Context, pvzID uuid.UUID, productType domain.ProductType) (domain.Product, error) {
	// 1. Проверяем валидность типа товара (хотя хендлер тоже должен проверять)
	if productType != domain.TypeElectronics && productType != domain.TypeClothes && productType != domain.TypeShoes {
		slog.WarnContext(ctx, "Попытка добавить товар недопустимого типа", "pvz_id", pvzID, "type", productType)
		return domain.Product{}, errors.New("недопустимый тип товара")
	}

	// 2. Находим последнюю открытую приемку для этого ПВЗ
	openReception, err := s.repo.GetLastOpenReceptionByPVZ(ctx, pvzID)
	if err != nil {
		if errors.Is(err, repository.ErrReceptionNotFound) {
			slog.WarnContext(ctx, "Попытка добавить товар без открытой приемки", "pvz_id", pvzID)
			return domain.Product{}, errors.New("нет открытой приемки для данного ПВЗ, чтобы добавить товар")
		}
		// Другая ошибка при поиске приемки
		slog.ErrorContext(ctx, "Ошибка поиска открытой приемки", "pvz_id", pvzID, "error", err)
		return domain.Product{}, fmt.Errorf("ошибка поиска открытой приемки: %w", err)
	}
	slog.DebugContext(ctx, "Найдена открытая приемка для добавления товара", "reception_id", openReception.ID, "pvz_id", pvzID)

	// 3. Готовим данные товара для сохранения
	productToCreate := domain.Product{
		ReceptionID: openReception.ID, // Связываем с найденной приемкой
		Type:        productType,
		// ID и DateTimeAdded будут сгенерированы БД/репозиторием
	}

	// 4. Вызываем репозиторий для сохранения товара
	newProductID, err := s.repo.AddProductToReception(ctx, productToCreate)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка добавления товара в репозиторий", "reception_id", openReception.ID, "type", productType, "error", err)
		return domain.Product{}, fmt.Errorf("не удалось добавить товар в приемку: %w", err)
	}
	slog.InfoContext(ctx, "Товар успешно добавлен в репозиторий", "product_id", newProductID, "reception_id", openReception.ID)

	// --- ИСПРАВЛЕНИЕ: Формируем и возвращаем ЗАПОЛНЕННУЮ структуру ---
	addedProduct := domain.Product{
		ID:            newProductID,     // Используем ID, полученный от репозитория
		ReceptionID:   openReception.ID, // ID найденной открытой приемки
		Type:          productType,      // Тип, который передали на вход
		DateTimeAdded: time.Now(),       // Примерное время для ответа API (БД ставит точное)
	}

	return addedProduct, nil // Возвращаем созданный товар и nil ошибку
	// --- Конец исправления ---
}

// DeleteLastProduct - удаляет последний добавленный товар из открытой приемки
func (s *receptionService) DeleteLastProduct(ctx context.Context, pvzID uuid.UUID) error {
	// 1. Находим последнюю открытую приемку
	openReception, err := s.repo.GetLastOpenReceptionByPVZ(ctx, pvzID)
	if err != nil {
		if errors.Is(err, repository.ErrReceptionNotFound) {
			slog.WarnContext(ctx, "Попытка удалить товар без открытой приемки", "pvz_id", pvzID)
			return errors.New("нет открытой приемки для данного ПВЗ, чтобы удалить товар")
		}
		slog.ErrorContext(ctx, "Ошибка поиска открытой приемки при удалении товара", "pvz_id", pvzID, "error", err)
		return fmt.Errorf("ошибка поиска открытой приемки: %w", err)
	}

	// 2. Находим последний добавленный товар в этой приемке
	lastProduct, err := s.repo.GetLastProductFromReception(ctx, openReception.ID)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			slog.WarnContext(ctx, "Попытка удалить товар из пустой приемки", "reception_id", openReception.ID)
			return errors.New("в текущей открытой приемке нет товаров для удаления")
		}
		slog.ErrorContext(ctx, "Ошибка поиска последнего товара в приемке", "reception_id", openReception.ID, "error", err)
		return fmt.Errorf("ошибка поиска последнего товара: %w", err)
	}

	// 3. Удаляем найденный товар по его ID
	err = s.repo.DeleteProductByID(ctx, lastProduct.ID)
	if err != nil {
		// Обрабатываем случай, если товар уже был удален (хотя мы его только что нашли)
		if errors.Is(err, repository.ErrProductNotFound) { // Репозиторий должен вернуть эту ошибку, если RowsAffected=0
			slog.ErrorContext(ctx, "Ошибка удаления товара: товар не найден (возможно, удален параллельно)", "product_id", lastProduct.ID, "error", err)
			return errors.New("не удалось удалить товар, так как он не найден") // Ошибка для клиента
		}
		// Другая ошибка репозитория
		slog.ErrorContext(ctx, "Ошибка удаления товара из репозитория", "product_id", lastProduct.ID, "error", err)
		return fmt.Errorf("не удалось удалить товар: %w", err)
	}

	slog.InfoContext(ctx, "Последний товар успешно удален из приемки", "product_id", lastProduct.ID, "reception_id", openReception.ID)
	return nil
}

// CloseLastReception - закрывает последнюю открытую приемку
func (s *receptionService) CloseLastReception(ctx context.Context, pvzID uuid.UUID) (domain.Reception, error) {
	// 1. Находим последнюю открытую приемку
	openReception, err := s.repo.GetLastOpenReceptionByPVZ(ctx, pvzID)
	if err != nil {
		if errors.Is(err, repository.ErrReceptionNotFound) {
			slog.WarnContext(ctx, "Попытка закрыть приемку при отсутствии открытой", "pvz_id", pvzID)
			return domain.Reception{}, errors.New("нет открытой приемки для данного ПВЗ для закрытия")
		}
		slog.ErrorContext(ctx, "Ошибка поиска открытой приемки при закрытии", "pvz_id", pvzID, "error", err)
		return domain.Reception{}, fmt.Errorf("ошибка поиска открытой приемки: %w", err)
	}

	// 2. Вызываем метод репозитория для изменения статуса на 'closed'
	err = s.repo.CloseReceptionByID(ctx, openReception.ID)
	if err != nil {
		// Обрабатываем случай, если приемка уже была закрыта или не найдена
		if errors.Is(err, repository.ErrReceptionNotFound) { // Репозиторий должен вернуть это, если RowsAffected=0
			slog.ErrorContext(ctx, "Ошибка закрытия приемки: приемка не найдена или уже закрыта", "reception_id", openReception.ID, "error", err)
			return domain.Reception{}, errors.New("не удалось закрыть приемку, так как она не найдена или уже закрыта")
		}
		// Другая ошибка репозитория
		slog.ErrorContext(ctx, "Ошибка закрытия приемки в репозитории", "reception_id", openReception.ID, "error", err)
		return domain.Reception{}, fmt.Errorf("не удалось закрыть приемку: %w", err)
	}

	// 3. Формируем ответ с обновленным статусом
	closedReception := openReception             // Копируем данные найденной приемки
	closedReception.Status = domain.StatusClosed // Обновляем статус
	// Время DateTime остается временем начала приемки

	slog.InfoContext(ctx, "Приемка успешно закрыта", "reception_id", closedReception.ID, "pvz_id", pvzID)
	return closedReception, nil
}
