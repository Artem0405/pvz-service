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
	"github.com/stretchr/testify/assert" // Можно вернуть для не-статус-код проверок
	"github.com/stretchr/testify/require"
)

const (
	baseURL       = "http://localhost:8080"
	clientTimeout = 30 * time.Second // Увеличим таймаут, так как будет больше запросов
)

type TokenResponse struct {
	Token string `json:"token"`
}

func TestIntegrationScenario(t *testing.T) {
	require := require.New(t) // Внешний require
	assert.New(t)             // Внешний assert

	client := &http.Client{
		Timeout: clientTimeout,
	}

	var moderatorToken string
	var employeeToken string
	var createdPvzId uuid.UUID
	var createdReceptionId uuid.UUID
	var lastCreatedProductId uuid.UUID // Будем хранить ID последнего добавленного товара

	// --- Шаг 0: Получение Тестовых Токенов ---
	t.Run("Step 0: Get Tokens", func(innerT *testing.T) {
		// Модератор
		modLoginReq := api.DummyLoginRequest{Role: "moderator"}
		modBodyBytes, err := json.Marshal(modLoginReq)
		require.NoError(err, innerT)
		statusCode, modRespBody := sendRequest(innerT, client, "POST", baseURL+"/dummyLogin", nil, bytes.NewReader(modBodyBytes))
		if statusCode != http.StatusOK {
			innerT.Fatalf("Dummy login moderator failed: expected status %d, got %d", http.StatusOK, statusCode)
		}
		var modTokenResp TokenResponse
		err = json.Unmarshal(modRespBody, &modTokenResp)
		require.NoError(err, innerT)
		require.NotEmpty(innerT, modTokenResp.Token)
		moderatorToken = modTokenResp.Token
		innerT.Logf("Received moderator token starting with: %s...", moderatorToken[:min(10, len(moderatorToken))])

		// Сотрудник
		empLoginReq := api.DummyLoginRequest{Role: "employee"}
		empBodyBytes, err := json.Marshal(empLoginReq)
		require.NoError(err, innerT)
		statusCode, empRespBody := sendRequest(innerT, client, "POST", baseURL+"/dummyLogin", nil, bytes.NewReader(empBodyBytes))
		if statusCode != http.StatusOK {
			innerT.Fatalf("Dummy login employee failed: expected status %d, got %d", http.StatusOK, statusCode)
		}
		var empTokenResp TokenResponse
		err = json.Unmarshal(empRespBody, &empTokenResp)
		require.NoError(err, innerT)
		require.NotEmpty(innerT, empTokenResp.Token)
		employeeToken = empTokenResp.Token
		innerT.Logf("Received employee token starting with: %s...", employeeToken[:min(10, len(employeeToken))])
	})

	require.NotEmpty(t, moderatorToken)
	require.NotEmpty(t, employeeToken)

	// --- Шаг 1: Создание ПВЗ (Модератор) ---
	t.Run("Step 1: Create PVZ (Moderator)", func(innerT *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + moderatorToken}
		createPvzReq := api.PVZ{City: api.Казань}
		bodyBytes, err := json.Marshal(createPvzReq)
		require.NoError(err, innerT)
		statusCode, respBody := sendRequest(innerT, client, "POST", baseURL+"/pvz", headers, bytes.NewReader(bodyBytes))
		if statusCode != http.StatusCreated {
			innerT.Fatalf("Failed to create PVZ: expected status %d, got %d", http.StatusCreated, statusCode)
		}
		var createdPvz api.PVZ
		err = json.Unmarshal(respBody, &createdPvz)
		require.NoError(err, innerT)
		require.NotNil(innerT, createdPvz.Id)
		require.NotEqual(innerT, uuid.Nil, *createdPvz.Id)
		// Используем assert для некритичных проверок
		if createdPvz.City != api.Казань {
			innerT.Errorf("Created PVZ city mismatch: expected %s, got %s", api.Казань, createdPvz.City)
		}
		if createdPvz.RegistrationDate == nil {
			innerT.Errorf("Created PVZ registration date should not be nil")
		}
		createdPvzId = *createdPvz.Id
		innerT.Logf("PVZ created successfully with ID: %s", createdPvzId)
	})

	require.NotEqual(t, uuid.Nil, createdPvzId)

	// --- Шаг 1b: Попытка Создания ПВЗ (Сотрудник - Ошибка) ---
	t.Run("Step 1b: Fail Create PVZ (Employee)", func(innerT *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		createPvzReq := api.PVZ{City: api.Москва}
		bodyBytes, err := json.Marshal(createPvzReq)
		require.NoError(err, innerT)
		statusCode, _ := sendRequest(innerT, client, "POST", baseURL+"/pvz", headers, bytes.NewReader(bodyBytes))
		if statusCode != http.StatusForbidden {
			innerT.Errorf("Employee should not be able to create PVZ: expected status %d, got %d", http.StatusForbidden, statusCode)
		} else {
			innerT.Log("Verified employee cannot create PVZ (403 Forbidden)")
		}
	})

	// --- Шаг 2: Инициация Приемки (Сотрудник) ---
	t.Run("Step 2: Initiate Reception (Employee)", func(innerT *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		initRecReq := api.PostReceptionsJSONRequestBody{PvzId: createdPvzId}
		bodyBytes, err := json.Marshal(initRecReq)
		require.NoError(err, innerT)
		statusCode, respBody := sendRequest(innerT, client, "POST", baseURL+"/receptions", headers, bytes.NewReader(bodyBytes))
		if statusCode != http.StatusCreated {
			innerT.Fatalf("Failed to initiate reception: expected status %d, got %d", http.StatusCreated, statusCode)
		}
		var createdRec api.Reception
		err = json.Unmarshal(respBody, &createdRec)
		require.NoError(err, innerT)
		require.NotNil(innerT, createdRec.Id)
		require.NotEqual(innerT, uuid.Nil, *createdRec.Id)
		if createdRec.PvzId != createdPvzId {
			innerT.Errorf("Reception PVZ ID mismatch: expected %s, got %s", createdPvzId, createdRec.PvzId)
		}
		if createdRec.Status != api.InProgress {
			innerT.Errorf("Reception status mismatch: expected %s, got %s", api.InProgress, createdRec.Status)
		}
		if createdRec.DateTime == nil {
			innerT.Errorf("Reception DateTime should not be nil")
		}
		createdReceptionId = *createdRec.Id
		innerT.Logf("Reception initiated successfully with ID: %s", createdReceptionId)
	})

	require.NotEqual(t, uuid.Nil, createdReceptionId)

	// --- Шаг 3: Добавление 50 Товаров (Сотрудник) ---
	// ***** МОДИФИКАЦИЯ ЗДЕСЬ *****
	t.Run("Step 3: Add 50 Products (Employee)", func(innerT *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		// Используем один тип товара для простоты
		productType := api.Одежда

		innerT.Logf("Starting to add 50 products of type %s...", productType)

		for i := 0; i < 50; i++ {
			addProdReq := api.PostProductsJSONRequestBody{PvzId: createdPvzId, Type: productType}
			bodyBytes, err := json.Marshal(addProdReq)
			// Используем require.NoError, т.к. ошибка маршалинга критична
			require.NoError(err, innerT, "Failed to marshal add product request for item %d", i+1)

			statusCode, respBody := sendRequest(innerT, client, "POST", baseURL+"/products", headers, bytes.NewReader(bodyBytes))

			// Ручная проверка статус кода
			if statusCode != http.StatusCreated {
				var errResp api.Error
				_ = json.Unmarshal(respBody, &errResp)
				// Используем Fatalf, так как если один товар не добавился, продолжать бессмысленно
				innerT.Fatalf("Failed to add product %d: expected status %d, got %d. Response body: %s",
					i+1, http.StatusCreated, statusCode, string(respBody))
			}

			// Проверяем ответ (необязательно, но полезно)
			var addedProd api.Product
			err = json.Unmarshal(respBody, &addedProd)
			require.NoError(err, innerT, "Failed to unmarshal add product response for item %d", i+1)
			require.NotNil(innerT, addedProd.Id, "Added Product ID is nil for item %d", i+1)
			require.NotEqual(innerT, uuid.Nil, *addedProd.Id, "Added Product ID is zero UUID for item %d", i+1)

			// Проверяем детали последнего добавленного товара (необязательно)
			if i == 49 { // Проверяем только последний для примера
				if addedProd.ReceptionId != createdReceptionId {
					innerT.Errorf("Last Product Reception ID mismatch: expected %s, got %s", createdReceptionId, addedProd.ReceptionId)
				}
				if addedProd.Type != productType {
					innerT.Errorf("Last Product Type mismatch: expected %s, got %s", productType, addedProd.Type)
				}
				if addedProd.DateTimeAdded == nil {
					innerT.Errorf("Last Product DateTimeAdded should not be nil")
				}
			}

			// Сохраняем ID *последнего* добавленного товара
			lastCreatedProductId = *addedProd.Id

			// Логируем прогресс (не для каждого товара, чтобы не засорять вывод)
			if (i+1)%10 == 0 {
				innerT.Logf("Added product %d of 50...", i+1)
			}
		}
		innerT.Logf("Successfully added 50 products. Last product ID: %s", lastCreatedProductId)
	}) // <--- Закрытие t.Run Шага 3

	// Проверяем, что ID последнего товара был сохранен
	require.NotEqual(t, uuid.Nil, lastCreatedProductId, "Last Product ID was not set after adding 50 products")

	// --- Шаг 4: Удаление Последнего Товара (Сотрудник) ---
	t.Run("Step 4: Delete Last Product (Employee)", func(innerT *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		url := fmt.Sprintf("%s/pvz/%s/delete_last_product", baseURL, createdPvzId)
		statusCode, respBody := sendRequest(innerT, client, "POST", url, headers, nil)
		if statusCode != http.StatusOK {
			innerT.Fatalf("Failed to delete last product: expected status %d, got %d", http.StatusOK, statusCode)
		}
		var msgResp api.Error
		err := json.Unmarshal(respBody, &msgResp)
		require.NoError(err, innerT)
		expectedMsg := "Последний добавленный товар удален"
		if msgResp.Message != expectedMsg {
			innerT.Errorf("Delete product message mismatch: expected '%s', got '%s'", expectedMsg, msgResp.Message)
		}
		innerT.Log("Last product (one of 50) deleted successfully") // Уточняем лог
	})

	// --- Шаг 5: Закрытие Приемки (Сотрудник) ---
	t.Run("Step 5: Close Reception (Employee)", func(innerT *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		url := fmt.Sprintf("%s/pvz/%s/close_last_reception", baseURL, createdPvzId)
		statusCode, respBody := sendRequest(innerT, client, "POST", url, headers, nil)
		if statusCode != http.StatusOK {
			innerT.Fatalf("Failed to close reception: expected status %d, got %d", http.StatusOK, statusCode)
		}
		var closedRec api.Reception
		err := json.Unmarshal(respBody, &closedRec)
		require.NoError(err, innerT)
		require.NotNil(innerT, closedRec.Id)
		if *closedRec.Id != createdReceptionId {
			innerT.Errorf("Closed reception ID mismatch: expected %s, got %s", createdReceptionId, *closedRec.Id)
		}
		if closedRec.Status != api.Closed {
			innerT.Errorf("Reception status mismatch: expected %s, got %s", api.Closed, closedRec.Status)
		}
		innerT.Logf("Reception closed successfully with ID: %s", *closedRec.Id)
	})

	// --- Шаг 6: Получение Списка ПВЗ (Сотрудник) ---
	t.Run("Step 6: List PVZ (Employee)", func(innerT *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + employeeToken}
		url := fmt.Sprintf("%s/pvz?page=1&limit=10", baseURL)
		statusCode, respBody := sendRequest(innerT, client, "GET", url, headers, nil)
		innerT.Logf("List PVZ response status code: %d", statusCode)
		if statusCode != http.StatusOK {
			innerT.Fatalf("Failed to list PVZ: expected status %d, got %d", http.StatusOK, statusCode)
		}
		innerT.Log("List PVZ status code check passed.")
		var listResp api.PvzListResponse
		err := json.Unmarshal(respBody, &listResp)
		require.NoError(err, innerT, "Failed to unmarshal PVZ list response")

		// Ручные проверки
		if listResp.TotalCount < 1 {
			innerT.Errorf("Total count should be at least 1, got %d", listResp.TotalCount)
		}
		if listResp.Page != 1 {
			innerT.Errorf("Page number mismatch: expected 1, got %d", listResp.Page)
		}
		if listResp.Limit != 10 {
			innerT.Errorf("Limit mismatch: expected 10, got %d", listResp.Limit)
		}
		if len(listResp.Items) == 0 {
			innerT.Errorf("PVZ list items should not be empty")
		}

		foundPvz := false
		for i, item := range listResp.Items {
			if item.Pvz.Id == nil {
				innerT.Errorf("PVZ ID in list item %d should not be nil", i)
				continue
			}
			if *item.Pvz.Id == createdPvzId {
				foundPvz = true
				if item.Pvz.City != api.Казань {
					innerT.Errorf("PVZ city in list mismatch for PVZ ID %s: expected %s, got %s", createdPvzId, api.Казань, item.Pvz.City)
				}
				if len(item.Receptions) == 0 {
					innerT.Errorf("Receptions for the created PVZ (ID: %s) should not be empty", createdPvzId)
				}

				foundReception := false
				expectedProductCount := 49 // Ожидаем 49 товаров после удаления одного
				for j, recInfo := range item.Receptions {
					if recInfo.Reception.Id == nil {
						innerT.Errorf("Reception ID in list item %d, reception %d should not be nil", i, j)
						continue
					}
					if *recInfo.Reception.Id == createdReceptionId {
						foundReception = true
						if recInfo.Reception.Status != api.Closed {
							innerT.Errorf("Reception status in list mismatch for Reception ID %s: expected %s, got %s", createdReceptionId, api.Closed, recInfo.Reception.Status)
						}
						// ***** МОДИФИКАЦИЯ ПРОВЕРКИ ЗДЕСЬ *****
						if len(recInfo.Products) != expectedProductCount {
							innerT.Errorf("Products count in the closed reception (ID: %s) mismatch: expected %d, got %d products", createdReceptionId, expectedProductCount, len(recInfo.Products))
						}
						break
					}
				}
				if !foundReception {
					innerT.Errorf("Created reception (ID: %s) was not found in the list for the PVZ (ID: %s)", createdReceptionId, createdPvzId)
				}
				break
			}
		}
		if !foundPvz {
			innerT.Errorf("Created PVZ (ID: %s) was not found in the list", createdPvzId)
		}
		if innerT.Failed() {
			innerT.Log("One or more checks failed in Step 6")
		} else {
			innerT.Log("PVZ list retrieved and validated (manually)")
		}
	}) // <--- ЗАКРЫТИЕ t.Run Шага 6

	// --- Шаг 7: Проверка Health Check ---
	t.Run("Step 7: Health Check", func(innerT *testing.T) {
		statusCode, respBody := sendRequest(innerT, client, "GET", baseURL+"/health", nil, nil)
		innerT.Logf("Health Check response status code: %d", statusCode)
		if statusCode != http.StatusOK {
			innerT.Fatalf("Health check failed: expected status %d, got %d", http.StatusOK, statusCode)
		}
		innerT.Log("Health Check status code check passed.")
		require.NotEmpty(innerT, respBody)
		var healthResp map[string]string
		err := json.Unmarshal(respBody, &healthResp)
		require.NoError(err, innerT)
		expectedStatus := "ok"
		if healthResp["status"] != expectedStatus {
			innerT.Errorf("Health status mismatch: expected '%s', got '%s'", expectedStatus, healthResp["status"])
		}
		expectedDBStatus := "up"
		if healthResp["database"] != expectedDBStatus {
			innerT.Errorf("Database status mismatch: expected '%s', got '%s'", expectedDBStatus, healthResp["database"])
		}
		innerT.Log("Health check successful and validated")
	}) // <--- ЗАКРЫТИЕ t.Run Шага 7

} // <--- Закрытие func TestIntegrationScenario

