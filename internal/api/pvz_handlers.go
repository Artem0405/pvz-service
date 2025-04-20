package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/google/uuid" // Нужен для uuid.Parse и *uuid.UUID

	// Используем псевдоним, чтобы избежать конфликта имен, если он был
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// HandleCreatePVZ - обработчик для POST /pvz
func (h *Handler) HandleCreatePVZ(w http.ResponseWriter, r *http.Request) {
	var req PVZ // Используем базовый тип PVZ из gen.go
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректное тело запроса: "+err.Error())
		return
	}
	defer r.Body.Close()

	// req.City УЖЕ api.PVZCity
	if req.City == "" {
		respondWithError(w, http.StatusBadRequest, "Поле 'city' является обязательным")
		return
	}

	// Передаем string в сервис
	pvzInput := domain.PVZ{
		City: string(req.City),
	}
	createdPVZDomain, err := h.pvzService.CreatePVZ(r.Context(), pvzInput)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "создание ПВЗ возможно только в городах: Москва, Санкт-Петербург, Казань" {
			respondWithError(w, http.StatusBadRequest, errMsg)
		} else {
			respondWithError(w, http.StatusInternalServerError, "Ошибка при создании ПВЗ: "+errMsg)
		}
		return
	}

	// Конвертация domain.PVZ -> api.PVZ
	// Поля Id и RegistrationDate в api.PVZ - указатели
	var responseIDPtr *openapi_types.UUID
	if createdPVZDomain.ID != uuid.Nil {
		// Так как openapi_types.UUID это псевдоним uuid.UUID,
		// мы можем просто взять адрес от доменного ID
		responseIDPtr = &createdPVZDomain.ID
	}
	var responseDatePtr *time.Time
	if !createdPVZDomain.RegistrationDate.IsZero() {
		responseDatePtr = &createdPVZDomain.RegistrationDate
	}

	responsePVZ := PVZ{
		Id:               responseIDPtr,                  // *uuid.UUID
		City:             PVZCity(createdPVZDomain.City), // api.PVZCity
		RegistrationDate: responseDatePtr,                // *time.Time
	}
	respondWithJSON(w, http.StatusCreated, responsePVZ)
}

