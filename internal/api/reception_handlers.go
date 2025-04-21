package api

import (
	"encoding/json"
	"errors" // Добавляем для errors.Is

	// Убираем, если fmt.Sprintf не используется в respondWithError
	"log/slog"
	"net/http"

	// "strings" // Больше не нужен для проверки ошибок
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository" // Нужен для ошибок репозитория
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// HandleInitiateReception - обработчик для POST /receptions
func (h *Handler) HandleInitiateReception(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // Получаем контекст из запроса для логирования
	if h.receptionService == nil {
		respondWithError(w, http.StatusInternalServerError, "Сервис приемок не инициализирован")
		return
	}

	var req InitiateReceptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректное тело запроса: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.PvzId == uuid.Nil {
		respondWithError(w, http.StatusBadRequest, "Поле 'pvzId' является обязательным")
		return
	}

	receptionDomain, err := h.receptionService.InitiateReception(ctx, req.PvzId)
	if err != nil {
		// --- ИСПРАВЛЕНО: Используем errors.Is ---
		// Проверяем на конкретную ошибку "уже открыта" (предполагаем, что сервис ее возвращает или оборачивает)
		// Вам может понадобиться определить свою ошибку в пакете service или domain, например domain.ErrReceptionAlreadyOpen
		// Здесь для примера проверяем исходное сообщение, но лучше проверять тип/значение ошибки.
		// if errors.Is(err, service.ErrReceptionAlreadyOpen) { // Пример с кастомной ошибкой сервиса
		if err.Error() == "предыдущая приемка для этого ПВЗ еще не закрыта" { // Оставляем проверку строки, если кастомной ошибки нет
			respondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			// Логируем ошибку сервера
			slog.ErrorContext(ctx, "Ошибка сервиса при инициации приемки", slog.Any("error", err), slog.Any("pvz_id", req.PvzId))
			respondWithError(w, http.StatusInternalServerError, "Внутренняя ошибка сервера при инициации приемки") // Не показываем детали внутренней ошибки клиенту
		}
		return
	}

	// Конвертация domain.Reception -> api.Reception
	var apiIdPtr *openapi_types.UUID
	if receptionDomain.ID != uuid.Nil {
		apiIdPtr = &receptionDomain.ID
	}
	var apiPvzIdPtr *openapi_types.UUID
	if receptionDomain.PVZID != uuid.Nil {
		apiPvzIdPtr = &receptionDomain.PVZID
	}
	var apiStatusPtr *ReceptionStatus
	if receptionDomain.Status != "" {
		statusWrapper := ReceptionStatus(receptionDomain.Status)
		apiStatusPtr = &statusWrapper
	}
	var apiDateTimePtr *time.Time
	if !receptionDomain.DateTime.IsZero() {
		apiDateTimePtr = &receptionDomain.DateTime
	}

	receptionAPI := Reception{
		Id:       apiIdPtr,
		PvzId:    apiPvzIdPtr,
		DateTime: apiDateTimePtr,
		Status:   apiStatusPtr,
	}

	// Используем respondWithJSON с потоковым кодированием
	respondWithJSON(w, http.StatusCreated, receptionAPI)
}

// HandleAddProduct - обработчик для POST /products
func (h *Handler) HandleAddProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.receptionService == nil {
		respondWithError(w, http.StatusInternalServerError, "Сервис приемок не инициализирован")
		return
	}
	var req AddProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректное тело запроса: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.PvzId == uuid.Nil {
		respondWithError(w, http.StatusBadRequest, "Поле 'pvzId' является обязательным")
		return
	}
	if req.Type == "" {
		respondWithError(w, http.StatusBadRequest, "Поле 'type' является обязательным")
		return
	}

	productTypeDomain := domain.ProductType(req.Type)
	if productTypeDomain != domain.TypeElectronics && productTypeDomain != domain.TypeClothes && productTypeDomain != domain.TypeShoes {
		respondWithError(w, http.StatusBadRequest, "Недопустимое значение для поля 'type'. Ожидается 'электроника', 'одежда' или 'обувь'.")
		return
	}

	productDomain, err := h.receptionService.AddProduct(ctx, req.PvzId, productTypeDomain)
	if err != nil {
		// --- ИСПРАВЛЕНО: Используем errors.Is ---
		// Проверяем на известные ошибки репозитория/сервиса
		if errors.Is(err, repository.ErrReceptionNotFound) { // Предполагаем, что сервис пробрасывает эту ошибку
			respondWithError(w, http.StatusBadRequest, "нет открытой приемки для данного ПВЗ, чтобы добавить товар")
		} else if err.Error() == "недопустимый тип товара" { // Если валидация типа происходит и в сервисе
			respondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			slog.ErrorContext(ctx, "Ошибка сервиса при добавлении товара", slog.Any("error", err), slog.Any("pvz_id", req.PvzId), slog.String("type", string(req.Type)))
			respondWithError(w, http.StatusInternalServerError, "Внутренняя ошибка сервера при добавлении товара")
		}
		return
	}

	// Конвертация domain.Product -> api.Product
	var apiIdPtr *openapi_types.UUID
	if productDomain.ID != uuid.Nil {
		apiIdPtr = &productDomain.ID
	}
	var apiReceptionIdPtr *openapi_types.UUID
	if productDomain.ReceptionID != uuid.Nil {
		apiReceptionIdPtr = &productDomain.ReceptionID
	}
	var apiTypePtr *ProductType
	if productDomain.Type != "" {
		typeWrapper := ProductType(productDomain.Type)
		apiTypePtr = &typeWrapper
	}
	var apiDateTimeAddedPtr *time.Time
	if !productDomain.DateTimeAdded.IsZero() {
		apiDateTimeAddedPtr = &productDomain.DateTimeAdded
	}

	productAPI := Product{
		Id:            apiIdPtr,
		ReceptionId:   apiReceptionIdPtr,
		DateTimeAdded: apiDateTimeAddedPtr,
		Type:          apiTypePtr,
	}

	respondWithJSON(w, http.StatusCreated, productAPI)
}

