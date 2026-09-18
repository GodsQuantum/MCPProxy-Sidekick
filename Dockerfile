# syntax=docker/dockerfile:1.7
FROM golang:1.27.1-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sidekick ./cmd/sidekick

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/sidekick /sidekick
USER 65532:65532
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD ["/sidekick","healthcheck"]
ENTRYPOINT ["/sidekick"]
