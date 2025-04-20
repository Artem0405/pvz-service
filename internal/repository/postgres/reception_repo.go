package postgres

import (
	"context"
	"database/sql"
	"errors" // Для проверки sql.ErrNoRows
	"fmt"
	"log"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository" // Для использования ErrReceptionNotFound
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

// ReceptionRepo - реализация ReceptionRepository для PostgreSQL
type ReceptionRepo struct {
	db *sql.DB
	sq squirrel.StatementBuilderType
}

// CreatePVZ implements repository.PVZRepository.
func (r *ReceptionRepo) CreatePVZ(ctx context.Context, pvz domain.PVZ) (uuid.UUID, error) {
	panic("unimplemented")
}

// GetAllPVZs implements repository.PVZRepository.
func (r *ReceptionRepo) GetAllPVZs(ctx context.Context) ([]domain.PVZ, error) {
	panic("unimplemented")
}

// ListPVZs implements repository.PVZRepository.
func (r *ReceptionRepo) ListPVZs(ctx context.Context, page int, limit int) ([]domain.PVZ, int, error) {
	panic("unimplemented")
}

// NewReceptionRepo - конструктор для ReceptionRepo
func NewReceptionRepo(db *sql.DB) *ReceptionRepo {
	return &ReceptionRepo{
		db: db,
		sq: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// CreateReception - сохраняет новую приемку
func (r *ReceptionRepo) CreateReception(ctx context.Context, reception domain.Reception) (uuid.UUID, error) {
	reception.ID = uuid.New()                  // Генерируем ID
	reception.Status = domain.StatusInProgress // Устанавливаем статус по умолчанию

	sqlQuery, args, err := r.sq.
		Insert("receptions").
		Columns("id", "pvz_id", "status"). // date_time по умолчанию NOW()
		Values(reception.ID, reception.PVZID, reception.Status).
		ToSql()
	if err != nil {
		return uuid.Nil, fmt.Errorf("ошибка построения SQL для создания приемки: %w", err)
	}

	_, err = r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return uuid.Nil, fmt.Errorf("ошибка выполнения SQL для создания приемки: %w", err)
	}

	return reception.ID, nil
}

// GetLastOpenReceptionByPVZ - ищет последнюю открытую приемку для ПВЗ
func (r *ReceptionRepo) GetLastOpenReceptionByPVZ(ctx context.Context, pvzID uuid.UUID) (domain.Reception, error) {
	var reception domain.Reception

	// Ищем одну запись со статусом 'in_progress' для данного pvz_id,
	// сортируем по дате создания (по убыванию), чтобы взять самую последнюю.
	sqlQuery, args, err := r.sq.
		Select("id", "pvz_id", "date_time", "status").
		From("receptions").
		Where(squirrel.Eq{"pvz_id": pvzID, "status": domain.StatusInProgress}).
		OrderBy("date_time DESC"). // Берем самую свежую открытую
		Limit(1).
		ToSql()
	if err != nil {
		return domain.Reception{}, fmt.Errorf("ошибка построения SQL для поиска открытой приемки: %w", err)
	}

	// Выполняем запрос и сканируем результат в структуру reception
	err = r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(
		&reception.ID,
		&reception.PVZID,
		&reception.DateTime,
		&reception.Status,
	)
	if err != nil {
		// Если QueryRowContext не нашел строк, он возвращает sql.ErrNoRows
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Reception{}, repository.ErrReceptionNotFound // Возвращаем нашу ошибку
		}
		// Другая ошибка при выполнении запроса
		return domain.Reception{}, fmt.Errorf("ошибка выполнения SQL для поиска открытой приемки: %w", err)
	}

	return reception, nil
}

// AddProductToReception - добавляет товар в указанную приемку
func (r *ReceptionRepo) AddProductToReception(ctx context.Context, product domain.Product) (uuid.UUID, error) {
	product.ID = uuid.New() // Генерируем ID для товара

	sqlQuery, args, err := r.sq.
		Insert("products").
		Columns("id", "reception_id", "type"). // date_time_added по умолчанию NOW()
		Values(product.ID, product.ReceptionID, product.Type).
		ToSql()
	if err != nil {
		return uuid.Nil, fmt.Errorf("ошибка построения SQL для добавления товара: %w", err)
	}

	_, err = r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		// TODO: Обработать ошибки (например, неверный reception_id, неверный product_type)
		return uuid.Nil, fmt.Errorf("ошибка выполнения SQL для добавления товара: %w", err)
	}

	return product.ID, nil
}

