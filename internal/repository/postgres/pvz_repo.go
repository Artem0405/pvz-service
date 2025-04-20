package postgres

import (
	"context"
	"database/sql"
	"errors" // Теперь не используется для ListPVZs, но может понадобиться для других методов
	"fmt"    // Для форматирования ошибок
	"log"    // Для логирования (или slog, если хотите)
	"time"   // Для *time.Time в сигнатуре

	// Импортируем внутренний пакет с доменными моделями
	"github.com/Artem0405/pvz-service/internal/domain"

	// Внешние зависимости
	"github.com/Masterminds/squirrel" // SQL билдер
	"github.com/google/uuid"          // Для работы с UUID
)

// PVZRepo - реализация интерфейса repository.PVZRepository для PostgreSQL.
// Содержит методы для взаимодействия с таблицей 'pvz'.
type PVZRepo struct {
	db *sql.DB                       // Пул соединений с базой данных
	sq squirrel.StatementBuilderType // Экземпляр squirrel для построения запросов
}

// AddProductToReception implements repository.ReceptionRepository.
func (r *PVZRepo) AddProductToReception(ctx context.Context, product domain.Product) (uuid.UUID, error) {
	panic("unimplemented")
}

// CloseReceptionByID implements repository.ReceptionRepository.
func (r *PVZRepo) CloseReceptionByID(ctx context.Context, receptionID uuid.UUID) error {
	panic("unimplemented")
}

// CreateReception implements repository.ReceptionRepository.
func (r *PVZRepo) CreateReception(ctx context.Context, reception domain.Reception) (uuid.UUID, error) {
	panic("unimplemented")
}

// DeleteProductByID implements repository.ReceptionRepository.
func (r *PVZRepo) DeleteProductByID(ctx context.Context, productID uuid.UUID) error {
	panic("unimplemented")
}

// GetLastOpenReceptionByPVZ implements repository.ReceptionRepository.
func (r *PVZRepo) GetLastOpenReceptionByPVZ(ctx context.Context, pvzID uuid.UUID) (domain.Reception, error) {
	panic("unimplemented")
}

// GetLastProductFromReception implements repository.ReceptionRepository.
func (r *PVZRepo) GetLastProductFromReception(ctx context.Context, receptionID uuid.UUID) (domain.Product, error) {
	panic("unimplemented")
}

// ListProductsByReceptionIDs implements repository.ReceptionRepository.
func (r *PVZRepo) ListProductsByReceptionIDs(ctx context.Context, receptionIDs []uuid.UUID) ([]domain.Product, error) {
	panic("unimplemented")
}

// ListReceptionsByPVZIDs implements repository.ReceptionRepository.
func (r *PVZRepo) ListReceptionsByPVZIDs(ctx context.Context, pvzIDs []uuid.UUID, startDate *time.Time, endDate *time.Time) ([]domain.Reception, error) {
	panic("unimplemented")
}

