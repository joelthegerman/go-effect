.PHONY: build run migrate test test-int lint check db-up db-down db-reset

build:
	go build ./...

# Runs the Todos API on :8080 (override with PORT=9090 make run). Needs Postgres
# reachable via DATABASE_URL — run `make db-up` first. Migrations run on startup.
run:
	go run ./cmd/server

# Apply database migrations and exit (also run automatically on `make run`).
migrate:
	go run ./cmd/server -migrate

test:
	go test ./...

# Integration tests hit a real database. They SKIP automatically when none is
# reachable; `make db-up` first to actually exercise them.
test-int:
	go test ./internal/shell/ -run TestRepo -v

# Enforces core purity via depguard (belt-and-suspenders; `test` already
# guarantees it via TestCoreIsPure). Requires golangci-lint on PATH.
lint:
	golangci-lint run

check: test lint

# --- Postgres (Postgres on :5432, pgAdmin on :8081) --------------------------
# The app now uses this database via a repository behind the effect gate (see
# docs/adr/0002-*). The app's schema is created by migrations in
# internal/shell/migrations; db/init.sql only seeds a throwaway pgAdmin demo.

db-up:
	docker compose up -d

db-down:
	docker compose down

# Wipes the data + pgadmin volumes, so the next db-up re-runs db/init.sql and
# the app re-runs its migrations on next start.
db-reset:
	docker compose down -v
