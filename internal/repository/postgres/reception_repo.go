package postgres

import (
	"context"
	"database/sql"
	"errors" // Для проверки sql.ErrNoRows
	"fmt"
	"log/slog" // --- ИСПОЛЬЗУЕМ SLOG ---
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository" // Для использования ErrReceptionNotFound, ErrProductNotFound
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

// ReceptionRepo - реализация ReceptionRepository для PostgreSQL
type ReceptionRepo struct {
	db *sql.DB
	sq squirrel.StatementBuilderType
	// logger *slog.Logger // Опционально: логгер
}

// NewReceptionRepo - конструктор для ReceptionRepo
func NewReceptionRepo(db *sql.DB) *ReceptionRepo { // Возвращаем *ReceptionRepo
	return &ReceptionRepo{
		db: db,
		sq: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
		// logger: slog.Default().With("repo", "reception"), // Опционально
	}
}

// CreateReception - сохраняет новую приемку
func (r *ReceptionRepo) CreateReception(ctx context.Context, reception domain.Reception) (uuid.UUID, error) {
	if reception.ID == uuid.Nil { // Генерируем ID, если не предоставлен
		reception.ID = uuid.New()
	}
	// Устанавливаем статус по умолчанию, если не задан
	if reception.Status == "" {
		reception.Status = domain.StatusInProgress
	}

	sqlQuery, args, err := r.sq.
		Insert("receptions").
		Columns("id", "pvz_id", "status"). // date_time по умолчанию NOW() в БД
		Values(reception.ID, reception.PVZID, reception.Status).
		ToSql()
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка построения SQL для создания приемки", slog.Any("error", err))
		return uuid.Nil, fmt.Errorf("ошибка построения SQL для создания приемки: %w", err)
	}

	_, err = r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка выполнения SQL для создания приемки", slog.String("query", sqlQuery), slog.Any("error", err))
		return uuid.Nil, fmt.Errorf("ошибка выполнения SQL для создания приемки: %w", err)
	}

	return reception.ID, nil
}

// GetLastOpenReceptionByPVZ - ищет последнюю открытую приемку для ПВЗ
func (r *ReceptionRepo) GetLastOpenReceptionByPVZ(ctx context.Context, pvzID uuid.UUID) (domain.Reception, error) {
	var reception domain.Reception

	sqlQuery, args, err := r.sq.
		Select("id", "pvz_id", "date_time", "status").
		From("receptions").
		Where(squirrel.Eq{"pvz_id": pvzID, "status": domain.StatusInProgress}).
		OrderBy("date_time DESC").
		Limit(1).
		ToSql()
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка построения SQL для поиска открытой приемки", slog.Any("pvz_id", pvzID), slog.Any("error", err))
		return domain.Reception{}, fmt.Errorf("ошибка построения SQL для поиска открытой приемки: %w", err)
	}

	err = r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(
		&reception.ID,
		&reception.PVZID,
		&reception.DateTime,
		&reception.Status,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Info или Debug, так как это ожидаемый случай "не найдено"
			slog.DebugContext(ctx, "Открытая приемка не найдена", slog.Any("pvz_id", pvzID))
			return domain.Reception{}, repository.ErrReceptionNotFound // Возвращаем нашу ошибку
		}
		// Другая ошибка - логируем как Error
		slog.ErrorContext(ctx, "Ошибка выполнения/сканирования SQL для поиска открытой приемки", slog.Any("pvz_id", pvzID), slog.String("query", sqlQuery), slog.Any("error", err))
		return domain.Reception{}, fmt.Errorf("ошибка выполнения SQL для поиска открытой приемки: %w", err)
	}

	return reception, nil
}

// AddProductToReception - добавляет товар в указанную приемку
func (r *ReceptionRepo) AddProductToReception(ctx context.Context, product domain.Product) (uuid.UUID, error) {
	if product.ID == uuid.Nil { // Генерируем ID для товара
		product.ID = uuid.New()
	}

	sqlQuery, args, err := r.sq.
		Insert("products").
		Columns("id", "reception_id", "type"). // date_time_added по умолчанию NOW() в БД
		Values(product.ID, product.ReceptionID, product.Type).
		ToSql()
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка построения SQL для добавления товара", slog.Any("error", err))
		return uuid.Nil, fmt.Errorf("ошибка построения SQL для добавления товара: %w", err)
	}

	_, err = r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		// TODO: Обработать специфические ошибки БД (например, неверный reception_id)
		slog.ErrorContext(ctx, "Ошибка выполнения SQL для добавления товара", slog.String("query", sqlQuery), slog.Any("error", err))
		return uuid.Nil, fmt.Errorf("ошибка выполнения SQL для добавления товара: %w", err)
	}

	return product.ID, nil
}