// NewPVZRepo - конструктор для PVZRepo.
// Принимает пул соединений и инициализирует SQL билдер squirrel для PostgreSQL.
func NewPVZRepo(db *sql.DB) *PVZRepo {
	return &PVZRepo{
		db: db,
		// Устанавливаем формат плейсхолдеров для PostgreSQL ($1, $2, ...)
		sq: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// CreatePVZ - сохраняет новый ПВЗ в базу данных.
// Генерирует UUID для нового ПВЗ.
// Возвращает сгенерированный UUID или ошибку.
func (r *PVZRepo) CreatePVZ(ctx context.Context, pvz domain.PVZ) (uuid.UUID, error) {
	// Генерируем новый уникальный ID для записи, если он не предоставлен
	if pvz.ID == uuid.Nil {
		pvz.ID = uuid.New()
	}

	// Строим SQL запрос INSERT с помощью squirrel
	sqlQuery, args, err := r.sq.
		Insert("pvz").
		Columns("id", "city"). // registration_date имеет DEFAULT NOW()
		Values(pvz.ID, pvz.City).
		ToSql()
	if err != nil {
		return uuid.Nil, fmt.Errorf("ошибка построения SQL запроса для создания ПВЗ: %w", err)
	}

	// Выполняем SQL запрос к базе данных
	_, err = r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		// TODO: Рассмотреть возможность обработки специфических ошибок PostgreSQL
		return uuid.Nil, fmt.Errorf("ошибка выполнения SQL запроса для создания ПВЗ: %w", err)
	}

	return pvz.ID, nil
}

// ListPVZs - получает список ПВЗ из базы данных с использованием keyset pagination.
// Принимает лимит и опциональные курсоры (дата и ID последнего элемента предыдущей страницы).
// Возвращает срез domain.PVZ для текущей страницы и ошибку.
// Подсчет totalCount больше не выполняется.
func (r *PVZRepo) ListPVZs(ctx context.Context, limit int, afterRegistrationDate *time.Time, afterID *uuid.UUID) ([]domain.PVZ, error) {

	// Базовый SELECT с сортировкой (добавляем ID для стабильности)
	queryBuilder := r.sq.
		Select("id", "registration_date", "city").
		From("pvz").
		OrderBy("registration_date DESC", "id DESC"). // Важно добавить id DESC!
		Limit(uint64(limit))

	// Добавляем условие WHERE, если курсор предоставлен (ОБА параметра)
	if afterRegistrationDate != nil && afterID != nil {
		// Используем <= и < потому что сортировка DESC по registration_date и id
		// Ищем записи, которые либо "старше", либо "такого же возраста, но с меньшим ID"
		queryBuilder = queryBuilder.Where(
			squirrel.Or{
				squirrel.Lt{"registration_date": *afterRegistrationDate}, // registration_date < ?
				squirrel.And{ // ИЛИ (registration_date = ? AND id < ?)
					squirrel.Eq{"registration_date": *afterRegistrationDate},
					squirrel.Lt{"id": *afterID},
				},
			},
		)
	} else if afterRegistrationDate != nil || afterID != nil {
		// Ситуация, когда передан только один параметр курсора - некорректна
		// Хотя API может это проверять, добавим проверку и здесь для надежности
		return nil, errors.New("для keyset pagination необходимо передавать оба параметра курсора (after_registration_date и after_id) или ни одного")
	}

	// Генерируем SQL и аргументы
	sqlQuery, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения SQL для списка ПВЗ: %w", err)
	}

	// log.Printf("DEBUG: Executing ListPVZs: %s with args %v", sqlQuery, args) // Раскомментировать для отладки

	// Выполняем запрос
	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		log.Printf("!!! ОШИБКА выполнения SQL для списка ПВЗ: %v", err) // Используем стандартный log для простоты
		return nil, fmt.Errorf("ошибка выполнения SQL для списка ПВЗ: %w", err)
	}
	defer rows.Close()

	// Сканируем результаты
	pvzList := make([]domain.PVZ, 0, limit) // Создаем срез с capacity = limit
	for rows.Next() {
		var pvz domain.PVZ
		// Убедитесь, что порядок сканирования соответствует SELECT
		if err := rows.Scan(&pvz.ID, &pvz.RegistrationDate, &pvz.City); err != nil {
			log.Printf("!!! ОШИБКА сканирования строки ПВЗ: %v", err)
			// Можно вернуть ошибку или просто пропустить строку и залогировать
			// return nil, fmt.Errorf("ошибка сканирования строки ПВЗ: %w", err)
			continue // Пропускаем строку с ошибкой сканирования
		}
		pvzList = append(pvzList, pvz)
	}

	// Проверяем ошибки после итерации
	if err = rows.Err(); err != nil {
		log.Printf("!!! ОШИБКА итерации по результатам ПВЗ: %v", err)
		return nil, fmt.Errorf("ошибка итерации по результатам ПВЗ: %w", err)
	}

	// Возвращаем полученный список (totalCount больше не нужен)
	return pvzList, nil
}

// GetAllPVZs - реализует repository.PVZRepository.
// ВАЖНО: Эта реализация может быть неэффективной для очень больших таблиц.
// Используется в основном для gRPC, где пагинация не была реализована.
func (r *PVZRepo) GetAllPVZs(ctx context.Context) ([]domain.PVZ, error) {
	query, args, err := r.sq.
		Select("id", "registration_date", "city").
		From("pvz").
		OrderBy("registration_date DESC"). // Оставляем сортировку для консистентности
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения SQL для GetAllPVZs: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("!!! ОШИБКА выполнения SQL для GetAllPVZs: %v", err)
		return nil, fmt.Errorf("ошибка выполнения SQL для GetAllPVZs: %w", err)
	}
	defer rows.Close()

	pvzList := make([]domain.PVZ, 0)
	for rows.Next() {
		var pvz domain.PVZ
		if err := rows.Scan(&pvz.ID, &pvz.RegistrationDate, &pvz.City); err != nil {
			log.Printf("!!! ОШИБКА сканирования строки ПВЗ в GetAllPVZs: %v", err)
			continue
		}
		pvzList = append(pvzList, pvz)
	}
	if err = rows.Err(); err != nil {
		log.Printf("!!! ОШИБКА итерации по результатам GetAllPVZs: %v", err)
		return nil, fmt.Errorf("ошибка итерации по результатам GetAllPVZs: %w", err)
	}

	return pvzList, nil
}
