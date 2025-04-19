# Используем официальный образ Go как базу
FROM golang:1.24-alpine AS builder

# Устанавливаем рабочую директорию внутри контейнера
WORKDIR /app

# Копируем файлы управления зависимостями
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код проекта
COPY . .

# Собираем приложение
# CGO_ENABLED=0 отключает Cgo, что позволяет создавать статические бинарники
# -o /pvz-service указывает имя выходного файла
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /pvz-service ./cmd/api

# Используем маленький базовый образ для финального контейнера
FROM alpine:latest

# Копируем собранный бинарник из builder'а
COPY --from=builder /pvz-service /pvz-service

# Указываем порт, который будет слушать сервис (согласно заданию)
EXPOSE 8080

# Команда для запуска сервиса при старте контейнера
ENTRYPOINT ["/pvz-service"]