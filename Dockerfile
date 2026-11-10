### Билд
FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
#Билдим бинарник для Linux без зависимостей от Си-либ и без отладочной информации
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/server ./cmd/api


### Рантайм
FROM alpine:3.21

# Устанавливаенм сертификаты и Time Zone data для Alpine Linux
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /bin/server /app/server
COPY migrations/ /app/migrations/

EXPOSE 8080

ENTRYPOINT ["/app/server"]
