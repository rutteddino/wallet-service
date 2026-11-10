package repository

import (
	"context"
	"uuid"

	"github.com/rutteddino/wallet-service/internal/domain"
)

type WalletRepository interface {
	ApplyOperation(ctx context.Context, op domain.WalletOperation) (newBalance int64, err error)
	GetBalance(ctx context.Context, walletID uuid.UUID) (int64, error)
}
