package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time" // Добавлен

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types" // Импорт нужен
)

// HandleInitiateReception - обработчик для POST /receptions
func (h *Handler) HandleInitiateReception(w http.ResponseWriter, r *http.Request) {
	if h.receptionService == nil {
		respondWithError(w, http.StatusInternalServerError, "Сервис приемок не инициализирован")
		return
	}

	var req InitiateReceptionRequest // Используем базовый тип запроса из openapi_types.gen.go
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректное тело запроса: "+err.Error())
		return
	}
	defer r.Body.Close()

	// Тип req.PvzId - openapi_types.UUID (псевдоним uuid.UUID)
	if req.PvzId == uuid.Nil { // Сравниваем с uuid.Nil
		respondWithError(w, http.StatusBadRequest, "Поле 'pvzId' является обязательным")
		return
	}

	// Передаем значение uuid.UUID в сервис
	receptionDomain, err := h.receptionService.InitiateReception(r.Context(), req.PvzId)
	if err != nil {
		if strings.Contains(err.Error(), "предыдущая приемка для этого ПВЗ еще не закрыта") {
			respondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Ошибка при инициации приемки: "+err.Error())
		}
		return
	}

	// Конвертация domain.Reception -> api.Reception
	// Проверяем типы полей в СГЕНЕРИРОВАННОМ api.Reception
	var apiIdPtr *openapi_types.UUID
	if receptionDomain.ID != uuid.Nil { // Генерируется всегда, значит можно взять адрес
		idWrapper := openapi_types.UUID(receptionDomain.ID) // Так как это псевдоним
		apiIdPtr = &idWrapper
	}

	var apiPvzIdPtr *openapi_types.UUID
	if receptionDomain.PVZID != uuid.Nil {
		pvzIdWrapper := openapi_types.UUID(receptionDomain.PVZID)
		apiPvzIdPtr = &pvzIdWrapper
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
		Id:       apiIdPtr,       // *openapi_types.UUID
		PvzId:    apiPvzIdPtr,    // *openapi_types.UUID
		DateTime: apiDateTimePtr, // *time.Time
		Status:   apiStatusPtr,   // *ReceptionStatus
	}

	respondWithJSON(w, http.StatusCreated, receptionAPI)
}

// HandleAddProduct - обработчик для POST /products
func (h *Handler) HandleAddProduct(w http.ResponseWriter, r *http.Request) {
	if h.receptionService == nil {
		respondWithError(w, http.StatusInternalServerError, "Сервис приемок не инициализирован")
		return
	}
	var req AddProductRequest // Используем базовый тип запроса из openapi_types.gen.go
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректное тело запроса: "+err.Error())
		return
	}
	defer r.Body.Close()

	// req.PvzId УЖЕ uuid.UUID
	if req.PvzId == uuid.Nil {
		respondWithError(w, http.StatusBadRequest, "Поле 'pvzId' является обязательным")
		return
	}
	// req.Type УЖЕ api.ProductType
	if req.Type == "" {
		respondWithError(w, http.StatusBadRequest, "Поле 'type' является обязательным")
		return
	}

	// Конвертируем api.ProductType -> domain.ProductType
	productTypeDomain := domain.ProductType(req.Type)
	if productTypeDomain != domain.TypeElectronics && productTypeDomain != domain.TypeClothes && productTypeDomain != domain.TypeShoes {
		respondWithError(w, http.StatusBadRequest, "Недопустимое значение для поля 'type'. Ожидается 'электроника', 'одежда' или 'обувь'.")
		return
	}

	// Передаем uuid.UUID и domain.ProductType в сервис
	productDomain, err := h.receptionService.AddProduct(r.Context(), req.PvzId, productTypeDomain)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "нет открытой приемки") || strings.Contains(errMsg, "недопустимый тип товара") {
			respondWithError(w, http.StatusBadRequest, errMsg)
		} else {
			respondWithError(w, http.StatusInternalServerError, "Ошибка при добавлении товара: "+errMsg)
		}
		return
	}

	// Конвертация domain.Product -> api.Product
	// Проверяем типы полей в СГЕНЕРИРОВАННОМ api.Product
	var apiIdPtr *openapi_types.UUID
	if productDomain.ID != uuid.Nil {
		idWrapper := openapi_types.UUID(productDomain.ID)
		apiIdPtr = &idWrapper
	}

	var apiReceptionIdPtr *openapi_types.UUID
	if productDomain.ReceptionID != uuid.Nil {
		receptionIdWrapper := openapi_types.UUID(productDomain.ReceptionID)
		apiReceptionIdPtr = &receptionIdWrapper
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
		Id:            apiIdPtr,            // *openapi_types.UUID
		ReceptionId:   apiReceptionIdPtr,   // *openapi_types.UUID
		DateTimeAdded: apiDateTimeAddedPtr, // *time.Time
		Type:          apiTypePtr,          // *ProductType
	}

	respondWithJSON(w, http.StatusCreated, productAPI)
}

