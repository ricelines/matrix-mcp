# syntax=docker/dockerfile:1.10

ARG GO_VERSION=1.26.1

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-trixie AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

ENV CGO_ENABLED=0 \
    GOMODCACHE=/go/pkg/mod \
    GOCACHE=/root/.cache/go-build

COPY --link go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    go mod download

COPY --link cmd ./cmd
COPY --link internal ./internal

RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build \
        -buildvcs=false \
        -trimpath \
        -ldflags='-s -w' \
        -o /out/matrix-mcp-server \
        ./cmd/matrix-mcp-server

FROM gcr.io/distroless/static-debian13:nonroot AS runtime

WORKDIR /app

COPY --from=build --chown=65532:65532 /out/matrix-mcp-server /app/matrix-mcp-server

EXPOSE 8080

ENTRYPOINT ["/app/matrix-mcp-server"]
