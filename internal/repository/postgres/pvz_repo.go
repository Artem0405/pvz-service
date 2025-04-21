package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"      // Для форматирования ошибок
	"log/slog" // --- ИСПОЛЬЗУЕМ SLOG ---
	"time"     // Для *time.Time в сигнатуре

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
	// logger *slog.Logger // Опционально: добавить логгер как поле, если он нужен часто
}

// NewPVZRepo - конструктор для PVZRepo.
// Принимает пул соединений и инициализирует SQL билдер squirrel для PostgreSQL.
func NewPVZRepo(db *sql.DB) *PVZRepo { // Возвращаем *PVZRepo (конкретный тип)
	return &PVZRepo{
		db: db,
		// Устанавливаем формат плейсхолдеров для PostgreSQL ($1, $2, ...)
		sq: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
		// logger: slog.Default().With("repo", "pvz"), // Опционально: инициализировать логгер с атрибутом
	}
}

// CreatePVZ - сохраняет новый ПВЗ в базу данных.
// Генерирует UUID для нового ПВЗ, если он не предоставлен.
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
		// Используем slog для ошибки построения запроса
		slog.ErrorContext(ctx, "Ошибка построения SQL для создания ПВЗ", slog.Any("error", err))
		return uuid.Nil, fmt.Errorf("ошибка построения SQL запроса для создания ПВЗ: %w", err)
	}

	// Выполняем SQL запрос к базе данных
	_, err = r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		// Используем slog для ошибки выполнения запроса
		slog.ErrorContext(ctx, "Ошибка выполнения SQL для создания ПВЗ",
			slog.String("query", sqlQuery), // Логируем сам запрос (без аргументов из соображений безопасности)
			// slog.Any("args", args), // Осторожно: не логируйте чувствительные данные!
			slog.Any("error", err),
		)
		// TODO: Рассмотреть возможность обработки специфических ошибок PostgreSQL
		return uuid.Nil, fmt.Errorf("ошибка выполнения SQL запроса для создания ПВЗ: %w", err)
	}

	// Возвращаем ID успешно созданного ПВЗ
	return pvz.ID, nil
}

// ListPVZs - получает список ПВЗ из базы данных с использованием keyset pagination.
// Принимает лимит и опциональные курсоры (дата и ID последнего элемента предыдущей страницы).
// Возвращает срез domain.PVZ для текущей страницы и ошибку.
func (r *PVZRepo) ListPVZs(ctx context.Context, limit int, afterRegistrationDate *time.Time, afterID *uuid.UUID) ([]domain.PVZ, error) {

	// Базовый SELECT с сортировкой
	queryBuilder := r.sq.
		Select("id", "registration_date", "city").
		From("pvz").
		OrderBy("registration_date DESC", "id DESC").
		Limit(uint64(limit))

	// Добавляем условие WHERE для курсора
	if afterRegistrationDate != nil && afterID != nil {
		queryBuilder = queryBuilder.Where(
			squirrel.Or{
				squirrel.Lt{"registration_date": *afterRegistrationDate},
				squirrel.And{
					squirrel.Eq{"registration_date": *afterRegistrationDate},
					squirrel.Lt{"id": *afterID},
				},
			},
		)
	} else if afterRegistrationDate != nil || afterID != nil {
		return nil, errors.New("для keyset pagination необходимо передавать оба параметра курсора (after_registration_date и after_id) или ни одного")
	}

	// Генерируем SQL
	sqlQuery, args, err := queryBuilder.ToSql()
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка построения SQL для списка ПВЗ", slog.Any("error", err))
		return nil, fmt.Errorf("ошибка построения SQL для списка ПВЗ: %w", err)
	}

	slog.DebugContext(ctx, "Выполнение SQL для списка ПВЗ", slog.String("query", sqlQuery), slog.Any("args", args)) // Используем Debug

	// Выполняем запрос
	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		// Используем slog
		slog.ErrorContext(ctx, "Ошибка выполнения SQL для списка ПВЗ", slog.String("query", sqlQuery), slog.Any("error", err))
		return nil, fmt.Errorf("ошибка выполнения SQL для списка ПВЗ: %w", err)
	}
	defer rows.Close()

	// Сканируем результаты
	pvzList := make([]domain.PVZ, 0, limit)
	for rows.Next() {
		var pvz domain.PVZ
		if err := rows.Scan(&pvz.ID, &pvz.RegistrationDate, &pvz.City); err != nil {
			// Используем slog для ошибки сканирования
			slog.WarnContext(ctx, "Ошибка сканирования строки ПВЗ", slog.Any("error", err)) // Warn, т.к. продолжаем
			continue
		}
		pvzList = append(pvzList, pvz)
	}

	// Проверяем ошибки после итерации
	if err = rows.Err(); err != nil {
		// Используем slog
		slog.ErrorContext(ctx, "Ошибка итерации по результатам ПВЗ", slog.Any("error", err))
		return nil, fmt.Errorf("ошибка итерации по результатам ПВЗ: %w", err)
	}

	return pvzList, nil
}

// GetAllPVZs - реализует repository.PVZRepository.
// Используется в основном для gRPC.
func (r *PVZRepo) GetAllPVZs(ctx context.Context) ([]domain.PVZ, error) {
	// Используем Warn, т.к. этот метод может быть неэффективным
	slog.WarnContext(ctx, "Вызов неэффективного метода GetAllPVZs")

	query, args, err := r.sq.
		Select("id", "registration_date", "city").
		From("pvz").
		OrderBy("registration_date DESC").
		ToSql()
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка построения SQL для GetAllPVZs", slog.Any("error", err))
		return nil, fmt.Errorf("ошибка построения SQL для GetAllPVZs: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка выполнения SQL для GetAllPVZs", slog.String("query", query), slog.Any("error", err))
		return nil, fmt.Errorf("ошибка выполнения SQL для GetAllPVZs: %w", err)
	}
	defer rows.Close()

	pvzList := make([]domain.PVZ, 0)
	for rows.Next() {
		var pvz domain.PVZ
		if err := rows.Scan(&pvz.ID, &pvz.RegistrationDate, &pvz.City); err != nil {
			slog.WarnContext(ctx, "Ошибка сканирования строки ПВЗ в GetAllPVZs", slog.Any("error", err))
			continue
		}
		pvzList = append(pvzList, pvz)
	}
	if err = rows.Err(); err != nil {
		slog.ErrorContext(ctx, "Ошибка итерации по результатам GetAllPVZs", slog.Any("error", err))
		return nil, fmt.Errorf("ошибка итерации по результатам GetAllPVZs: %w", err)
	}

	return pvzList, nil
}

// --- УДАЛЕНЫ ЗАГЛУШКИ МЕТОДОВ ReceptionRepository ---
// Реализация этих методов должна находиться в internal/repository/postgres/reception_repo.go
// в структуре ReceptionRepo
