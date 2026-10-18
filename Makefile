GO          := $(shell which go)
AIR         := $(shell which air)
SWAG        := $(shell which swag)

FILE_ENV    := ./.env

.PHONY: run
run:
	set -a; . $(FILE_ENV); set +a; $(AIR)

.PHONY: api
api:
	set -a; . $(FILE_ENV); set +a; $(GO) run ./cmd/server

# +-----------+
# | sub tasks |
# +-----------+
.PHONY: swagger
swagger:
	$(SWAG) fmt
	$(SWAG) init --parseDependency --dir ./cmd/server,internal --output ./docs/swagger

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: mockgen
mockgen:
	$(GO) generate ./...

.PHONY: test
test: vet mockgen
	$(GO) test -v -count=1 -failfast ./...

.PHONY: coverage
coverage:
	$(GO) test ./... -coverprofile=coverage.out && $(GO) tool cover -html=coverage.out

# +------------+
# | migrations |
# +------------+
GOOSE       := $(shell which goose)
MIGRATE_DIR := ./db/migrations
GOOSE_DSN   := postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSLMODE

.PHONY: migrate
migrate:
	set -a; . $(FILE_ENV); set +a; $(GOOSE) -dir $(MIGRATE_DIR) postgres "$(GOOSE_DSN)" up

.PHONY: migrate-down
migrate-down:
	set -a; . $(FILE_ENV); set +a; $(GOOSE) -dir $(MIGRATE_DIR) postgres "$(GOOSE_DSN)" down

.PHONY: migrate-status
migrate-status:
	set -a; . $(FILE_ENV); set +a; $(GOOSE) -dir $(MIGRATE_DIR) postgres "$(GOOSE_DSN)" status
