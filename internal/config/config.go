package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBMaxConns int32

	MigrationsPath string
}

func Load(path string) (Config, error) {
	// Необрабатываем ошибку, переменые могут быть в реальном окружении
	_ = godotenv.Load(path)

	// Пароль обязателен: если переменная не задана вовсе - падаем с понятной
	// ошибкой на старте. Явно заданный пустой пароль (DB_PASSWORD=) допустим
	// для trust-аутентификации.
	dbPassword, ok := os.LookupEnv("DB_PASSWORD")
	if !ok {
		return Config{}, fmt.Errorf("DB_PASSWORD is required")
	}

	cfg := Config{
		ServerPort:     envOr("SERVER_PORT", "8080"),
		DBHost:         envOr("DB_HOST", "localhost"),
		DBPort:         envOr("DB_PORT", "5432"),
		DBUser:         envOr("DB_USER", "postgres"),
		DBPassword:     dbPassword,
		DBName:         envOr("DB_NAME", "wallet"),
		DBMaxConns:     int32(envOrInt("DB_MAX_CONNS", 25)),
		MigrationsPath: envOr("MIGRATIONS_PATH", "file://migrations"),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBHost, c.DBName)
}

func (c Config) validate() error {
	if c.ServerPort == "" {
		return fmt.Errorf("SERVER_PORT must not be empty")
	}
	if c.DBHost == "" {
		return fmt.Errorf("DB_HOST must not be empty")
	}
	if c.DBPort == "" {
		return fmt.Errorf("DB_PORT must not be empty")
	}
	if c.DBUser == "" {
		return fmt.Errorf("DB_USER must not be empty")
	}
	if c.DBName == "" {
		return fmt.Errorf("DB_NAME must not be empty")
	}
	if c.DBMaxConns <= 0 {
		return fmt.Errorf("DB_MAX_CONNS must be positive, got %d", c.DBMaxConns)
	}
	// Путь к миграциям должен быть file:// URL на существующую директорию,
	// иначе ошибка всплывёт только при запуске миграций и менее читаемо.
	dir, ok := strings.CutPrefix(c.MigrationsPath, "file://")
	if !ok {
		return fmt.Errorf("MIGRATIONS_PATH must be a file:// URL, got %q", c.MigrationsPath)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return fmt.Errorf("MIGRATIONS_PATH points to a non-existent directory: %q", dir)
	}

	return nil
}

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}

	return def
}

func envOrInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}

	return def
}
