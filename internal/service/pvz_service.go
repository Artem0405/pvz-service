package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository"
	"github.com/google/uuid"

	mmetrics "github.com/Artem0405/pvz-service/internal/metrics"
)

// pvzService - реализация интерфейса PVZService.
type pvzService struct {
	pvzRepo       repository.PVZRepository
	receptionRepo repository.ReceptionRepository
}

// --- ИСПРАВЛЕНО: NewPVZService - конструктор ---
// Возвращаемый тип - ИНТЕРФЕЙС PVZService
func NewPVZService(pvzRepo repository.PVZRepository, receptionRepo repository.ReceptionRepository) PVZService {
	return &pvzService{ // Возвращаем указатель на структуру, реализующую интерфейс
		pvzRepo:       pvzRepo,
		receptionRepo: receptionRepo,
	}
}

// CreatePVZ (код без изменений)
func (s *pvzService) CreatePVZ(ctx context.Context, input domain.PVZ) (domain.PVZ, error) {
	if input.City != "Москва" && input.City != "Санкт-Петербург" && input.City != "Казань" {
		slog.WarnContext(ctx, "Попытка создания ПВЗ с недопустимым городом", slog.String("город", input.City))
		return domain.PVZ{}, errors.New("создание ПВЗ возможно только в городах: Москва, Санкт-Петербург, Казань")
	}

	pvzToCreate := domain.PVZ{City: input.City}
	newID, err := s.pvzRepo.CreatePVZ(ctx, pvzToCreate)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка репозитория при создании ПВЗ", slog.String("город", input.City), slog.Any("error", err))
		return domain.PVZ{}, fmt.Errorf("не удалось сохранить ПВЗ: %w", err)
	}

	mmetrics.PVZCreatedTotal.Inc()

	createdPVZ := domain.PVZ{
		ID:               newID,
		City:             input.City,
		RegistrationDate: time.Now(),
	}
	slog.InfoContext(ctx, "ПВЗ успешно создан", slog.String("pvz_id", newID.String()), slog.String("город", input.City))
	return createdPVZ, nil
}

// --- ИСПРАВЛЕНО: GetPVZList - реализация метода ---
// Сигнатура соответствует интерфейсу service.PVZService
// Возвращаемый тип - GetPVZListResult (определенный выше или в domain)
func (s *pvzService) GetPVZList(ctx context.Context, startDate, endDate *time.Time, limit int, afterRegistrationDate *time.Time, afterID *uuid.UUID) (GetPVZListResult, error) {
	// Инициализируем структуру результата
	result := GetPVZListResult{ // Используем тип GetPVZListResult
		Receptions: make(map[uuid.UUID][]domain.Reception),
		Products:   make(map[uuid.UUID][]domain.Product),
		PVZs:       []domain.PVZ{},
	}

	// 1. Получаем ПВЗ
	pvzList, err := s.pvzRepo.ListPVZs(ctx, limit, afterRegistrationDate, afterID) // Вызов репозитория соответствует интерфейсу
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка получения списка ПВЗ из репозитория", "error", err)
		return result, fmt.Errorf("не удалось получить список ПВЗ: %w", err)
	}
	result.PVZs = pvzList

	// 2. Определяем курсор для следующей страницы
	if len(pvzList) == limit {
		lastPVZ := pvzList[len(pvzList)-1]
		nextDate := lastPVZ.RegistrationDate         // Копируем значение
		nextID := lastPVZ.ID                         // Копируем значение
		result.NextAfterRegistrationDate = &nextDate // Присваиваем указатель полю структуры result
		result.NextAfterID = &nextID                 // Присваиваем указатель полю структуры result
	}

	// 3. Если ПВЗ нет, выходим
	if len(pvzList) == 0 {
		slog.DebugContext(ctx, "ПВЗ не найдены для данного курсора/фильтров")
		return result, nil
	}

	// 4. Собираем ID ПВЗ
	pvzIDs := make([]uuid.UUID, 0, len(pvzList))
	for _, pvz := range pvzList {
		pvzIDs = append(pvzIDs, pvz.ID)
	}

	// 5. Получаем Приемки
	receptions, err := s.receptionRepo.ListReceptionsByPVZIDs(ctx, pvzIDs, startDate, endDate)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка получения приемок для ПВЗ", "pvz_ids", pvzIDs, "error", err)
		return result, fmt.Errorf("не удалось получить приемки: %w", err)
	}

	// 6. Группируем Приемки и собираем ID
	receptionIDs := make([]uuid.UUID, 0, len(receptions))
	if len(receptions) > 0 {
		for _, rcp := range receptions {
			result.Receptions[rcp.PVZID] = append(result.Receptions[rcp.PVZID], rcp)
			receptionIDs = append(receptionIDs, rcp.ID)
		}
	} else {
		slog.DebugContext(ctx, "Приемки не найдены для ПВЗ на этой странице/фильтров", "pvz_ids", pvzIDs)
		return result, nil // Возвращаем ПВЗ без приемок
	}

	// 7. Получаем Товары
	if len(receptionIDs) > 0 {
		products, err := s.receptionRepo.ListProductsByReceptionIDs(ctx, receptionIDs)
		if err != nil {
			slog.ErrorContext(ctx, "Ошибка получения товаров для приемок", "reception_ids", receptionIDs, "error", err)
			return result, fmt.Errorf("не удалось получить товары: %w", err)
		}

		// 8. Группируем Товары
		for _, p := range products {
			result.Products[p.ReceptionID] = append(result.Products[p.ReceptionID], p)
		}
	}

	slog.DebugContext(ctx, "Список ПВЗ (keyset) с деталями успешно сформирован",
		"pvz_count_on_page", len(result.PVZs),
		"limit", limit,
		"hasNextPage", result.NextAfterID != nil,
	)
	// 9. Возвращаем результат
	return result, nil
}

// --- УДАЛИТЕ ЭТОТ БЛОК (СТРОКИ ~155 И ДАЛЕЕ), ОН ДУБЛИРУЕТ КОНСТРУКТОР ---
/*
func NewPVZService(pvzRepo repository.PVZRepository, receptionRepo repository.ReceptionRepository) *pvzService {
	return &pvzService{
		pvzRepo:       pvzRepo,
		receptionRepo: receptionRepo,
	}
}
*/
// --- КОНЕЦ УДАЛЯЕМОГО БЛОКА ---
