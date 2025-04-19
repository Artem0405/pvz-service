package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log" // Оставляем для критических ошибок json.Marshal
	"log/slog"
	"net/http"
	"time"

	"github.com/Artem0405/pvz-service/internal/service"
)

// !!! --- Удалите определение этой структуры отсюда --- !!!
/*
type ErrorResponse struct {
	Message string `json:"message"`
}
*/
// !!! --------------------------------------------------- !!!

// Handler - основная структура для наших HTTP обработчиков.
// Содержит все зависимости.
type Handler struct {
	db               *sql.DB
	authService      service.AuthService
	pvzService       service.PVZService
	receptionService service.ReceptionService
}

// NewHandler - конструктор для Handler.
func NewHandler(db *sql.DB, authService service.AuthService, pvzService service.PVZService, receptionService service.ReceptionService) *Handler {
	return &Handler{
		db:               db,
		authService:      authService,
		pvzService:       pvzService,
		receptionService: receptionService,
	}
}

// respondWithError - вспомогательная функция для отправки стандартизированного
// JSON-ответа об ошибке клиенту. Логирует ошибку на сервере.
// Использует сгенерированный тип ошибки.
func respondWithError(w http.ResponseWriter, code int, message string) {
	// Логируем ошибку с использованием slog
	slog.Warn("Ошибка ответа API", slog.Int("status_code", code), slog.String("message", message))

	// --- Используем сгенерированный тип ошибки ---
	// Предполагаем, что он называется Error и поле Message *string
	errorResponse := Error{ // <--- Замените Error на ваше имя типа
		Message: message, // <--- Берем адрес строки, если поле - указатель
	}
	// --- Конец использования сгенерированного типа ---

	respondWithJSON(w, code, errorResponse)
}

// respondWithJSON - вспомогательная функция для кодирования payload в JSON
// и отправки ответа клиенту (без изменений).
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		// Используем стандартный log для критической ошибки, которую не можем вернуть клиенту
		log.Printf("Критическая ошибка кодирования JSON ответа: %v", err)
		// Отправляем простой текстовый JSON, т.к. не можем сформировать стандартный ErrorResponse
		http.Error(w, `{"message": "Внутренняя ошибка сервера при кодировании ответа"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err = w.Write(response) // Используем пустой идентификатор для количества записанных байт
	if err != nil {
		// Логируем, если не удалось записать ответ после установки заголовков
		slog.Error("Ошибка записи HTTP ответа", "error", err)
	}
}

// HandleHealthCheck - обработчик для эндпоинта GET /health (без изменений).
func (h *Handler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	dbErr := h.db.PingContext(ctx)
	if dbErr != nil {
		// Используем нашу обновленную respondWithError
		respondWithError(w, http.StatusServiceUnavailable, "База данных недоступна: "+dbErr.Error())
		return
	}

	slog.Info("Health check successful (DB ping ok)") // Используем slog
	// Используем нашу respondWithJSON
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "up"})
}

// --- Остальные обработчики находятся в других файлах пакета api ---
