-- Определяем допустимые типы товаров
CREATE TYPE product_type AS ENUM ('электроника', 'одежда', 'обувь');

CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY,
    reception_id UUID NOT NULL REFERENCES receptions(id) ON DELETE CASCADE, -- Ссылка на приемку
    date_time_added TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- Время добавления товара
    type product_type NOT NULL -- Тип товара
    -- Можно добавить другие поля товара, если нужно (например, name, description, order_id)
);

-- Индекс для возможного поиска товаров по приемке
CREATE INDEX IF NOT EXISTS idx_products_reception_id ON products (reception_id);
-- Индекс для LIFO удаления (по времени добавления)
CREATE INDEX IF NOT EXISTS idx_products_reception_time ON products (reception_id, date_time_added);