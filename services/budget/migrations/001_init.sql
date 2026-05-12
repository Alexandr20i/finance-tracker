CREATE TABLE IF NOT EXISTS budgets (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL,
    category     VARCHAR(100) NOT NULL,
    limit_amount NUMERIC(12,2) NOT NULL CHECK (limit_amount > 0),
    spent_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    period       VARCHAR(10) NOT NULL DEFAULT 'monthly'
                 CHECK (period IN ('monthly', 'weekly', 'yearly')),
    created_at   TIMESTAMPTZ DEFAULT NOW(),

    -- один бюджет на категорию в рамках одного периода
    UNIQUE(user_id, category, period)
);

CREATE INDEX IF NOT EXISTS idx_budgets_user_id  ON budgets(user_id);
CREATE INDEX IF NOT EXISTS idx_budgets_category ON budgets(category);