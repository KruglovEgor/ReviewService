.PHONY: help build run test test-unit test-integration test-coverage clean docker-up docker-down lint

# Переменные
APP_NAME=reviewservice
DOCKER_COMPOSE=docker-compose
GO=go

help: ## Показать эту справку
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Собрать приложение
	$(GO) build -o bin/$(APP_NAME) ./cmd/api

run: ## Запустить приложение локально
	$(GO) run ./cmd/api

test: test-unit ## Запустить все unit-тесты

test-unit: ## Запустить unit-тесты
	@echo "Running unit tests..."
	$(GO) test -v -race ./tests/unit/...

test-integration: ## Запустить integration-тесты (требует PostgreSQL)
	@echo "Running integration tests..."
	$(GO) test -v ./tests/integration/...

test-coverage: ## Запустить тесты с покрытием кода
	@echo "Running tests with coverage..."
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=coverage.out
	@echo "\nHTML report: coverage.html"
	$(GO) tool cover -html=coverage.out -o coverage.html

test-all: test-unit test-integration ## Запустить все тесты

clean: ## Удалить сгенерированные файлы
	rm -rf bin/
	rm -f coverage.out coverage.html

docker-up: ## Запустить сервис в Docker
	$(DOCKER_COMPOSE) up -d

docker-down: ## Остановить сервис в Docker
	$(DOCKER_COMPOSE) down

docker-logs: ## Показать логи Docker контейнеров
	$(DOCKER_COMPOSE) logs -f

docker-rebuild: ## Пересобрать и запустить Docker контейнеры
	$(DOCKER_COMPOSE) up -d --build

lint: ## Запустить линтер
	golangci-lint run

format: ## Форматировать код
	gofmt -s -w .
	goimports -w .

.DEFAULT_GOAL := help
