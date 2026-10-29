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

# +-------------------+
# | image / registry  |
# +-------------------+
# --platform linux/amd64 is required, not optional: this machine is arm64 and
# the GKE nodes are e2-medium (amd64). A natively-built image would fail on the
# node with "exec format error".
DOCKER    := $(shell which docker)
AR_HOST   := asia-southeast1-docker.pkg.dev
AR_IMAGE  := $(AR_HOST)/vincent-mini-apps-project/auth-repository/auth-service
TAG       ?= $(shell git rev-parse --short HEAD)

.PHONY: ar-login
ar-login:
	gcloud auth configure-docker $(AR_HOST)

# Tagged by commit SHA, never :latest — a mutable tag makes a rollout
# non-deterministic and `kubectl rollout undo` meaningless.
.PHONY: image-push
image-push:
	$(DOCKER) buildx build --platform linux/amd64 --target runtime \
	  -t $(AR_IMAGE):$(TAG) --push .

# Same build, kept local, to inspect before pushing.
.PHONY: image
image:
	$(DOCKER) buildx build --platform linux/amd64 --target runtime \
	  -t $(AR_IMAGE):$(TAG) --load .
