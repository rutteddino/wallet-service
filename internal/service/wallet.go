package service

import (
	"context"
	"uuid"

	"github.com/rutteddino/wallet-service/internal/domain"
	"github.com/rutteddino/wallet-service/internal/repository"
)

// WalletService реализует бизнес-логику кошельков.
type WalletService struct {
	repo repository.WalletRepository
}

// NewWalletService создаёт WalletService.
func NewWalletService(repo repository.WalletRepository) *WalletService {
	return &WalletService{repo: repo}
}

// ApplyOperation валидирует и применяет операцию к балансу.
func (s *WalletService) ApplyOperation(ctx context.Context, op domain.WalletOperation) (int64, error) {
	if !op.Type.Valid() || op.Amount <= 0 {
		return 0, domain.ErrInvalidOperation
	}

	// Если клиент не прислал ключ идемпотентности, генерируем сами,
	// чтобы каждая операция гарантированно имела уникальный ключ.
	if op.IdempotencyKey == "" {
		op.IdempotencyKey = uuid.New().String()
	}

	return s.repo.ApplyOperation(ctx, op)
}

// GetBalance возвращает текущий баланс кошелька.
func (s *WalletService) GetBalance(ctx context.Context, walletID uuid.UUID) (int64, error) {
	return s.repo.GetBalance(ctx, walletID)
}
