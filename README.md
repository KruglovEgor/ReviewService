# Review Service

Микросервис для автоматического назначения ревьюеров на Pull Request'ы.

## Описание

Сервис автоматически назначает до 2 ревьюеров из команды автора PR, поддерживает переназначение ревьюеров и управление командами.

**Основной функционал:**
- Автоматическое назначение ревьюеров из команды автора
- Переназначение ревьюеров с учётом активности
- Управление командами и пользователями
- Отслеживание статусов PR (OPEN/MERGED)
- Идемпотентные операции слияния
- Статистика по PR и пользователям

## Быстрый старт

### Запуск через Docker Compose

```bash
docker-compose up -d
```

Сервис будет доступен на `http://localhost:8080`

**Swagger UI:** `http://localhost:8080/swagger/`

### Остановка

```bash
docker-compose down
```

## Архитектура

Проект следует принципам Clean Architecture:

```
internal/
├── domain/          # Доменные модели и интерфейсы репозиториев
├── repository/      # Реализация работы с PostgreSQL
├── service/         # Бизнес-логика
├── handler/         # HTTP обработчики
├── config/          # Конфигурация
└── testutil/        # Утилиты для тестирования
```

**Слои:**
- **Domain** - модели данных, бизнес-правила, интерфейсы
- **Repository** - работа с БД через pgx/v5
- **Service** - бизнес-логика (назначение ревьюеров, валидация)
- **Handler** - HTTP API через chi/v5

## API

Полная документация доступна в Swagger UI по адресу `/swagger/`

### Основные эндпоинты

**Команды:**
- `POST /team/add` - создать команду
- `GET /team/get?team_name={name}` - получить команду
- `POST /team/deactivate` - массово деактивировать команду

**Пользователи:**
- `POST /users/setIsActive` - изменить статус активности
- `GET /users/getReview?user_id={id}` - получить PR пользователя

**Pull Requests:**
- `POST /pullRequest/create` - создать PR (автоназначение ревьюеров)
- `POST /pullRequest/merge` - слияние PR (идемпотентно)
- `POST /pullRequest/reassign` - переназначить ревьювера
- `GET /pullRequest/list?status={OPEN|MERGED}` - список PR

**Статистика:**
- `GET /stats` - общая статистика сервиса


## База данных

**Схема:**
```sql
teams           - команды
users           - пользователи (связь с командой)
pull_requests   - PR'ы
pr_reviewers    - связь PR-ревьювер
```

**Индексы** добавлены для оптимизации запросов:
- `users(team_name, is_active)` - поиск активных участников команды
- `pr_reviewers(user_id)` - поиск PR'ов пользователя
- `pull_requests(status)` - фильтрация по статусу

Миграции применяются автоматически при запуске приложения.

## Конфигурация

Сервис настраивается через переменные окружения. **Все значения имеют defaults** - файл `.env` **не обязателен** для запуска.

**Docker Compose использует defaults**, указанные в формате `${VAR:-default}`. Это значит:
- Если переменная не задана → используется default значение
- Для изменения настроек можно создать `.env` файл (см. `.env.example`)

**Основные переменные:**
```bash
# База данных
DB_HOST=postgres
DB_PORT=5432
DB_USER=reviewservice
DB_PASSWORD=password
DB_NAME=reviewservice

# Сервер
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Приложение
LOG_LEVEL=info
APP_ENV=development
```

**Создание .env файла (опционально):**
```bash
# Скопируйте шаблон
cp .env.example .env

# Отредактируйте значения при необходимости
# Файл .env в .gitignore и не будет закоммичен
```

Полный список переменных с defaults смотрите в `docker-compose.yml`

## Тестирование

### Unit-тесты

```bash
go test -v ./internal/...
```

**Покрытие кода:**
```bash
go test ./internal/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Текущее покрытие:**
- Service layer: 67.7%
- Handler layer: 48.8%
- Всего: 98 тестов

### Integration-тесты

**С использованием Docker (рекомендуется):**
```bash
# Запускает отдельную тестовую БД и выполняет e2e тесты
docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit

