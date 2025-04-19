package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// HandleInitiateReception - обработчик для POST /receptions
func (h *Handler) HandleInitiateReception(w http.ResponseWriter, r *http.Request) {
	if h.receptionService == nil {
		respondWithError(w, http.StatusInternalServerError, "Сервис приемок не инициализирован")
		return
	}

	var req PostReceptionsJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректное тело запроса: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.PvzId == uuid.Nil {
		respondWithError(w, http.StatusBadRequest, "Поле 'pvzId' является обязательным")
		return
	}

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
	receptionAPI := Reception{
		Id:       &receptionDomain.ID,                             // Указатель *openapi_types.UUID
		PvzId:    receptionDomain.PVZID,                           // НЕ указатель openapi_types.UUID
		DateTime: &receptionDomain.DateTime,                       // НЕ указатель time.Time
		Status:   ReceptionStatus(string(receptionDomain.Status)), // НЕ указатель ReceptionStatus
	}

	respondWithJSON(w, http.StatusCreated, receptionAPI)
}

// HandleAddProduct - обработчик для POST /products
func (h *Handler) HandleAddProduct(w http.ResponseWriter, r *http.Request) {
	if h.receptionService == nil {
		respondWithError(w, http.StatusInternalServerError, "Сервис приемок не инициализирован")
		return
	}

	var req PostProductsJSONRequestBody
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
	productAPI := Product{
		Id:            &productDomain.ID,                       // Указатель *openapi_types.UUID
		ReceptionId:   productDomain.ReceptionID,               // НЕ указатель openapi_types.UUID
		DateTimeAdded: &productDomain.DateTimeAdded,            // Указатель *time.Time (Используем правильное имя поля!)
		Type:          ProductType(string(productDomain.Type)), // НЕ указатель ProductType
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
	pvzID, err := uuid.Parse(pvzIdParam)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректный формат ID ПВЗ в пути: "+err.Error())
		return
	}

	err = h.receptionService.DeleteLastProduct(r.Context(), pvzID)
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

	// Используем сгенерированный Error для ответа
	respondWithJSON(w, http.StatusOK, Error{Message: "Последний добавленный товар удален"})
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
	pvzID, err := uuid.Parse(pvzIdParam)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректный формат ID ПВЗ в пути: "+err.Error())
		return
	}

	closedReceptionDomain, err := h.receptionService.CloseLastReception(r.Context(), pvzID)
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
	closedReceptionAPI := Reception{
		Id:       &closedReceptionDomain.ID,                             // Указатель *openapi_types.UUID
		PvzId:    closedReceptionDomain.PVZID,                           // НЕ указатель openapi_types.UUID
		DateTime: &closedReceptionDomain.DateTime,                       // НЕ указатель time.Time
		Status:   ReceptionStatus(string(closedReceptionDomain.Status)), // НЕ указатель ReceptionStatus
	}

	respondWithJSON(w, http.StatusOK, closedReceptionAPI)
}

// --- Определения старых DTO удалены ---
