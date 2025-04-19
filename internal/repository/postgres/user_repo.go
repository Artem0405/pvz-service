package postgres

import (
	"context"
	"database/sql"
	"errors" // Для errors.Is
	"fmt"
	"strings" // Для проверки ошибки unique_violation

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn" // Для проверки кода ошибки PostgreSQL

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository" // Для кастомных ошибок
)

type UserRepo struct {
	db *sql.DB
	sq squirrel.StatementBuilderType
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{
		db: db,
		sq: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// CreateUser сохраняет нового пользователя
func (r *UserRepo) CreateUser(ctx context.Context, user domain.User) (uuid.UUID, error) {
	if user.ID == uuid.Nil { // Генерируем ID, если не предоставлен
		user.ID = uuid.New()
	}

	sqlQuery, args, err := r.sq.
		Insert("users").
		Columns("id", "email", "password_hash", "role").
		Values(user.ID, user.Email, user.PasswordHash, user.Role).
		ToSql()
	if err != nil {
		return uuid.Nil, fmt.Errorf("ошибка построения SQL для создания пользователя: %w", err)
	}

	_, err = r.db.ExecContext(ctx, sqlQuery, args...)
	if err != nil {
		// Проверяем ошибку уникальности email (специфично для PostgreSQL)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // 23505 = unique_violation
			// Можно дополнительно проверить имя констрейнта, если он известен
			if strings.Contains(pgErr.ConstraintName, "users_email_key") { // Имя констрейнта может отличаться
				return uuid.Nil, repository.ErrUserDuplicateEmail
			}
		}
		return uuid.Nil, fmt.Errorf("ошибка выполнения SQL для создания пользователя: %w", err)
	}

	return user.ID, nil
}

// GetUserByEmail ищет пользователя по email
func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var user domain.User
	sqlQuery, args, err := r.sq.
		Select("id", "email", "password_hash", "role").
		From("users").
		Where(squirrel.Eq{"email": email}).
		Limit(1).
		ToSql()
	if err != nil {
		return user, fmt.Errorf("ошибка построения SQL для поиска пользователя по email: %w", err)
	}

	row := r.db.QueryRowContext(ctx, sqlQuery, args...)
	err = row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user, repository.ErrUserNotFound // Используем кастомную ошибку
		}
		return user, fmt.Errorf("ошибка сканирования данных пользователя по email: %w", err)
	}

	return user, nil
}
