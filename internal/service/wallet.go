package service

import (
	"context"
	"uuid"

	"github.com/rutteddino/wallet-service/internal/domain"
	"github.com/rutteddino/wallet-service/internal/repository"
)

type WalletService struct {
	repo repository.WalletRepository
}

func NewWalletService(repo repository.WalletRepository) *WalletService {
	return &WalletService{repo: repo}
}

func (s *WalletService) ApplyOperation(ctx context.Context, op domain.WalletOperation) (int64, error) {
	if !op.Type.Valid() || op.Amount <= 0 {
		return 0, domain.ErrInvalidOperation
	}

	return s.repo.ApplyOperation(ctx, op)
}

func (s *WalletService) GetBalance(ctx context.Context, walerId uuid.UUID) (int64, error) {
	return s.repo.GetBalance(ctx, walerId)
}
