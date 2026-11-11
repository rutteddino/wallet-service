// Package repository определяет порты персистентности сервиса кошельков.
package repository

import (
	"context"
	"uuid"

	"github.com/rutteddino/wallet-service/internal/domain"
)

// WalletRepository - интерфейс доступа к данным для кошельков и операций.
type WalletRepository interface {
	// ApplyOperation атомарно применяет депозит/вывод к балансу кошелька
	// и возвращает итоговый баланс. Операция идемпотентна:
	// повторный вызов с тем же IdempotencyKey не изменяет баланс дважды.
	ApplyOperation(ctx context.Context, op domain.WalletOperation) (newBalance int64, err error)

	// GetBalance возвращает текущий баланс кошелька.
	// Если кошелёк не существует, возвращает domain.ErrWalletNotFound.
	GetBalance(ctx context.Context, walletID uuid.UUID) (int64, error)
}
