# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Установка зависимостей для сборки
RUN apk add --no-cache git

# Копируем всё содержимое
COPY . .

# Скачиваем зависимости и создаём go.sum
RUN go mod download && go mod verify

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/api

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Установка CA сертификатов для HTTPS
RUN apk --no-cache add ca-certificates

# Копируем бинарник из build stage
COPY --from=builder /app/main .

# Копируем миграции
COPY migrations ./migrations

# Копируем OpenAPI спецификацию для Swagger
COPY openapi.yml .

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./main"]