# Очистка после тестов
docker-compose -f docker-compose.test.yml down -v
```

**Локально (требуется запущенная PostgreSQL):**
```bash
# Настройте переменные окружения для тестовой БД
export TEST_DB_HOST=localhost
export TEST_DB_NAME=reviewservice_test
# ... остальные переменные

go test -v ./tests/integration/...
```

## Локальная разработка

Если нужно запустить без Docker:

```bash
# 1. Установить зависимости
go mod download

# 2. Запустить PostgreSQL
docker run -d --name postgres \
  -e POSTGRES_USER=reviewservice \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=reviewservice \
  -p 5432:5432 \
  postgres:16-alpine

# 3. Настроить окружение
export DB_HOST=localhost
export DB_PORT=5432
# ... остальные переменные

# 4. Запустить приложение
go run ./cmd/api
```

## Принятые решения

### 1. Выбор ревьюеров
Используется алгоритм Fisher-Yates shuffle для честного случайного выбора из активных участников команды.

### 2. Идемпотентность
Повторный вызов `POST /pullRequest/merge` для уже слитого PR возвращает 200 OK с текущим состоянием.

### 3. Переназначение
Новый ревьювер выбирается из **команды заменяемого ревьювера**, а не автора PR. Это позволяет поддерживать кросс-командное ревью.

### 4. Неактивные пользователи
Пользователи с `is_active = false`:
- Не назначаются на новые PR
- Не участвуют в переназначении
- Остаются в существующих PR до явного переназначения

### 5. Транзакции
Критичные операции выполняются в транзакциях для консистентности данных:
- **CreateTeam** - атомарное создание команды + множественное создание/обновление пользователей
- **AssignReviewers** - атомарное назначение нескольких ревьюеров
- **ReassignReviewer** - атомарная замена ревьювера

**BulkDeactivateTeam:**
- Деактивация пользователей команды выполняется атомарно (один SQL запрос)
- Переназначение открытых PR происходит последовательно (best-effort подход)
- Частичные ошибки переназначения логируются и возвращаются в результате

### 6. Graceful Shutdown
Сервер корректно завершает активные соединения при получении SIGTERM/SIGINT (30 сек таймаут).

## Дополнительные задания

### ✅ Статистика
`GET /stats` возвращает:
- Общую статистику PR (total, open, merged, среднее число ревьюеров)
- Статистику по каждому пользователю

### ✅ Массовая деактивация
`POST /team/deactivate`:
- Деактивирует всех членов команды
- Автоматически переназначает их открытые PR
- Возвращает детальный отчёт

### ✅ Integration тесты
E2E тесты с реальной PostgreSQL в `tests/integration/`:
- Полный жизненный цикл (команда → PR → merge → статистика)
- Массовая деактивация с переназначением

### ✅ Линтер
Настроен `golangci-lint` с 15+ линтерами (см. `.golangci.yml`)

## Технологии

- **Go 1.23**
- **PostgreSQL 16** + pgx/v5
- **chi/v5** - HTTP router
- **zap** - структурированное логирование
- **golang-migrate** - миграции БД
- **Docker & Docker Compose**

## Производительность

- **RPS**: 5+ (согласно требованиям)
- **Response Time**: <300ms (p95)
- **Connection Pool**: 25 max open, 5 max idle
- **Timeouts**: Read 10s, Write 10s, Idle 60s

## Структура проекта

```
.
├── cmd/api/              # Точка входа
├── internal/             # Приватный код приложения
│   ├── domain/          # Модели и интерфейсы
│   ├── repository/      # Работа с БД
│   ├── service/         # Бизнес-логика
│   ├── handler/         # HTTP handlers
│   ├── config/          # Конфигурация
│   └── testutil/        # Тестовые утилиты
├── migrations/           # SQL миграции
├── tests/integration/    # E2E тесты
├── docker-compose.yml   # Docker окружение
├── Dockerfile           # Production образ
├── .golangci.yml        # Конфигурация линтера
└── openapi.yml          # OpenAPI спецификация
```