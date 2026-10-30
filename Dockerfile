FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

# CGO_ENABLED=0 is what makes the empty runtime image below possible. GOARCH is
# inherited from --platform, so build with --platform linux/amd64 for GKE.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=build /server /server

EXPOSE 9999

ENTRYPOINT ["/server"]
