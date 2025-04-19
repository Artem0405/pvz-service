package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Artem0405/pvz-service/internal/domain"
	"github.com/Artem0405/pvz-service/internal/repository"
	openapi_types "github.com/oapi-codegen/runtime/types" // Импорт для openapi_types.Email и UUID
)

// HandleRegister - обработчик для POST /register
func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	// Используем сгенерированный тип тела запроса
	var req PostRegisterJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректное тело запроса: "+err.Error())
		return
	}
	defer r.Body.Close()

	// Валидация (поля не указатели, проверяем на пустые значения)
	if req.Email == "" { // Тип openapi_types.Email - базовый string
		respondWithError(w, http.StatusBadRequest, "Поле 'email' обязательно")
		return
	}
	if req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Поле 'password' обязательно")
		return
	}
	if req.Role == "" { // Тип PostRegisterJSONBodyRole - базовый string
		respondWithError(w, http.StatusBadRequest, "Поле 'role' обязательно")
		return
	}

	// Передаем значения в сервис, приводя кастомные типы к string
	userDomain, err := h.authService.Register(r.Context(), string(req.Email), req.Password, string(req.Role))
	if err != nil {
		// Обработка ошибок (логика остается)
		if errors.Is(err, repository.ErrUserDuplicateEmail) || err.Error() == "пользователь с таким email уже существует" {
			respondWithError(w, http.StatusConflict, err.Error()) // 409 Conflict
		} else if err.Error() == "email и пароль не могут быть пустыми" || err.Error() == "недопустимая роль пользователя" {
			respondWithError(w, http.StatusBadRequest, err.Error()) // 400 Bad Request
		} else {
			respondWithError(w, http.StatusInternalServerError, "Не удалось зарегистрировать пользователя")
		}
		return
	}

	// Конвертация domain.User в api.User для ответа
	// Используем сгенерированный тип User
	userAPI := User{
		Id:    &userDomain.ID,                        // Id в DTO - *openapi_types.UUID, берем адрес
		Email: openapi_types.Email(userDomain.Email), // Приводим string к openapi_types.Email
		Role:  UserRole(userDomain.Role),             // Приводим string к UserRole
	}

	// Отвечаем 201 Created с данными пользователя (уже в формате API DTO)
	respondWithJSON(w, http.StatusCreated, userAPI)
}

// HandleLogin - обработчик для POST /login
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req PostLoginJSONRequestBody // Используем сгенерированный тип запроса
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректное тело запроса: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.Email == "" {
		respondWithError(w, http.StatusBadRequest, "Поле 'email' обязательно")
		return
	}
	if req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Поле 'password' обязательно")
		return
	}

	tokenString, err := h.authService.Login(r.Context(), string(req.Email), req.Password)
	if err != nil {
		if err.Error() == "неверный email или пароль" {
			respondWithError(w, http.StatusUnauthorized, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Ошибка входа в систему")
		}
		return
	}

	// --- ИЗМЕНЕНИЕ: Возвращаем JSON-объект ---
	// Используем map[string]string для простоты, или сгенерированный тип, если он объект
	// Предположим, ваш сгенерированный Token - это все еще псевдоним string, поэтому используем map
	responsePayload := map[string]string{"token": tokenString}
	respondWithJSON(w, http.StatusOK, responsePayload)
	// --- Конец изменения ---
}

// HandleDummyLogin - обработчик для POST /dummyLogin
func (h *Handler) HandleDummyLogin(w http.ResponseWriter, r *http.Request) {
	var req PostDummyLoginJSONRequestBody // Используем сгенерированный тип запроса
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректное тело запроса: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.Role == "" {
		respondWithError(w, http.StatusBadRequest, "Поле 'role' обязательно")
		return
	}

	role := string(req.Role)
	if role != domain.RoleEmployee && role != domain.RoleModerator {
		respondWithError(w, http.StatusBadRequest, "Недопустимая роль. Допустимые роли: 'employee', 'moderator'")
		return
	}

	tokenString, err := h.authService.GenerateToken(role)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Не удалось сгенерировать токен")
		return
	}

	// --- ИЗМЕНЕНИЕ: Возвращаем JSON-объект ---
	responsePayload := map[string]string{"token": tokenString}
	respondWithJSON(w, http.StatusOK, responsePayload)
	// --- Конец изменения ---
}

// --- Определения старых DTO удалены отсюда ---
