CREATE TYPE reception_status AS ENUM ('in_progress', 'closed');

CREATE TABLE IF NOT EXISTS receptions (
    id UUID PRIMARY KEY,
    pvz_id UUID NOT NULL REFERENCES pvz(id) ON DELETE CASCADE, -- Ссылка на ПВЗ
    date_time TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- Время начала приемки
    status reception_status NOT NULL DEFAULT 'in_progress' -- Статус (in_progress, closed)
);

-- Индекс для быстрого поиска последней открытой приемки для ПВЗ
CREATE INDEX IF NOT EXISTS idx_receptions_pvz_status ON receptions (pvz_id, status) WHERE status = 'in_progress';