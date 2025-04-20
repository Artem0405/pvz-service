package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Artem0405/pvz-service/internal/api"
	"github.com/google/uuid"

	// Используем только require для прерывания теста при критических ошибках
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	baseURL       = "http://localhost:8080"
	clientTimeout = 30 * time.Second
)

// Структура для ответа с токеном
type TokenResponse struct {
	Token string `json:"token"`
}

// Основной интеграционный тест
func TestIntegrationScenario(t *testing.T) {
	require := require.New(t) // Внешний require для всего теста

	client := &http.Client{
		Timeout: clientTimeout,
	}

	// Переменные для хранения данных между шагами
	var moderatorToken string
	var employeeToken string
	var createdPvzId uuid.UUID
	var createdReceptionId uuid.UUID
	var lastCreatedProductId uuid.UUID

	// --- Шаг 0: Получение Тестовых Токенов ---
	t.Run("Get Tokens", func(t *testing.T) {
		// --- Модератор ---
		modLoginReq := api.DummyLoginRequest{Role: api.Moderator}
		modBodyBytes, err := json.Marshal(modLoginReq)
		require.NoError(err, t, "Get Tokens: Failed to marshal moderator request") // Прерываем, если маршалинг не удался
		statusCode, modRespBody := sendRequest(t, client, "POST", baseURL+"/dummyLogin", nil, bytes.NewReader(modBodyBytes))

		// Ручная проверка статус кода с прерыванием при ошибке
		if statusCode != http.StatusOK {
			require.FailNowf("Get Tokens: Dummy login moderator failed", "Expected status %d, got %d. Body: %s", http.StatusOK, t, statusCode, string(modRespBody))
		}

		var modTokenResp TokenResponse
		err = json.Unmarshal(modRespBody, &modTokenResp)
		require.NoError(err, t, "Get Tokens: Failed to unmarshal moderator token response")
		require.NotEmpty(t, modTokenResp.Token, "Get Tokens: Moderator token is empty")
		moderatorToken = modTokenResp.Token
		t.Logf("Received moderator token starting with: %s...", moderatorToken[:min(10, len(moderatorToken))])

		// --- Сотрудник ---
		empLoginReq := api.DummyLoginRequest{Role: api.Employee}
		empBodyBytes, err := json.Marshal(empLoginReq)
		require.NoError(err, t, "Get Tokens: Failed to marshal employee request")
		statusCode, empRespBody := sendRequest(t, client, "POST", baseURL+"/dummyLogin", nil, bytes.NewReader(empBodyBytes))

		if statusCode != http.StatusOK {
			require.FailNowf("Get Tokens: Dummy login employee failed", "Expected status %d, got %d. Body: %s", http.StatusOK, t, statusCode, string(empRespBody))
		}

		var empTokenResp TokenResponse
		err = json.Unmarshal(empRespBody, &empTokenResp)
		require.NoError(err, t, "Get Tokens: Failed to unmarshal employee token response")
		require.NotEmpty(t, empTokenResp.Token, "Get Tokens: Employee token is empty")
		employeeToken = empTokenResp.Token
		t.Logf("Received employee token starting with: %s...", employeeToken[:min(10, len(employeeToken))])
	})

	// Проверяем, что токены получены перед основными шагами
	require.NotEmpty(t, moderatorToken, "Moderator token was not set after Get Tokens step")
	require.NotEmpty(t, employeeToken, "Employee token was not set after Get Tokens step")

	// --- Шаг 1: Создание ПВЗ (Модератор) ---
	t.Run("Create PVZ (Moderator)", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + moderatorToken}
		createPvzReq := api.PVZ{City: api.Казань}
		bodyBytes, err := json.Marshal(createPvzReq)
		require.NoError(err, t, "Create PVZ: Failed to marshal request")
		statusCode, respBody := sendRequest(t, client, "POST", baseURL+"/pvz", headers, bytes.NewReader(bodyBytes))

		if statusCode != http.StatusCreated {
			require.FailNowf("Create PVZ: Failed to create PVZ", "Expected status %d, got %d. Body: %s", http.StatusCreated, t, statusCode, string(respBody))
		}

		var createdPvz api.PVZ
		err = json.Unmarshal(respBody, &createdPvz)
		require.NoError(err, t, "Create PVZ: Failed to unmarshal response")
		require.NotNil(t, createdPvz.Id, "Create PVZ: Created PVZ ID is nil")
		if *createdPvz.Id == uuid.Nil {
			require.FailNow("Create PVZ: Created PVZ ID is zero UUID", t)
		}
		if createdPvz.City != api.Казань {
			t.Errorf("Create PVZ: City mismatch: expected %s, got %s", api.Казань, createdPvz.City)
		}
		if createdPvz.RegistrationDate == nil {
			t.Errorf("Create PVZ: Registration date should not be nil")
		}

		createdPvzId = *createdPvz.Id
		t.Logf("PVZ created successfully with ID: %s", createdPvzId)
	})

	require.NotEqual(t, uuid.Nil, createdPvzId, "PVZ ID was not set after creation step")

	// --- Шаг 1b: Попытка Создания ПВЗ (Сотрудник - Ошибка) ---
	t.Run("Fail Create PVZ (Employee)", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		createPvzReq := api.PVZ{City: api.Москва}
		bodyBytes, err := json.Marshal(createPvzReq)
		require.NoError(err, t, "Fail Create PVZ: Failed to marshal request")
		statusCode, _ := sendRequest(t, client, "POST", baseURL+"/pvz", headers, bytes.NewReader(bodyBytes))

		if statusCode != http.StatusForbidden {
			// Используем Errorf, так как это проверка ожидаемой ошибки, тест не должен падать
			t.Errorf("Fail Create PVZ: Employee should not be able to create PVZ: expected status %d, got %d", http.StatusForbidden, statusCode)
		} else {
			t.Log("Verified employee cannot create PVZ (403 Forbidden)")
		}
	})

	// --- Шаг 2: Инициация Приемки (Сотрудник) ---
	t.Run("Initiate Reception (Employee)", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		initRecReq := api.InitiateReceptionRequest{PvzId: createdPvzId}
		bodyBytes, err := json.Marshal(initRecReq)
		require.NoError(err, t, "Initiate Reception: Failed to marshal request")
		statusCode, respBody := sendRequest(t, client, "POST", baseURL+"/receptions", headers, bytes.NewReader(bodyBytes))

		if statusCode != http.StatusCreated {
			require.FailNowf("Initiate Reception: Failed to initiate reception", "Expected status %d, got %d. Body: %s", http.StatusCreated, t, statusCode, string(respBody))
		}

		var createdRec api.Reception
		err = json.Unmarshal(respBody, &createdRec)
		require.NoError(err, t, "Initiate Reception: Failed to unmarshal response")
		require.NotNil(t, createdRec.Id, "Initiate Reception: Created Reception ID is nil")
		if *createdRec.Id == uuid.Nil {
			require.FailNow("Initiate Reception: Created Reception ID is zero UUID", t)
		}
		require.NotNil(t, createdRec.PvzId, "Initiate Reception: Created Reception PVZ ID is nil")
		require.NotNil(t, createdRec.Status, "Initiate Reception: Created Reception Status is nil")
		if *createdRec.PvzId != createdPvzId {
			t.Errorf("Initiate Reception: PVZ ID mismatch: expected %s, got %s", createdPvzId, *createdRec.PvzId)
		}
		if *createdRec.Status != api.InProgress {
			t.Errorf("Initiate Reception: status mismatch: expected %s, got %s", api.InProgress, *createdRec.Status)
		}
		if createdRec.DateTime == nil {
			t.Errorf("Initiate Reception: DateTime should not be nil")
		}

		createdReceptionId = *createdRec.Id
		t.Logf("Reception initiated successfully with ID: %s", createdReceptionId)
	})

	require.NotEqual(t, uuid.Nil, createdReceptionId, "Reception ID was not set after initiation step")

	// --- Шаг 3: Добавление 50 Товаров (Сотрудник) ---
	t.Run("Add 50 Products (Employee)", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		productType := api.Одежда

		t.Logf("Starting to add 50 products of type %s...", productType)
		var lastAddedProd *api.Product

		for i := 0; i < 50; i++ {
			addProdReq := api.AddProductRequest{PvzId: createdPvzId, Type: productType}
			bodyBytes, err := json.Marshal(addProdReq)
			require.NoError(err, t, "Add Products: Failed to marshal request for item %d", i+1)

			statusCode, respBody := sendRequest(t, client, "POST", baseURL+"/products", headers, bytes.NewReader(bodyBytes))

			if statusCode != http.StatusCreated {
				require.FailNowf("Add Products: Failed to add product", "Item %d: expected status %d, got %d. Body: %s", i+1, http.StatusCreated, t, statusCode, string(respBody))
			}

			var addedProd api.Product
			err = json.Unmarshal(respBody, &addedProd)
			require.NoError(err, t, "Add Products: Failed to unmarshal response for item %d", i+1)
			require.NotNil(t, addedProd.Id, "Add Products: Added Product ID is nil for item %d", i+1)
			if *addedProd.Id == uuid.Nil {
				t.Fatalf("Add Products: Added Product ID is zero UUID for item %d", i+1)
			}

			lastAddedProd = &addedProd
			lastCreatedProductId = *addedProd.Id

			if (i+1)%10 == 0 {
				t.Logf("Added product %d of 50...", i+1)
			}
		}
		t.Logf("Successfully added 50 products. Last product ID: %s", lastCreatedProductId)

		// Проверяем поля последнего добавленного товара
		require.NotNil(t, lastAddedProd, "Add Products: Last added product pointer should not be nil")
		if lastAddedProd.ReceptionId == nil {
			t.Errorf("Add Products: Last Product Reception ID is nil")
		} else if *lastAddedProd.ReceptionId != createdReceptionId {
			t.Errorf("Add Products: Last Product Reception ID mismatch: expected %s, got %s", createdReceptionId, *lastAddedProd.ReceptionId)
		}
		if lastAddedProd.Type == nil {
			t.Errorf("Add Products: Last Product Type is nil")
		} else if *lastAddedProd.Type != productType {
			t.Errorf("Add Products: Last Product Type mismatch: expected %s, got %s", productType, *lastAddedProd.Type)
		}
		if lastAddedProd.DateTimeAdded == nil {
			t.Errorf("Add Products: Last Product DateTimeAdded should not be nil")
		}
	})

	require.NotEqual(t, uuid.Nil, lastCreatedProductId, "Last Product ID was not set after adding products")

	// --- Шаг 4: Удаление Последнего Товара (Сотрудник) ---
	t.Run("Delete Last Product (Employee)", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		url := fmt.Sprintf("%s/pvz/%s/delete_last_product", baseURL, createdPvzId)
		statusCode, respBody := sendRequest(t, client, "POST", url, headers, nil)

		if statusCode != http.StatusOK {
			require.FailNowf("Delete Last Product: Failed to delete", "Expected status %d, got %d. Body: %s", http.StatusOK, t, statusCode, string(respBody))
		}

		var msgResp api.MessageResponse
		err := json.Unmarshal(respBody, &msgResp)
		require.NoError(err, t, "Delete Last Product: Failed to unmarshal response")
		expectedMsg := "Последний добавленный товар удален"
		if msgResp.Message != expectedMsg {
			t.Errorf("Delete Last Product: Message mismatch: expected '%s', got '%s'", expectedMsg, msgResp.Message)
		}
		t.Log("Last product deleted successfully")
	})

	// --- Шаг 5: Закрытие Приемки (Сотрудник) ---
	t.Run("Close Reception (Employee)", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		url := fmt.Sprintf("%s/pvz/%s/close_last_reception", baseURL, createdPvzId)
		statusCode, respBody := sendRequest(t, client, "POST", url, headers, nil)

		if statusCode != http.StatusOK {
			require.FailNowf("Close Reception: Failed to close", "Expected status %d, got %d. Body: %s", http.StatusOK, t, statusCode, string(respBody))
		}

		var closedRec api.Reception
		err := json.Unmarshal(respBody, &closedRec)
		require.NoError(err, t, "Close Reception: Failed to unmarshal response")
		require.NotNil(t, closedRec.Id, "Close Reception: Closed reception ID is nil")
		if *closedRec.Id != createdReceptionId {
			t.Errorf("Close Reception: ID mismatch: expected %s, got %s", createdReceptionId, *closedRec.Id)
		}
		require.NotNil(t, closedRec.Status, "Close Reception: Status is nil")
		if *closedRec.Status != api.Closed {
			t.Errorf("Close Reception: Status mismatch: expected %s, got %s", api.Closed, *closedRec.Status)
		}
		t.Logf("Reception closed successfully with ID: %s", *closedRec.Id)
	})

	// --- Шаг 6: Получение Списка ПВЗ (Сотрудник) ---
	t.Run("List PVZ (Employee)", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		url := fmt.Sprintf("%s/pvz?limit=10", baseURL) // Keyset - первая страница
		statusCode, respBody := sendRequest(t, client, "GET", url, headers, nil)

		if !assert.True(t, http.StatusOK == statusCode, fmt.Sprintf("List PVZ: Failed to list PVZ: expected status %d, got %d", http.StatusOK, statusCode)) {
			// Если проверка не удалась, останавливаем тест этого шага
			require.FailNowf("Stopping test due to status code mismatch", "Expected %d, got %d", http.StatusOK, t, statusCode)
		}

		var listResp api.PvzListResponseKeyset
		err := json.Unmarshal(respBody, &listResp)
		require.NoError(err, t, "List PVZ: Failed to unmarshal response")

		if len(listResp.Items) == 0 {
			t.Errorf("List PVZ: List items should not be empty")
		}

		foundPvz := false
		for i, item := range listResp.Items {
			require.NotNil(t, item.Pvz.Id, "List PVZ: Item %d: PVZ ID is nil", i)
			if *item.Pvz.Id == createdPvzId {
				foundPvz = true
				if item.Pvz.City != api.Казань {
					t.Errorf("List PVZ: PVZ %s: City mismatch: expected %s, got %s", createdPvzId, api.Казань, item.Pvz.City)
				}
				if len(item.Receptions) == 0 {
					t.Errorf("List PVZ: PVZ %s: Receptions should not be empty", createdPvzId)
				}

				foundReception := false
				expectedProductCount := 49 // 50 добавили, 1 удалили
				for j, recInfo := range item.Receptions {
					require.NotNil(t, recInfo.Reception.Id, "List PVZ: Item %d, Reception %d: ID is nil", i, j)
					if *recInfo.Reception.Id == createdReceptionId {
						foundReception = true
						require.NotNil(t, recInfo.Reception.Status, "List PVZ: Item %d, Reception %d: Status is nil", i, j)
						if *recInfo.Reception.Status != api.Closed {
							t.Errorf("List PVZ: Reception %s: Status mismatch: expected %s, got %s", createdReceptionId, api.Closed, *recInfo.Reception.Status)
						}
						actualProductCount := len(recInfo.Products)
						if actualProductCount != expectedProductCount {
							t.Errorf("List PVZ: Reception %s: Products count mismatch: expected %d, got %d", createdReceptionId, expectedProductCount, actualProductCount)
						}
						break // Нашли нужную приемку
					}
				}
				if !foundReception {
					t.Errorf("List PVZ: Created reception (ID: %s) was not found for PVZ (ID: %s)", createdReceptionId, createdPvzId)
				}
				break // Нашли нужный ПВЗ
			}
		}
		if !foundPvz {
			t.Errorf("List PVZ: Created PVZ (ID: %s) was not found in the list", createdPvzId)
		}
		t.Log("PVZ list retrieved and validated")
	})

	// --- Шаг 7: Проверка Health Check ---
	t.Run("Health Check", func(t *testing.T) {
		statusCode, respBody := sendRequest(t, client, "GET", baseURL+"/health", nil, nil)
		if !assert.True(t, http.StatusOK == statusCode, fmt.Sprintf("Health check failed: expected %d, got %d", http.StatusOK, statusCode)) {
			require.FailNowf("Stopping test due to status code mismatch in Health Check", "Expected %d, got %d", http.StatusOK, t, statusCode)
		}
		require.NotEmpty(t, respBody, "Health check response body is empty")

		var healthResp map[string]string
		err := json.Unmarshal(respBody, &healthResp)
		require.NoError(err, t, "Failed to unmarshal health response")
		if healthResp["status"] != "ok" {
			t.Errorf("Health status mismatch: expected 'ok', got '%s'", healthResp["status"])
		}
		if healthResp["database"] != "up" {
			t.Errorf("Database status mismatch: expected 'up', got '%s'", healthResp["database"])
		}
		t.Log("Health check successful and validated")
	})
}

// --- Вспомогательные функции ---
func sendRequest(t *testing.T, client *http.Client, method, url string, headers map[string]string, body io.Reader) (int, []byte) {
	// Используем require внутри хелпера, чтобы прерывать выполнение t.Run при ошибке запроса
	localRequire := require.New(t)
	t.Helper() // Помечаем как хелпер
	req, err := http.NewRequest(method, url, body)
	localRequire.NoError(err, "Helper: Failed to create request (%s %s)", method, url)
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	localRequire.NoError(err, "Helper: Failed to execute request (%s %s)", method, url)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	localRequire.NoError(err, "Helper: Failed to read response body (%s %s)", method, url)
	return resp.StatusCode, respBody
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
