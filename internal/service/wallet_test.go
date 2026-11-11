package service

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/rutteddino/wallet-service/internal/domain"
)

// fakeRepo — ручной мок repository.WalletRepository.
type fakeRepo struct {
	calls   int
	lastOp  domain.WalletOperation
	balance int64
	err     error
}

func (f *fakeRepo) ApplyOperation(_ context.Context, op domain.WalletOperation) (int64, error) {
	f.calls++
	f.lastOp = op
	return f.balance, f.err
}

func (f *fakeRepo) GetBalance(_ context.Context, _ uuid.UUID) (int64, error) {
	return f.balance, f.err
}

func TestApplyOperation_Validation(t *testing.T) {
	tests := []struct {
		name string
		op   domain.WalletOperation
	}{
		{"невалидный тип операции", domain.WalletOperation{WalletID: uuid.New(), Type: "TRANSFER", Amount: 100}},
		{"нулевая сумма", domain.WalletOperation{WalletID: uuid.New(), Type: domain.OperationDeposit, Amount: 0}},
		{"отрицательная сумма", domain.WalletOperation{WalletID: uuid.New(), Type: domain.OperationWithdraw, Amount: -50}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := NewWalletService(repo)

			_, err := svc.ApplyOperation(context.Background(), tt.op)

			if !errors.Is(err, domain.ErrInvalidOperation) {
				t.Fatalf("ожидали ErrInvalidOperation, получили %v", err)
			}
			if repo.calls != 0 {
				t.Fatalf("репозиторий не должен вызываться при невалидном вводе, вызовов: %d", repo.calls)
			}
		})
	}
}

func TestApplyOperation_GeneratesIdempotencyKey(t *testing.T) {
	repo := &fakeRepo{balance: 100}
	svc := NewWalletService(repo)
	op := domain.WalletOperation{WalletID: uuid.New(), Type: domain.OperationDeposit, Amount: 100}

	// Ключ не задан — сервис обязан сгенерировать.
	if _, err := svc.ApplyOperation(context.Background(), op); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if repo.lastOp.IdempotencyKey == "" {
		t.Fatal("сервис должен генерировать IdempotencyKey, если он не задан")
	}
	if _, err := uuid.Parse(repo.lastOp.IdempotencyKey); err != nil {
		t.Fatalf("сгенерированный ключ должен быть валидным UUID: %v", err)
	}

	// Два вызова без ключа — два РАЗНЫХ ключа (иначе ретраи «склеятся»).
	first := repo.lastOp.IdempotencyKey
	if _, err := svc.ApplyOperation(context.Background(), op); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if repo.lastOp.IdempotencyKey == first {
		t.Fatal("ключи разных операций должны различаться")
	}
}

func TestApplyOperation_KeepsProvidedKey(t *testing.T) {
	repo := &fakeRepo{balance: 100}
	svc := NewWalletService(repo)
	op := domain.WalletOperation{
		WalletID:       uuid.New(),
		Type:           domain.OperationDeposit,
		Amount:         100,
		IdempotencyKey: "client-key-123",
	}

	if _, err := svc.ApplyOperation(context.Background(), op); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if repo.lastOp.IdempotencyKey != "client-key-123" {
		t.Fatalf("ключ клиента не должен перезаписываться, получили %q", repo.lastOp.IdempotencyKey)
	}
}

func TestApplyOperation_PassesThroughRepoResult(t *testing.T) {
	repo := &fakeRepo{balance: 4200}
	svc := NewWalletService(repo)

	balance, err := svc.ApplyOperation(context.Background(), domain.WalletOperation{
		WalletID: uuid.New(), Type: domain.OperationWithdraw, Amount: 100,
	})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if balance != 4200 {
		t.Fatalf("ожидали баланс 4200, получили %d", balance)
	}
}

func TestApplyOperation_PropagatesRepoError(t *testing.T) {
	repo := &fakeRepo{err: domain.ErrInsufficientFunds}
	svc := NewWalletService(repo)

	_, err := svc.ApplyOperation(context.Background(), domain.WalletOperation{
		WalletID: uuid.New(), Type: domain.OperationWithdraw, Amount: 100,
	})
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("ошибка репозитория должна прокидываться как есть, получили %v", err)
	}
}

func TestGetBalance(t *testing.T) {
	repo := &fakeRepo{balance: 777}
	svc := NewWalletService(repo)

	balance, err := svc.GetBalance(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if balance != 777 {
		t.Fatalf("ожидали 777, получили %d", balance)
	}
}
