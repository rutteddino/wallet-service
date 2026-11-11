package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testPool *pgxpool.Pool

// TestMain поднимает Postgres в контейнере на весь пакет,
// применяет настоящие миграции и гасит контейнер после прогона.
// Без работающего Docker интеграционные тесты пропускаются.
func TestMain(m *testing.M) {
	ctx := context.Background()

	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("wallet_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	if err != nil {
		// На этом этапе ошибка почти всегда означает,
		// что Docker не запущен, поэтому пропускаем, а не падаем.
		log.Printf("docker is not available (%v), skipping integration tests", err)
		os.Exit(0)
	}

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("connection string: %v", err)
	}
	if err := runTestMigrations(dsn); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("pool: %v", err)
	}

	code := m.Run()

	testPool.Close()
	if err := pg.Terminate(ctx); err != nil {
		log.Printf("terminate container: %v", err)
	}
	os.Exit(code)
}

// setup возвращает репозиторий с чистыми таблицами:
// каждый тест начинает с пустого состояния.
func setup(t *testing.T) *WalletRepository {
	t.Helper()

	if _, err := testPool.Exec(context.Background(), `TRUNCATE operations, wallets`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return &WalletRepository{pool: testPool}
}

// runTestMigrations применяет миграции из корня репозитория:
// те же файлы и тот же движок, что и в проде.
// Source делаем через iofs (os.DirFS): file://-источник golang-migrate
// не умеет в Windows-пути вида D:\... (двоеточие ломает парсинг URL).
func runTestMigrations(dsn string) error {
	migrationsDir, err := filepath.Abs("../../../migrations")
	if err != nil {
		return err
	}

	src, err := iofs.New(os.DirFS(migrationsDir), ".")
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx", driver)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
