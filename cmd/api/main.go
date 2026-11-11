package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/rutteddino/wallet-service/internal/config"
	"github.com/rutteddino/wallet-service/internal/handler"
	"github.com/rutteddino/wallet-service/internal/repository/postgres"
	"github.com/rutteddino/wallet-service/internal/service"
)

func main() {
	// Логгер
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Вся логика в run: defer там отрабатывают штатно,
	// а os.Exit здесь вызывается уже после них.
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Чтение config.env
	cfg, err := config.Load("config.env")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()

	// Стартовые операции ограничиваем таймаутом: если БД висит
	// (TCP открыт, но не отвечает), падаем с ошибкой за 30 секунд,
	// а не стартуем вечно.
	startupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Миграции
	if err := runMigrations(cfg); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	slog.Info("migrations applied")

	// Коннект к ДБ
	repo, err := postgres.NewWalletRepository(startupCtx, cfg)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer repo.Close()
	slog.Info("connected to database")

	// Создание сервера (хэндлер, сервис)
	svc := service.NewWalletService(repo)
	router := handler.NewWalletRouter(handler.NewWalletHandler(svc))

	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           router,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server started", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Ждём сигнал остановки или падение сервера
	select {
	case <-shutdownCtx.Done():
		slog.Info("shutting down")
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	}

	// Даём активным запросам до 10 секунд на завершение
	graceCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(graceCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	slog.Info("server stopped")
	return nil
}

func runMigrations(cfg config.Config) error {
	m, err := migrate.New(cfg.MigrationsPath, cfg.MigrateURL())
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
