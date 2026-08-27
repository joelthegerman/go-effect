-- 0001_init: the app's real schema (the todos table).
--
-- This is distinct from db/init.sql, which seeds a throwaway `users` table for
-- pgAdmin. This migration is owned by the app and applied by the migration
-- runner in internal/shell/migrate.go on startup.

CREATE TABLE IF NOT EXISTS todos (
    id         UUID        PRIMARY KEY,
    title      TEXT        NOT NULL CHECK (length(btrim(title)) > 0),
    done       BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS todos_created_at_idx ON todos (created_at DESC, id);