// HandleListPVZ - обработчик для GET /pvz с использованием Keyset Pagination
func (h *Handler) HandleListPVZ(w http.ResponseWriter, r *http.Request) {
	// --- 1. Парсинг параметров ---
	q := r.URL.Query()

	// Limit
	limitStr := q.Get("limit")
	limit := 10 // Default
	if limitStr != "" {
		l, errConv := strconv.Atoi(limitStr) // Используем другую переменную для ошибки
		if errConv == nil && l >= 1 && l <= 30 {
			limit = l
		} else {
			respondWithError(w, http.StatusBadRequest, "Некорректное значение для параметра 'limit' (1-30)")
			return
		}
	}

	// Date filters
	var startDatePtr, endDatePtr *time.Time
	if sdStr := q.Get("startDate"); sdStr != "" {
		t, errParse := time.Parse(time.RFC3339, sdStr)
		if errParse != nil {
			respondWithError(w, http.StatusBadRequest, "Некорректный формат startDate (ожидается RFC3339)")
			return
		}
		startDatePtr = &t
	}
	if edStr := q.Get("endDate"); edStr != "" {
		t, errParse := time.Parse(time.RFC3339, edStr)
		if errParse != nil {
			respondWithError(w, http.StatusBadRequest, "Некорректный формат endDate (ожидается RFC3339)")
			return
		}
		endDatePtr = &t
	}

	// Keyset Pagination Parameters
	var afterRegistrationDatePtr *time.Time
	var afterIDPtr *uuid.UUID // Используем *uuid.UUID для передачи в сервис

	afterDateStr := q.Get("after_registration_date")
	afterIDStr := q.Get("after_id")

	if afterDateStr != "" && afterIDStr != "" {
		t, errParse := time.Parse(time.RFC3339, afterDateStr)
		if errParse != nil {
			respondWithError(w, http.StatusBadRequest, "Некорректный формат after_registration_date (ожидается RFC3339)")
			return
		}
		afterRegistrationDatePtr = &t

		id, errParse := uuid.Parse(afterIDStr) // Парсим в uuid.UUID
		if errParse != nil {
			respondWithError(w, http.StatusBadRequest, "Некорректный формат after_id (ожидается UUID)")
			return
		}
		afterIDPtr = &id // Берем указатель на распарсенный uuid.UUID
	} else if afterDateStr != "" || afterIDStr != "" {
		respondWithError(w, http.StatusBadRequest, "Для пагинации необходимо передать оба параметра курсора (after_registration_date и after_id) или ни одного")
		return
	}

	// --- 2. Вызов сервиса с НОВЫМИ параметрами ---
	serviceResult, err := h.pvzService.GetPVZList(r.Context(), startDatePtr, endDatePtr, limit, afterRegistrationDatePtr, afterIDPtr)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Ошибка получения списка ПВЗ: "+err.Error())
		return
	}

	// --- 3. Сборка НОВОГО ответа API (PvzListResponseKeyset) ---
	apiItems := make([]PvzListItem, 0, len(serviceResult.PVZs))

	for _, pvzDomain := range serviceResult.PVZs {
		domainReceptions := serviceResult.Receptions[pvzDomain.ID]
		apiReceptions := make([]ReceptionInfo, 0, len(domainReceptions))

		for _, rcpDomain := range domainReceptions {
			domainProducts := serviceResult.Products[rcpDomain.ID]
			apiProducts := make([]ProductInfo, 0, len(domainProducts)) // ProductInfo = Product

			for _, pDomain := range domainProducts {
				// Конвертируем domain.Product -> api.Product (все поля указатели)
				var apiProductIDPtr *openapi_types.UUID
				if pDomain.ID != uuid.Nil {
					apiProductIDPtr = &pDomain.ID
				}
				var apiProductReceptionIDPtr *openapi_types.UUID
				if pDomain.ReceptionID != uuid.Nil {
					apiProductReceptionIDPtr = &pDomain.ReceptionID
				}
				var apiProductTypePtr *ProductType
				if pDomain.Type != "" {
					tempType := ProductType(pDomain.Type)
					apiProductTypePtr = &tempType
				}
				var apiProductDateTimeAddedPtr *time.Time
				if !pDomain.DateTimeAdded.IsZero() {
					apiProductDateTimeAddedPtr = &pDomain.DateTimeAdded
				}

				apiProduct := ProductInfo{
					Id:            apiProductIDPtr,
					ReceptionId:   apiProductReceptionIDPtr,
					DateTimeAdded: apiProductDateTimeAddedPtr,
					Type:          apiProductTypePtr,
				}
				apiProducts = append(apiProducts, apiProduct)
			}

			// Конвертируем domain.Reception -> api.Reception (все поля указатели)
			var apiReceptionIDPtr *openapi_types.UUID
			if rcpDomain.ID != uuid.Nil {
				apiReceptionIDPtr = &rcpDomain.ID
			}
			var apiReceptionPvzIDPtr *openapi_types.UUID
			if rcpDomain.PVZID != uuid.Nil {
				apiReceptionPvzIDPtr = &rcpDomain.PVZID
			}
			var apiReceptionStatusPtr *ReceptionStatus
			if rcpDomain.Status != "" {
				tempStatus := ReceptionStatus(rcpDomain.Status)
				apiReceptionStatusPtr = &tempStatus
			}
			var apiReceptionDateTimePtr *time.Time
			if !rcpDomain.DateTime.IsZero() {
				apiReceptionDateTimePtr = &rcpDomain.DateTime
			}

			apiReceptionBase := Reception{
				Id:       apiReceptionIDPtr,
				PvzId:    apiReceptionPvzIDPtr,
				DateTime: apiReceptionDateTimePtr,
				Status:   apiReceptionStatusPtr,
			}
			apiReceptionItem := ReceptionInfo{
				Reception: apiReceptionBase, // Структура Reception (не указатель)
				Products:  apiProducts,      // Срез ProductInfo
			}
			apiReceptions = append(apiReceptions, apiReceptionItem)
		}

		// Конвертируем domain.PVZ -> api.PVZ (Id и RegistrationDate - указатели)
		var apiPvzIDPtr *openapi_types.UUID
		if pvzDomain.ID != uuid.Nil {
			apiPvzIDPtr = &pvzDomain.ID
		}
		var apiPvzRegDatePtr *time.Time
		if !pvzDomain.RegistrationDate.IsZero() {
			apiPvzRegDatePtr = &pvzDomain.RegistrationDate
		}

		apiPvzBase := PVZ{
			Id:               apiPvzIDPtr,
			City:             PVZCity(pvzDomain.City), // Не указатель
			RegistrationDate: apiPvzRegDatePtr,
		}
		apiItem := PvzListItem{
			Pvz:        apiPvzBase,    // Структура PVZ (не указатель)
			Receptions: apiReceptions, // Срез ReceptionInfo
		}
		apiItems = append(apiItems, apiItem)
	}

	// --- 4. Формирование НОВОГО полного ответа (PvzListResponseKeyset) ---
	response := PvzListResponseKeyset{
		Items:                     apiItems,
		NextAfterRegistrationDate: serviceResult.NextAfterRegistrationDate, // *time.Time
		NextAfterId:               serviceResult.NextAfterID,               // *uuid.UUID (т.к. openapi_types.UUID - псевдоним)
	}

	respondWithJSON(w, http.StatusOK, response)
}
