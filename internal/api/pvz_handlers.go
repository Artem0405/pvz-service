package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	// Нужен для openapi_types.UUID, если используется в полях DTO
	// Нужен для uuid.Nil и конвертации
)

// HandleCreatePVZ - обработчик для POST /pvz
func (h *Handler) HandleCreatePVZ(w http.ResponseWriter, r *http.Request) {
	// Используем сгенерированный тип запроса = PVZ
	var req PVZ // Используем сгенерированный тип PVZ
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректное тело запроса: "+err.Error())
		return
	}
	defer r.Body.Close()

	// Валидация: City обязателен (поле не указатель, проверяем на пустую строку)
	if req.City == "" {
		respondWithError(w, http.StatusBadRequest, "Поле 'city' является обязательным")
		return
	}

	// Передаем в сервис доменную модель
	pvzInput := domain.PVZ{
		City: string(req.City), // Приводим PVZCity к string
	}
	createdPVZDomain, err := h.pvzService.CreatePVZ(r.Context(), pvzInput)
	if err != nil {
		errMsg := err.Error()
		// Обработка ошибок валидации города
		if errMsg == "создание ПВЗ возможно только в городах: Москва, Санкт-Петербург, Казань" {
			respondWithError(w, http.StatusBadRequest, errMsg)
		} else {
			respondWithError(w, http.StatusInternalServerError, "Ошибка при создании ПВЗ: "+errMsg)
		}
		return
	}

	// Конвертируем доменную модель в API DTO для ответа
	// Используем сгенерированный тип ответа PVZ
	responsePVZ := PVZ{
		Id:               &createdPVZDomain.ID,               // Id в DTO - *openapi_types.UUID, берем адрес
		City:             PVZCity(createdPVZDomain.City),     // City в DTO - PVZCity (не указатель), приводим string к PVZCity
		RegistrationDate: &createdPVZDomain.RegistrationDate, // RegistrationDate в DTO - *time.Time, берем адрес
	}

	respondWithJSON(w, http.StatusCreated, responsePVZ)
}

// HandleListPVZ - обработчик для GET /pvz
func (h *Handler) HandleListPVZ(w http.ResponseWriter, r *http.Request) {
	// --- 1. Парсинг параметров ---
	// Используем локальные переменные для парсинга, так как GetPvzParams - для параметров, а не для хранения распарсенных значений
	q := r.URL.Query()
	pageStr := q.Get("page")
	limitStr := q.Get("limit")
	page := 1   // Значение по умолчанию
	limit := 10 // Значение по умолчанию

	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err == nil && p >= 1 {
			page = p
		} else {
			respondWithError(w, http.StatusBadRequest, "Некорректное значение для параметра 'page'")
			return
		}
	}
	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err == nil && l >= 1 && l <= 30 { // Применяем ваши ограничения
			limit = l
		} else {
			respondWithError(w, http.StatusBadRequest, "Некорректное значение для параметра 'limit' (1-30)")
			return
		}
	}

	var startDatePtr, endDatePtr *time.Time
	if sdStr := q.Get("startDate"); sdStr != "" {
		t, err := time.Parse(time.RFC3339, sdStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Некорректный формат startDate (ожидается RFC3339)")
			return
		}
		startDatePtr = &t
	}
	if edStr := q.Get("endDate"); edStr != "" {
		t, err := time.Parse(time.RFC3339, edStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Некорректный формат endDate (ожидается RFC3339)")
			return
		}
		endDatePtr = &t
	}

	// --- 2. Вызов сервиса (получаем domain.GetPVZListResult) ---
	serviceResult, err := h.pvzService.GetPVZList(r.Context(), startDatePtr, endDatePtr, page, limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Ошибка получения списка ПВЗ: "+err.Error())
		return
	}

	// --- 3. Сборка ответа API с использованием СГЕНЕРИРОВАННЫХ типов ---

	// Используем сгенерированные типы из openapi_types.gen.go
	apiItems := make([]PvzListItem, 0, len(serviceResult.PVZs)) // Используем сгенерированный PvzListItem

	for _, pvzDomain := range serviceResult.PVZs {
		domainReceptions := serviceResult.Receptions[pvzDomain.ID]
		apiReceptions := make([]ReceptionInfo, 0, len(domainReceptions)) // Используем сгенерированный ReceptionInfo

		for _, rcpDomain := range domainReceptions {
			domainProducts := serviceResult.Products[rcpDomain.ID]
			apiProducts := make([]ProductInfo, 0, len(domainProducts)) // Используем сгенерированный ProductInfo (псевдоним Product)

			for _, pDomain := range domainProducts {
				// Конвертируем domain.Product в api.ProductInfo (он же api.Product)
				apiProduct := ProductInfo{ // Используем ProductInfo (псевдоним Product)
					Id:            &pDomain.ID,                       // Указатель *openapi_types.UUID
					ReceptionId:   pDomain.ReceptionID,               // Не указатель openapi_types.UUID
					DateTimeAdded: &pDomain.DateTimeAdded,            // Указатель *time.Time
					Type:          ProductType(string(pDomain.Type)), // Не указатель ProductType
				}
				apiProducts = append(apiProducts, apiProduct)
			}

			// Конвертируем domain.Reception в api.Reception
			apiReceptionBase := Reception{
				Id:       &rcpDomain.ID,                             // Указатель *openapi_types.UUID
				PvzId:    rcpDomain.PVZID,                           // Не указатель openapi_types.UUID
				DateTime: &rcpDomain.DateTime,                       // Указатель *time.Time
				Status:   ReceptionStatus(string(rcpDomain.Status)), // Не указатель ReceptionStatus
			}
			// Создаем api.ReceptionInfo
			apiReceptionItem := ReceptionInfo{
				Reception: apiReceptionBase, // Вставляем базовую приемку
				Products:  apiProducts,      // Вставляем массив товаров
			}
			apiReceptions = append(apiReceptions, apiReceptionItem)
		}

		// Конвертируем domain.PVZ в api.PVZ
		apiPvzBase := PVZ{
			Id:               &pvzDomain.ID,
			City:             PVZCity(pvzDomain.City),
			RegistrationDate: &pvzDomain.RegistrationDate,
		}
		// Создаем api.PvzListItem
		apiItem := PvzListItem{
			Pvz:        apiPvzBase,    // Вставляем базовый ПВЗ
			Receptions: apiReceptions, // Вставляем массив приемок
		}
		apiItems = append(apiItems, apiItem)
	}

	// --- 4. Формирование и отправка полного ответа ---
	// Используем сгенерированный тип для всего ответа
	response := PvzListResponse{
		Items:      apiItems,                // Срез PvzListItem (не указатель)
		TotalCount: serviceResult.TotalPVZs, // int (не указатель)
		Page:       page,                    // int (не указатель)
		Limit:      limit,                   // int (не указатель)
	}
	respondWithJSON(w, http.StatusOK, response)
}