// GetLastProductFromReception находит последний добавленный товар в приемке
func (r *ReceptionRepo) GetLastProductFromReception(ctx context.Context, receptionID uuid.UUID) (domain.Product, error) {
	var product domain.Product

	// Ищем один товар для данной приемки, сортируем по времени добавления ПО УБЫВАНИЮ, берем первый
	sqlQuery, args, err := r.sq.
		Select("id", "reception_id", "date_time_added", "type").
		From("products").
		Where(squirrel.Eq{"reception_id": receptionID}).
		OrderBy("date_time_added DESC"). // LIFO - последний добавленный
		Limit(1).
		ToSql()
	if err != nil {
		return domain.Product{}, fmt.Errorf("ошибка построения SQL для поиска последнего товара: %w", err)
	}

	err = r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(
		&product.ID,
		&product.ReceptionID,
		&product.DateTimeAdded,
		&product.Type,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Используем нашу ошибку ErrProductNotFound
			return domain.Product{}, repository.ErrProductNotFound
		}
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
		return fmt.Errorf("ошибка построения SQL для удаления товара: %w", err)
	}

	// Используем ExecContext и проверяем количество удаленных строк
	result, err := r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("ошибка выполнения SQL для удаления товара: %w", err)
	}

	// Проверяем, была ли удалена хотя бы одна строка
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Не критично, но логируем
		log.Printf("Предупреждение: не удалось получить количество удаленных строк при удалении товара %s: %v", productID, err)
		return nil // Ошибку не возвращаем, т.к. запрос выполнился
	}
	if rowsAffected == 0 {
		// Это странно, если мы только что нашли этот ID. Возможно, гонка состояний.
		// Возвращаем ошибку "не найдено", чтобы сервис мог ее обработать.
		log.Printf("Попытка удалить товар %s, но он не найден (rowsAffected=0)", productID)
		return repository.ErrProductNotFound
	}

	return nil // Успешное удаление
}

// CloseReceptionByID изменяет статус приемки на 'closed'
func (r *ReceptionRepo) CloseReceptionByID(ctx context.Context, receptionID uuid.UUID) error {
	sqlQuery, args, err := r.sq.
		Update("receptions").
		Set("status", domain.StatusClosed).                                       // Устанавливаем новый статус
		Where(squirrel.Eq{"id": receptionID, "status": domain.StatusInProgress}). // Обновляем только если ID совпадает и статус 'in_progress'
		ToSql()
	if err != nil {
		return fmt.Errorf("ошибка построения SQL для закрытия приемки: %w", err)
	}

	// Выполняем запрос и проверяем, была ли обновлена хотя бы одна строка
	result, err := r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("ошибка выполнения SQL для закрытия приемки: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Предупреждение: не удалось получить количество обновленных строк при закрытии приемки %s: %v", receptionID, err)
		return nil // Ошибку не возвращаем, запрос прошел
	}

	// Если ни одна строка не была обновлена, значит приемка с таким ID не найдена
	// или она уже была закрыта. Возвращаем ошибку "не найдено".
	if rowsAffected == 0 {
		log.Printf("Попытка закрыть приемку %s, но она не найдена или уже закрыта (rowsAffected=0)", receptionID)
		return repository.ErrReceptionNotFound // Используем стандартную ошибку "не найдено"
	}

	return nil // Успешное закрытие
}

// ListReceptionsByPVZIDs возвращает приемки для списка ПВЗ с фильтром по дате
func (r *ReceptionRepo) ListReceptionsByPVZIDs(ctx context.Context, pvzIDs []uuid.UUID, startDate, endDate *time.Time) ([]domain.Reception, error) {
	if len(pvzIDs) == 0 {
		return []domain.Reception{}, nil // Если нет ID ПВЗ, нечего искать
	}

	// Начинаем строить запрос
	queryBuilder := r.sq.
		Select("id", "pvz_id", "date_time", "status").
		From("receptions").
		Where(squirrel.Eq{"pvz_id": pvzIDs}). // Используем IN (...)
		OrderBy("pvz_id, date_time DESC")     // Группируем по ПВЗ, сортируем по дате

	// Добавляем фильтры по дате, если они есть
	if startDate != nil {
		queryBuilder = queryBuilder.Where(squirrel.GtOrEq{"date_time": *startDate}) // >= startDate
	}
	if endDate != nil {
		queryBuilder = queryBuilder.Where(squirrel.LtOrEq{"date_time": *endDate}) // <= endDate
	}

	sqlQuery, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения SQL для получения списка приемок: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения SQL для получения списка приемок: %w", err)
	}
	defer rows.Close()

	receptions := make([]domain.Reception, 0)
	for rows.Next() {
		var rcp domain.Reception
		if err := rows.Scan(&rcp.ID, &rcp.PVZID, &rcp.DateTime, &rcp.Status); err != nil {
			log.Printf("Ошибка сканирования строки приемки: %v", err)
			continue
		}
		receptions = append(receptions, rcp)
	}
	if err = rows.Err(); err != nil {
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
		OrderBy("reception_id, date_time_added ASC"). // Группируем, сортируем по добавлению
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка построения SQL для получения списка товаров: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения SQL для получения списка товаров: %w", err)
	}
	defer rows.Close()

	products := make([]domain.Product, 0)
	for rows.Next() {
		var p domain.Product
		// Важно: убедитесь, что сканируете в правильные поля и тип ProductType обрабатывается
		if err := rows.Scan(&p.ID, &p.ReceptionID, &p.DateTimeAdded, &p.Type); err != nil {
			log.Printf("Ошибка сканирования строки товара: %v", err)
			continue
		}
		products = append(products, p)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации по результатам товаров: %w", err)
	}

	return products, nil
}
