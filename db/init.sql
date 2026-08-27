-- Throwaway seed data for pgAdmin — a DEMO, not the app's schema.
--
-- The app's real schema (the `todos` table) is created by the migrations in
-- internal/shell/migrations, applied on startup. This `users` table is
-- ILLUSTRATIVE ONLY: it exists so opening pgAdmin shows a table with rows. The
-- app never reads or writes it, and nothing in internal/core knows it exists.
--
-- Auto-run by the postgres image exactly once, on first boot of an empty data
-- directory. To re-run it, wipe the volume: `make db-reset`.

CREATE TABLE IF NOT EXISTS users (
    id    SERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE
);

INSERT INTO users (email) VALUES
    ('ada@example.com'),
    ('grace@example.com')
ON CONFLICT (email) DO NOTHING;
