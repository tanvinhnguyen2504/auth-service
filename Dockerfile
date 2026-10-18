# syntax=docker/dockerfile:1

# ---- build ----------------------------------------------------------------
FROM golang:1.25-alpine AS build

WORKDIR /src

# Manifests first: this layer is cached and only rebuilt when dependencies
# change, so editing Go source does not re-download the module graph.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

# CGO_ENABLED=0 produces a fully static binary, which is what lets the runtime
# stage below be an empty image. -trimpath and -s -w drop build paths and the
# symbol table for a smaller, reproducible binary.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags='-s -w' \
      -o /out/server ./cmd/server

# ---- runtime --------------------------------------------------------------
# distroless static: no shell, no package manager, no libc. Runs as nonroot.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# The binary talks to Anthropic over TLS, so it needs the CA bundle; distroless
# ships one. Time zone data comes from the tzdata layer in the same image.
COPY --from=build /out/server /server

USER nonroot:nonroot
EXPOSE 4000

ENTRYPOINT ["/server"]
