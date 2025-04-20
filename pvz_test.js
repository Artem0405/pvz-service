// pvz_test.js
import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Trend, Rate, Counter } from 'k6/metrics';
import { SharedArray } from 'k6/data';

http.setResponseCallback(http.expectedStatuses({ min: 200, max: 400 })); // Считать все 2xx, 3xx, 400 как НЕ failed
// --- Глобальные переменные и Настройки ---
// !!! ЗАМЕНИТЕ НА АДРЕС ВАШЕГО ТЕСТОВОГО СЕРВИСА !!!
const BASE_URL = 'http://localhost:8080';

// Пользовательские метрики
let pvzListLatency = new Trend('pvz_list_latency');
let createPvzLatency = new Trend('create_pvz_latency');
let initiateReceptionLatency = new Trend('initiate_reception_latency');
let addProductLatency = new Trend('add_product_latency');
let closeReceptionLatency = new Trend('close_reception_latency'); // Метрика для закрытия
let errorRate = new Rate('errors'); // Кастомный счетчик ошибок на основе checks

// --- Конфигурация Нагрузки ---
export const options = {
    stages: [
        { duration: '30s', target: 50 },  // Разгон до 50
        { duration: '1m', target: 100 }, // Разгон до 100
        { duration: '3m', target: 100 }, // Стабильная нагрузка 100 VUs
        { duration: '30s', target: 0 },   // Снижение
    ],

    thresholds: {
        'http_req_duration': ['p(95)<100'], // p95 < 100ms - ВАШ SLI
        'http_req_failed': ['rate<0.001'],  // < 0.1% ошибок сети/5xx (близко к 99.99% успеха)
        'checks': ['rate>0.98'],           // > 98% чеков должны проходить
        'errors': ['rate<0.01'],           // < 1% кастомных ошибок (проваленных check)
    },
    // timeout: '120s', // Можно раскомментировать, если таймауты продолжатся
};

// --- Функция Setup: выполняется один раз перед тестом ---
export function setup() {
    console.log('Running setup...');
    let moderatorToken = null;
    let employeeToken = null;
    let basePvzId = null;

    // 1. Получаем токен модератора
    try {
        let modRes = http.post(`${BASE_URL}/dummyLogin`, JSON.stringify({ role: 'moderator' }), { headers: { 'Content-Type': 'application/json' }, timeout: '30s' });
        if (modRes.status === 200 && modRes.json().token) {
            moderatorToken = modRes.json().token;
            console.log('Moderator token obtained.');
        } else {
            throw new Error(`Setup: Failed to get moderator token. Status: ${modRes.status}, Body: ${modRes.body}`);
        }
    } catch (e) {
         console.error(e.message);
         throw e;
    }

    // 2. Получаем токен сотрудника
     try {
        let empRes = http.post(`${BASE_URL}/dummyLogin`, JSON.stringify({ role: 'employee' }), { headers: { 'Content-Type': 'application/json' }, timeout: '30s' });
        if (empRes.status === 200 && empRes.json().token) {
            employeeToken = empRes.json().token;
            console.log('Employee token obtained.');
        } else {
             throw new Error(`Setup: Failed to get employee token. Status: ${empRes.status}, Body: ${empRes.body}`);
        }
     } catch(e) {
         console.error(e.message);
         throw e;
     }

    // 3. Создаем один базовый ПВЗ для теста (используя токен модератора)
    try {
        let createPvzPayload = JSON.stringify({ city: "Казань" });
        let createPvzHeaders = {
            headers: {
                'Authorization': `Bearer ${moderatorToken}`,
                'Content-Type': 'application/json',
            },
             timeout: '30s'
        };
        let createRes = http.post(`${BASE_URL}/pvz`, createPvzPayload, createPvzHeaders);
        if (createRes.status === 201 && createRes.json().id) {
             basePvzId = createRes.json().id;
             console.log(`Setup: Base PVZ created with ID: ${basePvzId}`);
        } else {
             // Попытка создать еще раз, если вдруг конфликт (маловероятно с UUID)
             sleep(0.5);
             createRes = http.post(`${BASE_URL}/pvz`, createPvzPayload, createPvzHeaders);
              if (createRes.status === 201 && createRes.json().id) {
                 basePvzId = createRes.json().id;
                 console.log(`Setup: Base PVZ created with ID (2nd try): ${basePvzId}`);
              } else {
                 throw new Error(`Setup: Failed to create base PVZ. Status: ${createRes.status}, Body: ${createRes.body}`);
              }
        }
    } catch(e) {
        console.error(e.message);
        throw e;
    }

    console.log('Setup finished.');
    return {
        moderatorToken: moderatorToken,
        employeeToken: employeeToken,
        basePvzId: basePvzId
    };
}

