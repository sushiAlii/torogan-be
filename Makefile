ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Internal Docker network connection URL
DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@torogan-postgres:5432/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.PHONY: up down migrate-up migrate-down migrate-create

# --------------------------------------------------------------------
# Core Infrastructure Controls
# --------------------------------------------------------------------

# Spin up infrastructure and instantly execute pending migrations
up:
	@echo "🚀 Spinning up Torogan infrastructure..."
	@docker compose up -d --build
	@echo "⏳ Waiting 3 seconds for database layers to settle..."
	@sleep 3
	@echo "🔄 Running migrations..."
	@$(MAKE) migrate-up
	@echo "✅ Torogan backend is fully live and up-to-date!"

# Cleanly stop and dismantle all active local containers
down:
	@echo "🛑 Stopping infrastructure..."
	@docker compose down

# --------------------------------------------------------------------
# Granular Database Migration Utilities
# --------------------------------------------------------------------

# Run all pending upward migrations manually
migrate-up:
	@docker run --rm --network torogan-network -v $(shell pwd)/internal/database/migrations:/migrations migrate/migrate:v4.17.1 \
		-path=/migrations -database '$(DB_URL)' up

# Rollback the last applied migration step
migrate-down:
	@docker run --rm --network torogan-network -v $(shell pwd)/internal/database/migrations:/migrations migrate/migrate:v4.17.1 \
		-path=/migrations -database '$(DB_URL)' down 1

# Generate a brand new sequential migration blueprint pair
migrate-create:
	@docker run --rm -v $(shell pwd)/internal/database/migrations:/migrations migrate/migrate:v4.17.1 \
		create -ext sql -dir /migrations -seq $(name)
