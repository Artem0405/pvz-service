-- Добавляем индекс для ускорения сортировки/пагинации по дате регистрации ПВЗ
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_pvz_registration_date ON pvz (registration_date DESC);