// GetLastProductFromReception находит последний добавленный товар в приемке
func (r *ReceptionRepo) GetLastProductFromReception(ctx context.Context, receptionID uuid.UUID) (domain.Product, error) {
	var product domain.Product

	sqlQuery, args, err := r.sq.
		Select("id", "reception_id", "date_time_added", "type").
		From("products").
		Where(squirrel.Eq{"reception_id": receptionID}).
		OrderBy("date_time_added DESC").
		Limit(1).
		ToSql()
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка построения SQL для поиска последнего товара", slog.Any("reception_id", receptionID), slog.Any("error", err))
		return domain.Product{}, fmt.Errorf("ошибка построения SQL для поиска последнего товара: %w", err)
	}

	err = r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(
		&product.ID,
		&product.ReceptionID,
		&product.DateTimeAdded,
		&product.Type, // Убедитесь, что тип domain.ProductType корректно сканируется из БД
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.DebugContext(ctx, "Товары не найдены в приемке", slog.Any("reception_id", receptionID))
			return domain.Product{}, repository.ErrProductNotFound // Используем нашу ошибку
		}
		slog.ErrorContext(ctx, "Ошибка выполнения/сканирования SQL для поиска последнего товара", slog.Any("reception_id", receptionID), slog.String("query", sqlQuery), slog.Any("error", err))
		return domain.Product{}, fmt.Errorf("ошибка выполнения SQL для поиска последнего товара: %w", err)
	}

	return product, nil
}

// DeleteProductByID удаляет товар по ID
func (r *ReceptionRepo) DeleteProductByID(ctx context.Context, productID uuid.UUID) error {
	sqlQuery, args, err := r.sq.
		Delete("products").
		Where(squirrel.Eq{"id": productID}).
		ToSql()
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка построения SQL для удаления товара", slog.Any("product_id", productID), slog.Any("error", err))
		return fmt.Errorf("ошибка построения SQL для удаления товара: %w", err)
	}

	result, err := r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка выполнения SQL для удаления товара", slog.Any("product_id", productID), slog.String("query", sqlQuery), slog.Any("error", err))
		return fmt.Errorf("ошибка выполнения SQL для удаления товара: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Не критично, но логируем как Warn
		slog.WarnContext(ctx, "Не удалось получить количество удаленных строк", slog.Any("product_id", productID), slog.Any("error", err))
		return nil // Ошибку не возвращаем
	}
	if rowsAffected == 0 {
		slog.WarnContext(ctx, "Попытка удалить несуществующий товар", slog.Any("product_id", productID))
		return repository.ErrProductNotFound // Возвращаем ошибку "не найдено"
	}

	slog.InfoContext(ctx, "Товар успешно удален", slog.Any("product_id", productID))
	return nil // Успешное удаление
}

// CloseReceptionByID изменяет статус приемки на 'closed'
func (r *ReceptionRepo) CloseReceptionByID(ctx context.Context, receptionID uuid.UUID) error {
	sqlQuery, args, err := r.sq.
		Update("receptions").
		Set("status", domain.StatusClosed).
		Where(squirrel.Eq{"id": receptionID, "status": domain.StatusInProgress}).
		ToSql()
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка построения SQL для закрытия приемки", slog.Any("reception_id", receptionID), slog.Any("error", err))
		return fmt.Errorf("ошибка построения SQL для закрытия приемки: %w", err)
	}

	result, err := r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка выполнения SQL для закрытия приемки", slog.Any("reception_id", receptionID), slog.String("query", sqlQuery), slog.Any("error", err))
		return fmt.Errorf("ошибка выполнения SQL для закрытия приемки: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		slog.WarnContext(ctx, "Не удалось получить количество обновленных строк при закрытии приемки", slog.Any("reception_id", receptionID), slog.Any("error", err))
		return nil // Запрос прошел, ошибку не возвращаем
	}

	if rowsAffected == 0 {
		slog.WarnContext(ctx, "Попытка закрыть не найденную или уже закрытую приемку", slog.Any("reception_id", receptionID))
		return repository.ErrReceptionNotFound // Используем ошибку "не найдено"
	}

	slog.InfoContext(ctx, "Приемка успешно закрыта", slog.Any("reception_id", receptionID))
	return nil // Успешное закрытие
}

