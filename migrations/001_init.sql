-- Users
CREATE TABLE IF NOT EXISTS users (
  id            BIGSERIAL PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Recipe content lives in Markdown files under recipes/ (see internal/recipes),
-- not in the database, so anyone can contribute a recipe with a GitHub PR.
-- Only per-user state (favorites, meal plans) is stored here, keyed by the
-- recipe's slug (its filename, e.g. "tomato-basil-pasta").

-- Favorites
CREATE TABLE IF NOT EXISTS user_favorites (
  user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipe_slug TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, recipe_slug)
);

-- Meal plans
CREATE TABLE IF NOT EXISTS meal_plans (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  week_start  DATE NOT NULL,
  day_of_week SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6), -- 0=Sun
  slot        TEXT NOT NULL CHECK (slot IN ('breakfast','lunch','dinner')),
  recipe_slug TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, week_start, day_of_week, slot)
);

CREATE INDEX IF NOT EXISTS idx_meal_plans_user_week ON meal_plans (user_id, week_start);
