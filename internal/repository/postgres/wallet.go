package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rutteddino/wallet-service/internal/config"
	"github.com/rutteddino/wallet-service/internal/domain"
)

// WalletRepository хранит кошельки и операции в PostgreSQL.
type WalletRepository struct {
	pool *pgxpool.Pool
}

// NewWalletRepository создаёт пул соединений и ждёт, пока база
// начнёт отвечать (полезно сразу после docker compose up).
func NewWalletRepository(ctx context.Context, cfg config.Config) (*WalletRepository, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	poolCfg.MaxConns = cfg.DBMaxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	const attempts = 15
	for i := 1; ; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			break
		}

		if i == attempts {
			pool.Close()
			return nil, fmt.Errorf("database is not reachable after %d attempts: %w", attempts, err)
		}

		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}

	return &WalletRepository{pool: pool}, nil
}

// Close освобождает пул соединений.
func (r *WalletRepository) Close() {
	r.pool.Close()
}

// ApplyOperation реализует repository.WalletRepository.
//
// Стратегия работы с конкурентностью: изменение баланса выполняется
// одним атомарным SQL-запросом внутри транзакции вместе с записью
// об операции. Postgres берёт короткую блокировку на уровне строки
// кошелька, поэтому конкурентные операции с одним кошельком
// сериализуются самой базой, а баланс не может уйти в минус
// благодаря условию в WHERE.
func (r *WalletRepository) ApplyOperation(ctx context.Context, op domain.WalletOperation) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Сначала меняем баланс: это гарантирует, что к моменту вставки
	// записи об операции строка кошелька уже существует (Foreign Key constraint).
	var balance int64
	switch op.Type {
	case domain.OperationDeposit:
		// Кошелёк создаётся автоматически при первом депозите. ON CONFLICT
		// сохраняет корректность, когда множество конкурентных депозитов
		// одновременно пытаются создать один и тот же кошелёк.
		err = tx.QueryRow(ctx, `
			INSERT INTO wallets (id, balance)
			VALUES ($1, $2)
			ON CONFLICT (id) DO UPDATE
			SET balance = wallets.balance + EXCLUDED.balance, updated_at = now()
			RETURNING balance`,
			op.WalletID, op.Amount).Scan(&balance)
	case domain.OperationWithdraw:
		err = tx.QueryRow(ctx, `
			UPDATE wallets
			SET balance = balance - $2, updated_at = now()
			WHERE id = $1 AND balance >= $2
			RETURNING balance`,
			op.WalletID, op.Amount).Scan(&balance)
		if errors.Is(err, pgx.ErrNoRows) {
			// Отличаем "Нет кошелька" от "Не хватает средств".
			var exists bool
			if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM wallets WHERE id = $1)`, op.WalletID).Scan(&exists); e != nil {
				return 0, fmt.Errorf("check wallet existence: %w", e)
			}
			if !exists {
				return 0, domain.ErrWalletNotFound
			}
			return 0, domain.ErrInsufficientFunds
		}
	default:
		return 0, domain.ErrInvalidOperation
	}

	if err != nil {
		return 0, fmt.Errorf("apply %s: %w", op.Type, err)
	}

	// 2. Записываем операцию; уникальный ключ идемпотентности
	// делает повторные запросы безопасными.
	tag, err := tx.Exec(ctx, `
		INSERT INTO operations (id, wallet_id, operation_type, amount, idempotency_key)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		uuid.New(), op.WalletID, string(op.Type), op.Amount, op.IdempotencyKey)

	if err != nil {
		return 0, fmt.Errorf("insert operation: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// Идемпотентный ретрай: операция уже была применена.
		// Откатываем только что сделанное изменение баланса
		// и возвращаем текущий зафиксированный баланс.
		_ = tx.Rollback(ctx)
		// WithoutCancel: клиент мог отключиться после ретрая,
		// но штатный ответ всё равно нужно попытаться отдать.
		return r.GetBalance(context.WithoutCancel(ctx), op.WalletID)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return balance, nil
}

// GetBalance реализует repository.WalletRepository.
func (r *WalletRepository) GetBalance(ctx context.Context, walletID uuid.UUID) (int64, error) {
	var balance int64
	err := r.pool.QueryRow(ctx, `SELECT balance FROM wallets WHERE id = $1`, walletID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrWalletNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get balance: %w", err)
	}
	return balance, nil
}