// HandleDeleteLastProduct - обработчик для POST /pvz/{pvzId}/delete_last_product
func (h *Handler) HandleDeleteLastProduct(w http.ResponseWriter, r *http.Request) {
	if h.receptionService == nil {
		respondWithError(w, http.StatusInternalServerError, "Сервис приемок не инициализирован")
		return
	}

	pvzIdParam := chi.URLParam(r, "pvzId")
	if pvzIdParam == "" {
		respondWithError(w, http.StatusBadRequest, "Не указан ID ПВЗ в пути запроса")
		return
	}
	pvzID, err := uuid.Parse(pvzIdParam) // Парсим в uuid.UUID
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректный формат ID ПВЗ в пути: "+err.Error())
		return
	}

	err = h.receptionService.DeleteLastProduct(r.Context(), pvzID) // Передаем uuid.UUID
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "нет открытой приемки") ||
			strings.Contains(errMsg, "нет товаров для удаления") ||
			strings.Contains(errMsg, "не найден") {
			respondWithError(w, http.StatusBadRequest, errMsg)
		} else {
			respondWithError(w, http.StatusInternalServerError, "Ошибка при удалении товара: "+errMsg)
		}
		return
	}

	responseMessage := MessageResponse{Message: "Последний добавленный товар удален"} // Используем api.MessageResponse
	respondWithJSON(w, http.StatusOK, responseMessage)
}

// HandleCloseLastReception - обработчик для POST /pvz/{pvzId}/close_last_reception
func (h *Handler) HandleCloseLastReception(w http.ResponseWriter, r *http.Request) {
	if h.receptionService == nil {
		respondWithError(w, http.StatusInternalServerError, "Сервис приемок не инициализирован")
		return
	}

	pvzIdParam := chi.URLParam(r, "pvzId")
	if pvzIdParam == "" {
		respondWithError(w, http.StatusBadRequest, "Не указан ID ПВЗ в пути запроса")
		return
	}
	pvzID, err := uuid.Parse(pvzIdParam) // Парсим в uuid.UUID
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректный формат ID ПВЗ в пути: "+err.Error())
		return
	}

	closedReceptionDomain, err := h.receptionService.CloseLastReception(r.Context(), pvzID) // Передаем uuid.UUID
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "нет открытой приемки") ||
			strings.Contains(errMsg, "не найдена или уже закрыта") {
			respondWithError(w, http.StatusBadRequest, errMsg)
		} else {
			respondWithError(w, http.StatusInternalServerError, "Ошибка при закрытии приемки: "+errMsg)
		}
		return
	}

	// Конвертация domain.Reception -> api.Reception
	// Все поля в api.Reception - указатели
	var apiIdPtr *openapi_types.UUID
	if closedReceptionDomain.ID != uuid.Nil {
		idWrapper := openapi_types.UUID(closedReceptionDomain.ID)
		apiIdPtr = &idWrapper
	}

	var apiPvzIdPtr *openapi_types.UUID
	if closedReceptionDomain.PVZID != uuid.Nil {
		pvzIdWrapper := openapi_types.UUID(closedReceptionDomain.PVZID)
		apiPvzIdPtr = &pvzIdWrapper
	}

	var apiStatusPtr *ReceptionStatus
	if closedReceptionDomain.Status != "" {
		statusWrapper := ReceptionStatus(closedReceptionDomain.Status)
		apiStatusPtr = &statusWrapper
	}

	var apiDateTimePtr *time.Time
	if !closedReceptionDomain.DateTime.IsZero() {
		apiDateTimePtr = &closedReceptionDomain.DateTime
	}

	closedReceptionAPI := Reception{
		Id:       apiIdPtr,       // *openapi_types.UUID
		PvzId:    apiPvzIdPtr,    // *openapi_types.UUID
		DateTime: apiDateTimePtr, // *time.Time
		Status:   apiStatusPtr,   // *ReceptionStatus
	}

	respondWithJSON(w, http.StatusOK, closedReceptionAPI)
}