// HandleDeleteLastProduct - обработчик для POST /pvz/{pvzId}/delete_last_product
func (h *Handler) HandleDeleteLastProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.receptionService == nil {
		respondWithError(w, http.StatusInternalServerError, "Сервис приемок не инициализирован")
		return
	}

	pvzIdParam := chi.URLParam(r, "pvzId")
	if pvzIdParam == "" {
		respondWithError(w, http.StatusBadRequest, "Не указан ID ПВЗ в пути запроса")
		return
	}
	pvzID, err := uuid.Parse(pvzIdParam)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректный формат ID ПВЗ в пути: "+err.Error())
		return
	}

	err = h.receptionService.DeleteLastProduct(ctx, pvzID)
	if err != nil {
		// --- ИСПРАВЛЕНО: Используем errors.Is ---
		if errors.Is(err, repository.ErrReceptionNotFound) {
			respondWithError(w, http.StatusBadRequest, "нет открытой приемки для данного ПВЗ, чтобы удалить товар")
		} else if errors.Is(err, repository.ErrProductNotFound) {
			// Эта ошибка может приходить от GetLastProductFromReception или DeleteProductByID
			respondWithError(w, http.StatusBadRequest, "в текущей открытой приемке нет товаров для удаления или товар уже удален")
		} else {
			slog.ErrorContext(ctx, "Ошибка сервиса при удалении товара", slog.Any("error", err), slog.Any("pvz_id", pvzID))
			respondWithError(w, http.StatusInternalServerError, "Внутренняя ошибка сервера при удалении товара")
		}
		return
	}

	responseMessage := MessageResponse{Message: "Последний добавленный товар удален"}
	respondWithJSON(w, http.StatusOK, responseMessage)
}

// HandleCloseLastReception - обработчик для POST /pvz/{pvzId}/close_last_reception
func (h *Handler) HandleCloseLastReception(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.receptionService == nil {
		respondWithError(w, http.StatusInternalServerError, "Сервис приемок не инициализирован")
		return
	}

	pvzIdParam := chi.URLParam(r, "pvzId")
	if pvzIdParam == "" {
		respondWithError(w, http.StatusBadRequest, "Не указан ID ПВЗ в пути запроса")
		return
	}
	pvzID, err := uuid.Parse(pvzIdParam)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректный формат ID ПВЗ в пути: "+err.Error())
		return
	}

	closedReceptionDomain, err := h.receptionService.CloseLastReception(ctx, pvzID)
	if err != nil {
		// --- ИСПРАВЛЕНО: Используем errors.Is ---
		if errors.Is(err, repository.ErrReceptionNotFound) {
			// Эта ошибка может приходить от GetLastOpenReceptionByPVZ или CloseReceptionByID
			respondWithError(w, http.StatusBadRequest, "не удалось закрыть приемку, так как она не найдена или уже закрыта")
		} else {
			slog.ErrorContext(ctx, "Ошибка сервиса при закрытии приемки", slog.Any("error", err), slog.Any("pvz_id", pvzID))
			respondWithError(w, http.StatusInternalServerError, "Внутренняя ошибка сервера при закрытии приемки")
		}
		return
	}

	// Конвертация domain.Reception -> api.Reception
	var apiIdPtr *openapi_types.UUID
	if closedReceptionDomain.ID != uuid.Nil {
		apiIdPtr = &closedReceptionDomain.ID
	}
	var apiPvzIdPtr *openapi_types.UUID
	if closedReceptionDomain.PVZID != uuid.Nil {
		apiPvzIdPtr = &closedReceptionDomain.PVZID
	}
	var apiStatusPtr *ReceptionStatus
	if closedReceptionDomain.Status != "" {
		tempStatus := ReceptionStatus(closedReceptionDomain.Status)
		apiStatusPtr = &tempStatus
	}
	var apiDateTimePtr *time.Time
	if !closedReceptionDomain.DateTime.IsZero() {
		apiDateTimePtr = &closedReceptionDomain.DateTime
	}

	closedReceptionAPI := Reception{
		Id:       apiIdPtr,
		PvzId:    apiPvzIdPtr,
		DateTime: apiDateTimePtr,
		Status:   apiStatusPtr,
	}

	respondWithJSON(w, http.StatusOK, closedReceptionAPI)
}

// --- Убедитесь, что функция respondWithJSON использует json.NewEncoder ---
// (Эта функция, вероятно, находится в handler.go или аналогичном файле)
/*
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    err := json.NewEncoder(w).Encode(payload)
    if err != nil {
        slog.Error("Ошибка записи JSON ответа", slog.Any("error", err))
    }
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	// Логируем ошибку с использованием slog
	slog.WarnContext(r.Context(), "Ошибка ответа API", slog.Int("status_code", code), slog.String("message", message)) // Нужен r *http.Request или ctx context.Context

	errorResponse := Error{ // Используем сгенерированный тип api.Error
		Message: message,
	}
	respondWithJSON(w, code, errorResponse)
}
*/
