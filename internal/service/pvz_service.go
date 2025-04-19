package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time" // Необходим для GetPVZList и формирования ответа в CreatePVZ

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository"
	"github.com/google/uuid" // Необходим для ключей карт в GetPVZList

	// --- ИМПОРТ ПАКЕТА С МЕТРИКАМИ ---
	// Предполагается, что вы создадите этот пакет и определите в нем
	// экспортируемую метрику: var PVZCreatedTotal = promauto.NewCounter(...)
	mmetrics "github.com/Artem0405/pvz-service/internal/metrics"
	// ---------------------------------
)

// pvzService - реализация интерфейса PVZService.
// Содержит бизнес-логику для управления ПВЗ.
// Использует интерфейсы репозиториев для взаимодействия с хранилищем данных.
type pvzService struct {
	pvzRepo       repository.PVZRepository       // Репозиторий для работы с ПВЗ
	receptionRepo repository.ReceptionRepository // Репозиторий для работы с приемками (нужен для GetPVZList)
}

// NewPVZService - конструктор для создания экземпляра pvzService.
// Принимает интерфейсы репозиториев в качестве зависимостей.
func NewPVZService(pvzRepo repository.PVZRepository, receptionRepo repository.ReceptionRepository) *pvzService {
	return &pvzService{
		pvzRepo:       pvzRepo,
		receptionRepo: receptionRepo,
	}
}

// CreatePVZ - создает новый ПВЗ после валидации входных данных.
func (s *pvzService) CreatePVZ(ctx context.Context, input domain.PVZ) (domain.PVZ, error) {
	// 1. Валидация входных данных (город)
	if input.City != "Москва" && input.City != "Санкт-Петербург" && input.City != "Казань" {
		// Логируем как предупреждение (Warn), так как это ошибка ввода пользователя, а не системы
		slog.WarnContext(ctx, "Попытка создания ПВЗ с недопустимым городом", slog.String("город", input.City))
		return domain.PVZ{}, errors.New("создание ПВЗ возможно только в городах: Москва, Санкт-Петербург, Казань")
	}

	// 2. Вызов репозитория ПВЗ для сохранения данных
	// Передаем только те поля, которые нужны для создания в БД
	pvzToCreate := domain.PVZ{
		City: input.City,
	}
	newID, err := s.pvzRepo.CreatePVZ(ctx, pvzToCreate)
	if err != nil {
		// Логируем как ошибку (Error), так как проблема на уровне БД/репозитория
		slog.ErrorContext(ctx, "Ошибка репозитория при создании ПВЗ",
			slog.String("город", input.City),
			slog.Any("error", err), // Используем Any для ошибки
		)
		return domain.PVZ{}, fmt.Errorf("не удалось сохранить ПВЗ: %w", err)
	}

	// --- ИНКРЕМЕНТ БИЗНЕС-МЕТРИКИ ---
	mmetrics.PVZCreatedTotal.Inc()
	slog.InfoContext(ctx, "Инкрементирована метрика pvz_created_total")
	// ---------------------------------

	// 3. Формирование ответа
	// Заполняем структуру ответа, включая сгенерированный ID и дату (приблизительную)
	createdPVZ := domain.PVZ{
		ID:               newID,
		City:             input.City,
		RegistrationDate: time.Now(), // Генерируем дату для ответа API, БД сама ставит точную
	}

	// Логируем успешное создание как информацию (Info)
	slog.InfoContext(ctx, "ПВЗ успешно создан", slog.String("pvz_id", newID.String()), slog.String("город", input.City))

	return createdPVZ, nil
}

// GetPVZList - реализация метода получения списка ПВЗ с деталями (приемками и товарами).
// Возвращает GetPVZListResult с доменными моделями.
func (s *pvzService) GetPVZList(ctx context.Context, startDate, endDate *time.Time, page, limit int) (domain.GetPVZListResult, error) {
	// Инициализируем пустую структуру результата
	// Важно инициализировать карты, чтобы избежать nil pointer panic позже
	result := domain.GetPVZListResult{
		Receptions: make(map[uuid.UUID][]domain.Reception),
		Products:   make(map[uuid.UUID][]domain.Product),
		PVZs:       []domain.PVZ{}, // Инициализируем пустым слайсом
	}

	// 1. Получаем страницу ПВЗ и общее количество из репозитория ПВЗ
	pvzList, totalCount, err := s.pvzRepo.ListPVZs(ctx, page, limit)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка получения списка ПВЗ из репозитория", "error", err, "page", page, "limit", limit)
		return result, fmt.Errorf("не удалось получить список ПВЗ: %w", err)
	}
	result.TotalPVZs = totalCount

	// Если ПВЗ на этой странице (или вообще) нет, возвращаем результат с пустым списком PVZs
	if len(pvzList) == 0 {
		slog.InfoContext(ctx, "ПВЗ не найдены для данной страницы/фильтров", "page", page, "limit", limit, "startDate", startDate, "endDate", endDate)
		// result уже инициализирован с пустым PVZs и totalCount = 0 (или актуальным)
		return result, nil
	}
	result.PVZs = pvzList // Сохраняем найденные ПВЗ

	// 2. Собираем ID полученных ПВЗ для последующих запросов
	pvzIDs := make([]uuid.UUID, 0, len(pvzList))
	for _, pvz := range pvzList {
		pvzIDs = append(pvzIDs, pvz.ID)
	}

	// 3. Получаем все приемки для этих ПВЗ с учетом фильтров по дате
	receptions, err := s.receptionRepo.ListReceptionsByPVZIDs(ctx, pvzIDs, startDate, endDate)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка получения приемок для ПВЗ", "pvz_ids", pvzIDs, "error", err)
		// Возвращаем ошибку, т.к. без приемок не можем показать полную картину
		return result, fmt.Errorf("не удалось получить приемки: %w", err)
	}

	// Если приемок нет (из-за фильтров или их отсутствия), нет смысла запрашивать товары
	if len(receptions) == 0 {
		slog.InfoContext(ctx, "Приемки не найдены для ПВЗ на этой странице/фильтров", "pvz_ids", pvzIDs, "startDate", startDate, "endDate", endDate)
		// result уже содержит PVZs и пустые карты Receptions/Products
		return result, nil
	}

	// Группируем приемки по ID ПВЗ и собираем ID приемок для запроса товаров
	receptionIDs := make([]uuid.UUID, 0, len(receptions))
	for _, rcp := range receptions {
		result.Receptions[rcp.PVZID] = append(result.Receptions[rcp.PVZID], rcp)
		receptionIDs = append(receptionIDs, rcp.ID)
	}

	// 4. Получаем все товары для найденных приемок
	products, err := s.receptionRepo.ListProductsByReceptionIDs(ctx, receptionIDs)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка получения товаров для приемок", "reception_ids", receptionIDs, "error", err)
		// Возвращаем ошибку, т.к. запросили приемки, но не смогли получить товары
		return result, fmt.Errorf("не удалось получить товары: %w", err)
	}

	// Группируем товары по ID приемки
	for _, p := range products {
		result.Products[p.ReceptionID] = append(result.Products[p.ReceptionID], p)
	}

	slog.InfoContext(ctx, "Список ПВЗ с деталями успешно сформирован", "page", page, "limit", limit, "pvz_count_on_page", len(result.PVZs), "total_pvz_count", result.TotalPVZs)
	// 5. Возвращаем собранную структуру с доменными данными
	return result, nil
}
