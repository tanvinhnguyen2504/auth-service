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

# +-------------------+
# | image / registry  |
# +-------------------+
# --platform linux/amd64 is required, not optional: this machine is arm64 and
# the GKE nodes are e2-medium (amd64). A natively-built image would fail on the
# node with "exec format error".
DOCKER    := $(shell which docker)
AR_HOST   := asia-southeast1-docker.pkg.dev
AR_IMAGE  := $(AR_HOST)/vincent-mini-apps-project/recommendations-repository/recommendations-service
TAG       ?= $(shell git rev-parse --short HEAD)

.PHONY: ar-login
ar-login:
	gcloud auth configure-docker $(AR_HOST)

.PHONY: image-push
image-push:
	$(DOCKER) buildx build --platform linux/amd64 --target runtime \
	  -t $(AR_IMAGE):$(TAG) --push .

.PHONY: image
image:
	$(DOCKER) buildx build --platform linux/amd64 --target runtime \
	  -t $(AR_IMAGE):$(TAG) --load .