// ListReceptionsByPVZIDs возвращает приемки для списка ПВЗ с фильтром по дате
func (r *ReceptionRepo) ListReceptionsByPVZIDs(ctx context.Context, pvzIDs []uuid.UUID, startDate, endDate *time.Time) ([]domain.Reception, error) {
	if len(pvzIDs) == 0 {
		return []domain.Reception{}, nil
	}

	queryBuilder := r.sq.
		Select("id", "pvz_id", "date_time", "status").
		From("receptions").
		Where(squirrel.Eq{"pvz_id": pvzIDs}).
		OrderBy("pvz_id, date_time DESC")

	if startDate != nil {
		queryBuilder = queryBuilder.Where(squirrel.GtOrEq{"date_time": *startDate})
	}
	if endDate != nil {
		queryBuilder = queryBuilder.Where(squirrel.LtOrEq{"date_time": *endDate})
	}

	sqlQuery, args, err := queryBuilder.ToSql()
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка построения SQL для получения списка приемок", slog.Any("error", err))
		return nil, fmt.Errorf("ошибка построения SQL для получения списка приемок: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка выполнения SQL для получения списка приемок", slog.String("query", sqlQuery), slog.Any("error", err))
		return nil, fmt.Errorf("ошибка выполнения SQL для получения списка приемок: %w", err)
	}
	defer rows.Close()

	receptions := make([]domain.Reception, 0) // Инициализируем пустой слайс
	for rows.Next() {
		var rcp domain.Reception
		if err := rows.Scan(&rcp.ID, &rcp.PVZID, &rcp.DateTime, &rcp.Status); err != nil {
			slog.WarnContext(ctx, "Ошибка сканирования строки приемки", slog.Any("error", err))
			continue
		}
		receptions = append(receptions, rcp)
	}
	if err = rows.Err(); err != nil {
		slog.ErrorContext(ctx, "Ошибка итерации по результатам приемок", slog.Any("error", err))
		return nil, fmt.Errorf("ошибка итерации по результатам приемок: %w", err)
	}

	return receptions, nil
}

// ListProductsByReceptionIDs возвращает товары для списка приемок
func (r *ReceptionRepo) ListProductsByReceptionIDs(ctx context.Context, receptionIDs []uuid.UUID) ([]domain.Product, error) {
	if len(receptionIDs) == 0 {
		return []domain.Product{}, nil
	}

	sqlQuery, args, err := r.sq.
		Select("id", "reception_id", "date_time_added", "type").
		From("products").
		Where(squirrel.Eq{"reception_id": receptionIDs}).
		OrderBy("reception_id, date_time_added ASC").
		ToSql()
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка построения SQL для получения списка товаров", slog.Any("error", err))
		return nil, fmt.Errorf("ошибка построения SQL для получения списка товаров: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		slog.ErrorContext(ctx, "Ошибка выполнения SQL для получения списка товаров", slog.String("query", sqlQuery), slog.Any("error", err))
		return nil, fmt.Errorf("ошибка выполнения SQL для получения списка товаров: %w", err)
	}
	defer rows.Close()

	products := make([]domain.Product, 0) // Инициализируем пустой слайс
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.ReceptionID, &p.DateTimeAdded, &p.Type); err != nil {
			slog.WarnContext(ctx, "Ошибка сканирования строки товара", slog.Any("error", err))
			continue
		}
		products = append(products, p)
	}
	if err = rows.Err(); err != nil {
		slog.ErrorContext(ctx, "Ошибка итерации по результатам товаров", slog.Any("error", err))
		return nil, fmt.Errorf("ошибка итерации по результатам товаров: %w", err)
	}

	return products, nil
}

// --- УДАЛЕНЫ ЗАГЛУШКИ МЕТОДОВ PVZRepository ---
// Реализация этих методов должна находиться в internal/repository/postgres/pvz_repo.go