// --- Сценарий Теста (выполняется каждым VU) ---
export default function (data) {
    const employeeToken = data.employeeToken;
    const moderatorToken = data.moderatorToken;
    const basePvzId = data.basePvzId;

    if (!employeeToken || !moderatorToken || !basePvzId) {
        console.error(`VU ${__VU}: Missing data from setup. Skipping iteration.`);
        return;
    }

    let employeeHeaders = { headers: { 'Authorization': `Bearer ${employeeToken}`, 'Content-Type': 'application/json' } };
    let moderatorHeaders = { headers: { 'Authorization': `Bearer ${moderatorToken}`, 'Content-Type': 'application/json' } };

    let action = Math.random();

    if (action < 0.7) { // 70% - чтение списка ПВЗ
        group('List PVZs (Employee)', function () {
            let page = Math.floor(Math.random() * 5) + 1;
            let limit = 10;
            let res = http.get(`${BASE_URL}/pvz?page=${page}&limit=${limit}`, employeeHeaders);

            let success = check(res, {
                'List PVZ status is 200': (r) => r.status === 200,
                'List PVZ response is JSON': (r) => r.status === 200 && r.headers['Content-Type']?.includes('application/json'),
                'List PVZ has items array': (r) => {
                    if (r.status !== 200) return true;
                    try { return Array.isArray(r.json().items); } catch (e) { console.error(`List PVZ JSON parse error: ${e}, body: ${r.body}`); return false; }
                }
            });
            // Добавляем в errorRate только если статус не 200 (основная проблема)
            errorRate.add(res.status !== 200);
            pvzListLatency.add(res.timings.duration);
        });

    } else if (action < 0.8 && moderatorToken && (__VU % 10 === 1)) { // ~10% * ~10% VUs = 1% - создание ПВЗ (редко)
            group('Create PVZ (Moderator)', function () {
                let city = ["Москва", "Санкт-Петербург", "Казань"][Math.floor(Math.random()*3)];
                let payload = JSON.stringify({ city: city });
                let res = http.post(`${BASE_URL}/pvz`, payload, moderatorHeaders);

                let success = check(res, {
                    'Create PVZ status is 201': (r) => r.status === 201,
                    'Create PVZ returns ID': (r) => {
                        if (r.status !== 201) return true;
                        try { return typeof r.json().id === 'string' && r.json().id !== ''; } catch (e) { console.error(`Create PVZ JSON parse error: ${e}, body: ${r.body}`); return false;}
                    }
                });
                errorRate.add(res.status !== 201);
                createPvzLatency.add(res.timings.duration);
            });

    } else if (employeeToken) { // ~20% - рабочий флоу сотрудника
        let currentReceptionId = null;
        let receptionInitiated = false; // Флаг успешной инициации ЭТИМ VU в ЭТОЙ итерации

        group('Employee Workflow', function() {
            // --- ШАГ А: Инициировать приемку ---
            let initPayload = JSON.stringify({ pvzId: basePvzId });
            let initRes = http.post(`${BASE_URL}/receptions`, initPayload, employeeHeaders);
            let successInitCheck = check(initRes, {
                '[Employee] Initiate Reception status is 201 or 400': (r) => r.status === 201 || r.status === 400,
            });
            initiateReceptionLatency.add(initRes.timings.duration);

            let isExpectedConflict = false;
            if (initRes.status === 201) {
                try {
                    let respJson = initRes.json(); // Парсим JSON один раз
                    if (respJson && typeof respJson.id === 'string' && respJson.id !== '') {
                       currentReceptionId = respJson.id;
                       receptionInitiated = true;
                       // console.log(`VU ${__VU} iteration ${__ITER} initiated reception ${currentReceptionId} for PVZ ${basePvzId}`);
                    } else {
                        console.error(`VU ${__VU} iteration ${__ITER} received status 201 but ID is missing or invalid in response: ${initRes.body}`);
                        successInitCheck = false; // Отмечаем чек как проваленный
                    }
                } catch (e) {
                    console.error(`VU ${__VU} iteration ${__ITER} failed to parse reception ID from 201 response: ${e}, body: ${initRes.body}`);
                    receptionInitiated = false;
                    successInitCheck = false; // Отмечаем чек как проваленный
                }
            } else if (initRes.status === 400) {
                // Проверяем, ожидаемый ли это конфликт
                try {
                   if (initRes.body.includes("предыдущая приемка для этого ПВЗ еще не закрыта")) {
                      isExpectedConflict = true; // Это нормально, другой VU работает
                   } else {
                      console.warn(`VU ${__VU} iteration ${__ITER} unexpected 400 error initiating reception for PVZ ${basePvzId}. Body: ${initRes.body}`);
                   }
                } catch(e) {
                     console.warn(`VU ${__VU} iteration ${__ITER} failed to check 400 error body: ${e}, body: ${initRes.body}`);
                }
            } else {
                 // Неожиданная ошибка при инициации (не 201 и не 400)
                 console.warn(`VU ${__VU} iteration ${__ITER} unexpected status initiating reception for PVZ ${basePvzId}. Status: ${initRes.status}, Body: ${initRes.body}`);
            }
            // Добавляем ошибку, только если это не ожидаемый конфликт 400
            errorRate.add(!successInitCheck || (initRes.status !== 201 && !isExpectedConflict));

            // --- ШАГ Б: Добавить товар (только если ЭТОТ VU успешно инициировал приемку) ---
            if (receptionInitiated && currentReceptionId) {
                sleep(0.05 + Math.random() * 0.05); // Небольшая пауза
                let productType = ["электроника", "одежда", "обувь"][Math.floor(Math.random()*3)];
                let addPayload = JSON.stringify({ pvzId: basePvzId, type: productType });
                let addRes = http.post(`${BASE_URL}/products`, addPayload, employeeHeaders);

                let successAdd = check(addRes, {
                    '[Employee] Add Product status is 201': (r) => r.status === 201,
                });
                addProductLatency.add(addRes.timings.duration);
                if (!successAdd) {
                    console.warn(`VU ${__VU} iteration ${__ITER} failed to add product to PVZ ${basePvzId} (Reception ${currentReceptionId}). Status: ${addRes.status}, Body: ${addRes.body}`);
                }
                errorRate.add(!successAdd);

                // --- ШАГ В: Закрыть приемку (только если ЭТОТ VU успешно ее инициировал) ---
                sleep(0.05 + Math.random() * 0.05); // Небольшая пауза
                let closeRes = http.post(`${BASE_URL}/pvz/${basePvzId}/close_last_reception`, null, employeeHeaders);
                 let successClose = check(closeRes, {
                     '[Employee] Close Reception status is 200': (r) => r.status === 200,
                 });
                 closeReceptionLatency.add(closeRes.timings.duration);
                 if (!successClose) {
                     console.warn(`VU ${__VU} iteration ${__ITER} failed to close reception ${currentReceptionId} for PVZ ${basePvzId}. Status: ${closeRes.status}, Body: ${closeRes.body}`);
                 }
                 errorRate.add(!successClose);
            }
        });
    }

    // --- Общая пауза в конце итерации ---
    sleep(0.1 + Math.random() * 0.1); // Пауза 0.5 - 1 сек (можно уменьшить для увеличения RPS)
}

// --- Функция Teardown (опционально) ---
export function teardown(data) {
  console.log("Test finished.");
  // Можно добавить очистку, но будьте осторожны с параллельными тестами
}