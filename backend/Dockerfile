FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build

ARG APP_VERSION=development
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=1970-01-01T00:00:00Z

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN test -n "${APP_VERSION}" \
    && test -n "${GIT_COMMIT}" \
    && test -n "${BUILD_TIME}" \
    && CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w \
      -X 'c2c-market/backend/internal/buildinfo.Version=${APP_VERSION}' \
      -X 'c2c-market/backend/internal/buildinfo.GitCommit=${GIT_COMMIT}' \
      -X 'c2c-market/backend/internal/buildinfo.BuildTime=${BUILD_TIME}'" \
    -o /out/c2c-market-backend \
    ./cmd/api

FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce

ARG APP_VERSION=development
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=1970-01-01T00:00:00Z

LABEL org.opencontainers.image.title="C2CMarket backend" \
      org.opencontainers.image.source="https://github.com/xiangrikuil/c2cmarket" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}"

RUN apk add --no-cache tzdata \
    && addgroup -S c2c \
    && adduser -S c2c -G c2c
WORKDIR /app
COPY --from=build /out/c2c-market-backend /app/c2c-market-backend

USER c2c
EXPOSE 8080
ENTRYPOINT ["/app/c2c-market-backend"]