// ... функции sendRequest, min, TestSimpleAssert, TestBasicGoTest ...
// (Они остаются без изменений)

// sendRequest - вспомогательная функция
func sendRequest(testContextT *testing.T, client *http.Client, method, url string, headers map[string]string, body io.Reader) (int, []byte) {
	testContextT.Helper()
	req, err := http.NewRequest(method, url, body)
	require.NoError(testContextT, err, "Failed to create request (%s %s)", method, url)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	require.NoError(testContextT, err, "Failed to execute request (%s %s)", method, url)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(testContextT, err, "Failed to read response body (%s %s)", method, url)
	return resp.StatusCode, respBody
}

// Вспомогательная функция для безопасного среза строки
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ----- Тестовые функции для диагностики -----
func TestSimpleAssert(simpleT *testing.T) {
	assert := assert.New(simpleT)
	myBool := true
	assert.True(myBool, simpleT, "This must pass")
}

func TestBasicGoTest(basicT *testing.T) {
	myBool := true
	if !myBool {
		basicT.Errorf("Basic boolean check failed, expected true")
	}
	basicT.Log("Basic Go test checking 'true' passed.")
	myFalseBool := false
	if myFalseBool {
		basicT.Errorf("Basic boolean check failed, expected false")
	}
	basicT.Log("Basic Go test checking 'false' passed.")
}
