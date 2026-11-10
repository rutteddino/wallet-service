package config

import (
	"os"
	"strconv"

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
	_ = godotenv.Load(path)

	cfg := Config{
		ServerPort:     envOr("SERVER_PORT", "8080"),
		DBHost:         envOr("DB_HOST", "localhost"),
		DBPort:         envOr("DB_PORT", "5432"),
		DBUser:         envOr("DB_USER", "postgres"),
		DBPassword:     envOr("DB_PASSWORD", "postgres"),
		DBName:         envOr("DB_NAME", "wallet"),
		DBMaxConns:     int32(envOrInt("DB_MAX_CONNS", 25)),
		MigrationsPath: envOr("MIGRATIONS_PATH", "file://migrations"),
	}

	return cfg, nil
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
