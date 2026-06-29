FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/c2c-market-backend ./cmd/api

FROM alpine:3.22

RUN addgroup -S c2c && adduser -S c2c -G c2c
WORKDIR /app
COPY --from=build /out/c2c-market-backend /app/c2c-market-backend

USER c2c
EXPOSE 8080
ENTRYPOINT ["/app/c2c-market-backend"]
