package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"reviewservice/internal/config"
	"reviewservice/internal/handler"
	"reviewservice/internal/repository/postgres"
	"reviewservice/internal/service"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "application error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Инициализация логгера
	logger, err := initLogger(cfg.App.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("starting application",
		zap.String("env", cfg.App.Env),
		zap.String("log_level", cfg.App.LogLevel))

	// Запуск миграций
	if err := runMigrations(cfg.Database, logger); err != nil {
		logger.Error("failed to run migrations", zap.Error(err))
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Подключение к БД
	db, err := postgres.NewDB(postgres.Config{
		DSN:             cfg.Database.DSN(),
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		logger.Error("failed to connect to database", zap.Error(err))
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	logger.Info("connected to database")

	// Инициализация зависимостей
	app := initApp(db, logger)

	// Создание HTTP сервера
	srv := &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      app.router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Graceful shutdown
	serverErrors := make(chan error, 1)

	go func() {
		logger.Info("starting HTTP server", zap.String("address", srv.Addr))
		serverErrors <- srv.ListenAndServe()
	}()

	// Ожидание сигнала завершения
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		logger.Info("received shutdown signal", zap.String("signal", sig.String()))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", zap.Error(err))
			if err := srv.Close(); err != nil {
				return fmt.Errorf("could not stop server gracefully: %w", err)
			}
		}

		logger.Info("server stopped gracefully")
	}

	return nil
}

// App содержит все зависимости приложения
type App struct {
	router http.Handler
}

// initApp инициализирует приложение
func initApp(db *sql.DB, logger *zap.Logger) *App {
	// Transaction Manager
	txManager := postgres.NewTxManager(db)

	// Repositories
	teamRepo := postgres.NewTeamRepository(db)
	userRepo := postgres.NewUserRepository(db)
	prRepo := postgres.NewPullRequestRepository(db)

	// Services
	teamService := service.NewTeamService(teamRepo, userRepo, txManager, logger)
	userService := service.NewUserService(userRepo, prRepo, logger)
	prService := service.NewPullRequestService(prRepo, userRepo, logger)
	statsService := service.NewStatsService(prRepo, userRepo, logger)

	// Handlers
	teamHandler := handler.NewTeamHandler(teamService, statsService, logger)
	userHandler := handler.NewUserHandler(userService, prService, logger)
	prHandler := handler.NewPullRequestHandler(prService, logger)
	statsHandler := handler.NewStatsHandler(statsService, logger)

	// Router
	router := handler.Router(teamHandler, userHandler, prHandler, statsHandler, logger)

	return &App{
		router: router,
	}
}

// initLogger инициализирует структурированный логгер
func initLogger(level string) (*zap.Logger, error) {
	var zapConfig zap.Config

	if level == "debug" {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	// Парсим уровень логирования
	if err := zapConfig.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger, nil
}

// runMigrations выполняет миграции БД
func runMigrations(cfg config.DatabaseConfig, logger *zap.Logger) error {
	logger.Info("running database migrations", zap.String("path", cfg.MigrationsPath))

	m, err := migrate.New(
		cfg.MigrationsPath,
		fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, cfg.SSLMode),
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	logger.Info("database migrations completed successfully")
	return nil
}
