package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/KruglovEgor/ReviewService/internal/config"
	"github.com/KruglovEgor/ReviewService/internal/domain"
	"github.com/KruglovEgor/ReviewService/internal/handler"
	"github.com/KruglovEgor/ReviewService/internal/repository/postgres"
	"github.com/KruglovEgor/ReviewService/internal/service"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

// TestE2EWorkflow проверяет полный сценарий работы с PR
func TestE2EWorkflow(t *testing.T) {
	// Пропускаем если нет переменной окружения для integration тестов
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Настройка тестовой БД
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Создаём тестовый сервер
	router := setupTestServer(db, t)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Сценарий: создаём команду, PR, деактивируем пользователя, проверяем статистику

	// 1. Создаём команду
	team := domain.Team{
		TeamName: "backend",
		Members: []domain.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: true},
			{UserID: "u3", Username: "Charlie", IsActive: true},
		},
	}

	resp := makeRequest(t, ts, "POST", "/team/add", team)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, readBody(t, resp))
	}

	// 2. Создаём PR от Alice
	prReq := map[string]string{
		"pull_request_id":   "pr-1",
		"pull_request_name": "Add feature",
		"author_id":         "u1",
	}

	resp = makeRequest(t, ts, "POST", "/pullRequest/create", prReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, readBody(t, resp))
	}

	var prResp struct {
		PR domain.PullRequest `json:"pr"`
	}
	decodeJSON(t, resp, &prResp)

	// Проверяем что назначено до 2 ревьюверов
	if len(prResp.PR.AssignedReviewers) == 0 || len(prResp.PR.AssignedReviewers) > 2 {
		t.Errorf("expected 1-2 reviewers, got %d", len(prResp.PR.AssignedReviewers))
	}

	// 3. Получаем отзывы пользователя Bob (если он назначен)
	resp = makeRequest(t, ts, "GET", "/users/getReview?user_id=u2", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, readBody(t, resp))
	}

	// 4. Мёржим PR
	mergeReq := map[string]string{"pull_request_id": "pr-1"}
	resp = makeRequest(t, ts, "POST", "/pullRequest/merge", mergeReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, readBody(t, resp))
	}

	// 5. Получаем статистику
	resp = makeRequest(t, ts, "GET", "/stats", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, readBody(t, resp))
	}

	var stats service.GlobalStats
	decodeJSON(t, resp, &stats)

	if stats.PRStats.TotalPRs != 1 {
		t.Errorf("expected 1 PR, got %d", stats.PRStats.TotalPRs)
	}
	if stats.PRStats.MergedPRs != 1 {
		t.Errorf("expected 1 merged PR, got %d", stats.PRStats.MergedPRs)
	}
}

// TestBulkDeactivation проверяет массовую деактивацию
func TestBulkDeactivation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()

	router := setupTestServer(db, t)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Создаём команду с несколькими пользователями
	team := domain.Team{
		TeamName: "payments",
		Members: []domain.TeamMember{
			{UserID: "p1", Username: "Pay1", IsActive: true},
			{UserID: "p2", Username: "Pay2", IsActive: true},
			{UserID: "p3", Username: "Pay3", IsActive: true},
			{UserID: "p4", Username: "Pay4", IsActive: true},
		},
	}

	makeRequest(t, ts, "POST", "/team/add", team)

	// Создаём несколько PR
	for i := 1; i <= 3; i++ {
		prReq := map[string]string{
			"pull_request_id":   fmt.Sprintf("pr-p%d", i),
			"pull_request_name": fmt.Sprintf("Payment PR %d", i),
			"author_id":         "p1",
		}
		makeRequest(t, ts, "POST", "/pullRequest/create", prReq)
	}

	// Деактивируем всю команду
	deactivateReq := map[string]string{"team_name": "payments"}
	resp := makeRequest(t, ts, "POST", "/team/deactivate", deactivateReq)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, readBody(t, resp))
	}

	var result service.BulkDeactivateResult
	decodeJSON(t, resp, &result)

	if len(result.DeactivatedUsers) != 4 {
		t.Errorf("expected 4 deactivated users, got %d", len(result.DeactivatedUsers))
	}

	t.Logf("Bulk deactivation: %d users, %d PRs reassigned, %d errors",
		len(result.DeactivatedUsers), result.ReassignedPRs, result.Errors)
}

// setupTestDB создаёт тестовую БД и применяет миграции
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	// Используем переменные окружения или дефолтные значения
	cfg := config.DatabaseConfig{
		Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
		Port:     5432,
		User:     getEnvOrDefault("TEST_DB_USER", "reviewservice"),
		Password: getEnvOrDefault("TEST_DB_PASSWORD", "password"),
		Name:     getEnvOrDefault("TEST_DB_NAME", "reviewservice_test"),
		SSLMode:  "disable",
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, cfg.SSLMode)

	// Применяем миграции
	migrationsPath := "file://../../migrations"
	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		t.Fatalf("failed to create migrate: %v", err)
	}

	// Сбрасываем и применяем миграции
	m.Down()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to run migrations: %v", err)
	}
	m.Close()

	// Подключаемся к БД
	db, err := postgres.NewDB(postgres.Config{
		DSN:             dsn,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 0,
	})
	if err != nil {
		t.Fatalf("failed to connect to test DB: %v", err)
	}

	cleanup := func() {
		// Очищаем данные после теста
		db.ExecContext(context.Background(), "TRUNCATE teams, users, pull_requests, pr_reviewers CASCADE")
		db.Close()
	}

	return db, cleanup
}

// setupTestServer создаёт тестовый HTTP сервер
func setupTestServer(db *sql.DB, t *testing.T) http.Handler {
	t.Helper()

	logger, _ := zap.NewDevelopment()

	// Transaction Manager
	txManager := postgres.NewTxManager(db)

	// Repositories
	teamRepo := postgres.NewTeamRepository(db)
	userRepo := postgres.NewUserRepository(db)
	prRepo := postgres.NewPullRequestRepository(db)

	// Services
	teamService := service.NewTeamService(teamRepo, userRepo, txManager, logger)
	userService := service.NewUserService(userRepo, logger)
	prService := service.NewPullRequestService(prRepo, userRepo, logger)
	statsService := service.NewStatsService(prRepo, userRepo, logger)

	// Handlers
	teamHandler := handler.NewTeamHandler(teamService, statsService, logger)
	userHandler := handler.NewUserHandler(userService, prService, logger)
	prHandler := handler.NewPullRequestHandler(prService, logger)
	statsHandler := handler.NewStatsHandler(statsService, logger)

	return handler.Router(teamHandler, userHandler, prHandler, statsHandler, logger)
}

// makeRequest выполняет HTTP запрос к тестовому серверу
func makeRequest(t *testing.T, ts *httptest.Server, method, path string, body interface{}) *http.Response {
	t.Helper()

	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request: %v", err)
		}
	}

	req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}

	return resp
}

// readBody читает тело ответа
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return buf.String()
}

// decodeJSON декодирует JSON ответ
func decodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()

	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// getEnvOrDefault возвращает значение переменной окружения или дефолт
func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
