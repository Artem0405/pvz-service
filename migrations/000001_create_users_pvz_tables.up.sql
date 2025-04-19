-- Создаем таблицу пользователей
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY, -- UUID будет генерироваться приложением
    email VARCHAR(255) UNIQUE, -- Пока необязательное, но пригодится для полной авторизации
    password_hash VARCHAR(255), -- Тоже для полной авторизации
    role VARCHAR(50) NOT NULL CHECK (role IN ('employee', 'moderator')) -- Роли согласно заданию
    -- created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- Можно добавить для аудита
    -- updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Создаем таблицу ПВЗ
CREATE TABLE IF NOT EXISTS pvz (
    id UUID PRIMARY KEY, -- UUID будет генерироваться приложением
    registration_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    city VARCHAR(100) NOT NULL CHECK (city IN ('Москва', 'Санкт-Петербург', 'Казань')) -- Города согласно заданию
    -- name VARCHAR(255), -- Можно добавить имя/адрес ПВЗ
    -- address TEXT
);

-- Можно добавить индексы, если ожидаются частые поиски по определенным полям
-- CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);