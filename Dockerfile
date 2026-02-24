# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Установка зависимостей для сборки
RUN apk add --no-cache git

# Копирование go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка приложения
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/api ./cmd/api

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Установка ca-certificates для HTTPS запросов
RUN apk --no-cache add ca-certificates tzdata

# Копирование бинарника из builder stage
COPY --from=builder /app/api .

# Создание непривилегированного пользователя
RUN adduser -D -g '' appuser
USER appuser

# Порт приложения
EXPOSE 8080

# Запуск приложения
CMD ["./api"]