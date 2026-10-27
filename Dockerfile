# syntax=docker/dockerfile:1

# ---- build ----------------------------------------------------------------
FROM golang:1.26-alpine AS build

WORKDIR /src

# Manifests first, so editing source does not re-download the module graph.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

# CGO_ENABLED=0 is what makes the empty runtime image below possible. GOARCH is
# inherited from --platform, so build with --platform linux/amd64 for GKE.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /server ./cmd/server

# ---- runtime --------------------------------------------------------------
# distroless static: no shell, no package manager, no libc. The :nonroot tag
# already sets USER to the numeric 65532 — do not restate it as a name, or
# Kubernetes runAsNonRoot can no longer verify the user is not root.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=build /server /server

EXPOSE 9999

ENTRYPOINT ["/server"]
