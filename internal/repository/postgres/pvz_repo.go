package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt" // Для форматирования ошибок
	"log" // Для логирования

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

// GetAllPVZs implements repository.PVZRepository.
func (r *PVZRepo) GetAllPVZs(ctx context.Context) ([]domain.PVZ, error) {
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
	// Генерируем новый уникальный ID для записи
	pvz.ID = uuid.New()

	// Строим SQL запрос INSERT с помощью squirrel
	sqlQuery, args, err := r.sq.
		Insert("pvz").            // Указываем таблицу
		Columns("id", "city").    // Перечисляем колонки для вставки (registration_date имеет DEFAULT NOW())
		Values(pvz.ID, pvz.City). // Указываем значения
		ToSql()                   // Генерируем SQL строку и аргументы
	if err != nil {
		// Ошибка на этапе построения запроса - маловероятна для такого простого запроса
		return uuid.Nil, fmt.Errorf("ошибка построения SQL запроса для создания ПВЗ: %w", err)
	}

	// Выполняем SQL запрос к базе данных
	_, err = r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		// Здесь могут быть различные ошибки БД: нарушение ограничений (CHECK на city),
		// проблемы с соединением, нарушение уникальности (если бы оно было на city) и т.д.
		// TODO: Рассмотреть возможность обработки специфических ошибок PostgreSQL (например, по коду ошибки)
		return uuid.Nil, fmt.Errorf("ошибка выполнения SQL запроса для создания ПВЗ: %w", err)
	}

	// Возвращаем ID успешно созданного ПВЗ
	return pvz.ID, nil
}

// ListPVZs - получает список ПВЗ из базы данных с пагинацией.
// Возвращает срез domain.PVZ для текущей страницы,
// общее количество ПВЗ в базе данных (для пагинации на клиенте/в сервисе)
// и ошибку, если что-то пошло не так.
// internal/repository/postgres/pvz_repo.go

func (r *PVZRepo) ListPVZs(ctx context.Context, page, limit int) ([]domain.PVZ, int, error) {
	// --- 1. Получаем общее количество ---
	var totalCount int
	countQuery, _, err := r.sq.Select("COUNT(*)").From("pvz").ToSql()
	if err != nil {
		log.Printf("!!! ОШИБКА построения countQuery: %v", err) // Логируем ошибку
		return nil, 0, fmt.Errorf("ошибка построения SQL для подсчета ПВЗ: %w", err)
	}
	log.Printf(">>> Выполняется countQuery: %s", countQuery) // Логируем сам SQL

	// Выполняем запрос подсчета
	row := r.db.QueryRowContext(ctx, countQuery) // Получаем *sql.Row
	err = row.Scan(&totalCount)                  // Сканируем результат в totalCount
	if err != nil {
		// Логируем ошибку сканирования
		log.Printf("!!! ОШИБКА выполнения/сканирования countQuery: %v", err)
		// Проверяем, может быть, это sql.ErrNoRows (хотя для COUNT(*) это невозможно)
		if errors.Is(err, sql.ErrNoRows) {
			log.Println("Получена ошибка sql.ErrNoRows при подсчете ПВЗ (очень странно!)")
			// В этом случае считаем, что count = 0
			totalCount = 0
			// НЕ возвращаем ошибку, просто count будет 0
		} else {
			// Другая ошибка - возвращаем ее
			return nil, 0, fmt.Errorf("ошибка выполнения SQL для подсчета ПВЗ: %w", err)
		}
	}
	log.Printf(">>> Подсчитано ПВЗ (totalCount после Scan): %d", totalCount) // Логируем результат

	// --- Обработка случая, когда ПВЗ нет ---
	if totalCount == 0 {
		log.Println("ПВЗ не найдены, возвращаем пустой результат.")
		return []domain.PVZ{}, 0, nil
	}

	// --- 2. Получаем нужную страницу ---
	// (остальной код получения страницы без изменений)
	offset := (page - 1) * limit
	selectQuery, args, err := r.sq.
		Select("id", "registration_date", "city").
		From("pvz").
		OrderBy("registration_date DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка построения SQL для получения списка ПВЗ: %w", err)
	}
	// log.Printf(">>> Выполняется selectQuery: %s, Args: %v", selectQuery, args) // Можно раскомментировать для отладки

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		log.Printf("!!! ОШИБКА выполнения selectQuery: %v", err) // Логируем ошибку
		return nil, 0, fmt.Errorf("ошибка выполнения SQL для получения списка ПВЗ: %w", err)
	}
	defer rows.Close()

	pvzList := make([]domain.PVZ, 0, limit)
	scanErrors := 0 // Счетчик ошибок сканирования строк
	for rows.Next() {
		var pvz domain.PVZ
		if err := rows.Scan(&pvz.ID, &pvz.RegistrationDate, &pvz.City); err != nil {
			log.Printf("!!! ОШИБКА сканирования строки ПВЗ: %v", err)
			scanErrors++
			continue // Пытаемся продолжить со следующей строкой
		}
		pvzList = append(pvzList, pvz)
	}
	if err = rows.Err(); err != nil { // Ошибка во время итерации
		return nil, 0, fmt.Errorf("ошибка итерации по результатам ПВЗ: %w", err)
	}
	if scanErrors > 0 {
		log.Printf("!!! Было %d ошибок при сканировании строк ПВЗ", scanErrors)
		// Решаем, возвращать ли ошибку, если были проблемы со сканированием части строк
		// return nil, 0, fmt.Errorf("произошли ошибки при чтении данных ПВЗ")
	}

	// --- Возвращаем результат ---
	log.Printf(">>> Возвращаем список ПВЗ (count: %d) и totalCount: %d", len(pvzList), totalCount)
	return pvzList, totalCount, nil
}
