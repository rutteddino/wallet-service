package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/rutteddino/wallet-service/internal/config"
)

func main() {
	// Логгер
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Чтение config.env
	_, err := config.Load("config.env")
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	// Миграции?
	// Коннект к ДБ
	// Создание сервера (хэндлер, сервис)
	fmt.Printf("Hello, %v", uuid.New())
}
