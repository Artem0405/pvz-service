package api

import (
	"context"
	"fmt"
	"log/slog" // Импортируем slog
	"net/http"
	"strconv"
	"strings"
	"time" // Для расчета длительности запроса

	// Импорт chi middleware нужен для обертки ответа
	"github.com/go-chi/chi/v5/middleware"

	mmetrics "github.com/Artem0405/pvz-service/internal/metrics"
	"github.com/Artem0405/pvz-service/internal/service"
)

// contextKey - кастомный тип для ключа в контексте запроса.
// Использование кастомного типа вместо строки предотвращает случайные коллизии
// с другими ключами, которые могут быть добавлены в контекст другими пакетами или middleware.
type contextKey string

// roleContextKey - конкретный ключ, который мы будем использовать
// для хранения извлеченной роли пользователя в контексте запроса.
const roleContextKey = contextKey("userRole") // Используем "userRole" для ясности

// AuthMiddleware - это функция высшего порядка (фабрика middleware).
// Она принимает зависимость - сервис аутентификации (как интерфейс AuthService),
// и возвращает саму middleware - функцию, которая оборачивает следующий http.Handler.
func AuthMiddleware(authService service.AuthService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			slog.Debug("AuthMiddleware: Received Authorization header", "header", authHeader) // <-- ЛОГ 1

			if authHeader == "" {
				slog.Warn("AuthMiddleware: Authorization header missing") // <-- ЛОГ 2
				respondWithError(w, http.StatusUnauthorized, "Отсутствует заголовок Authorization")
				return
			}

			headerParts := strings.Split(authHeader, " ")
			if len(headerParts) != 2 || strings.ToLower(headerParts[0]) != "bearer" {
				slog.Warn("AuthMiddleware: Invalid Authorization header format", "header", authHeader) // <-- ЛОГ 3
				respondWithError(w, http.StatusUnauthorized, "Неверный формат заголовка Authorization (ожидается 'Bearer <token>')")
				return
			}

			tokenString := headerParts[1]
			slog.Debug("AuthMiddleware: Extracted token string", "token_prefix", tokenString[:min(10, len(tokenString))]) // <-- ЛОГ 4 (часть токена)

			claims, err := authService.ValidateToken(tokenString)
			if err != nil {
				slog.Warn("AuthMiddleware: Token validation failed", "error", err.Error()) // <-- ЛОГ 5
				respondWithError(w, http.StatusUnauthorized, "Невалидный или просроченный токен: "+err.Error())
				return
			}

			slog.Debug("AuthMiddleware: Token validated successfully", "role", claims.Role) // <-- ЛОГ 6

			// ... (сохранение роли в контекст и вызов next) ...
			ctx := context.WithValue(r.Context(), roleContextKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetRoleFromContext - вспомогательная функция для безопасного извлечения
// роли пользователя из контекста запроса. Используется в RoleMiddleware
// и может быть использована напрямую в хендлерах при необходимости.
// Возвращает роль (string) и флаг (bool), указывающий, была ли роль найдена в контексте.
func GetRoleFromContext(ctx context.Context) (string, bool) {
	// Пытаемся получить значение по нашему ключу roleContextKey.
	// Выполняем утверждение типа (type assertion) к string.
	role, ok := ctx.Value(roleContextKey).(string)
	// Возвращаем полученное значение и результат утверждения типа.
	return role, ok
}

// RoleMiddleware - фабрика middleware для проверки наличия у пользователя
// требуемой роли. Принимает строку с необходимой ролью.
func RoleMiddleware(requiredRole string) func(next http.Handler) http.Handler {
	// Возвращаем функцию-middleware.
	return func(next http.Handler) http.Handler {
		// Возвращаем HandlerFunc, который выполняет проверку.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Извлекаем роль пользователя из контекста, используя нашу вспомогательную функцию.
			// Предполагается, что AuthMiddleware уже отработала и поместила роль в контекст.
			role, ok := GetRoleFromContext(r.Context())
			if !ok {
				// Если роль не найдена в контексте - это внутренняя ошибка сервера,
				// так как AuthMiddleware должна была ее добавить или прервать запрос раньше.
				respondWithError(w, http.StatusInternalServerError, "Не удалось получить роль пользователя из контекста")
				return
			}

			// Сравниваем роль пользователя с требуемой ролью.
			if role != requiredRole {
				// Если роли не совпадают, отправляем ошибку 403 Forbidden (Доступ запрещен).
				respondWithError(w, http.StatusForbidden, fmt.Sprintf("Доступ запрещен. Требуется роль: '%s', у вас роль: '%s'", requiredRole, role))
				return
			}

			// Если роль совпадает, передаем управление следующему обработчику.
			next.ServeHTTP(w, r)
		})
	}
}

// SlogMiddleware - middleware для структурированного логирования запросов с помощью slog.
func SlogMiddleware(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем атрибуты запроса для логирования
			requestID := middleware.GetReqID(r.Context()) // Получаем Request ID от chi
			method := r.Method
			path := r.URL.Path
			remoteAddr := r.RemoteAddr
			userAgent := r.UserAgent()

			// Логируем начало обработки запроса
			logger.Info("request started",
				slog.String("request_id", requestID),
				slog.String("method", method),
				slog.String("path", path),
				slog.String("remote_addr", remoteAddr),
				slog.String("user_agent", userAgent),
			)

			// Используем обертку для ResponseWriter, чтобы получить статус и размер ответа
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			startTime := time.Now() // Засекаем время начала

			// Передаем управление следующему обработчику
			next.ServeHTTP(ww, r)

			duration := time.Since(startTime) // Рассчитываем длительность
			statusCode := ww.Status()         // Получаем статус ответа
			bytesWritten := ww.BytesWritten() // Получаем размер ответа

			// Логируем завершение обработки запроса
			logger.Info("request completed",
				slog.String("request_id", requestID),
				slog.String("method", method), // Повторяем для удобства фильтрации
				slog.String("path", path),     // Повторяем
				slog.Int("status_code", statusCode),
				slog.Int("bytes_written", bytesWritten),
				slog.Duration("duration", duration),
			)
		})
	}
}

// PrometheusMiddleware собирает метрики HTTP запросов
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start)
		statusCode := ww.Status()
		path := r.URL.Path

		// --- ИСПОЛЬЗУЕМ ИМЕНА С ПАКЕТОМ ---
		mmetrics.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration.Seconds())
		mmetrics.HTTPRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(statusCode)).Inc()
		// --------------------------
	})
}
