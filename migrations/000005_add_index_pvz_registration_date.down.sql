-- Удаляем индекс для сортировки/пагинации по дате регистрации ПВЗ
DROP INDEX CONCURRENTLY IF EXISTS idx_pvz_registration_date;