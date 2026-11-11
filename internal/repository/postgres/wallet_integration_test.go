package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"uuid"

	"github.com/rutteddino/wallet-service/internal/domain"
)

func TestDeposit_CreatesWalletAndAccumulates(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()
	walletID := uuid.New()

	// Первый депозит создаёт кошелёк.
	balance, err := repo.ApplyOperation(ctx, domain.WalletOperation{
		WalletID: walletID, Type: domain.OperationDeposit, Amount: 1000, IdempotencyKey: uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("первый депозит: %v", err)
	}
	if balance != 1000 {
		t.Fatalf("ожидали 1000, получили %d", balance)
	}

	// Второй накапливает.
	balance, err = repo.ApplyOperation(ctx, domain.WalletOperation{
		WalletID: walletID, Type: domain.OperationDeposit, Amount: 500, IdempotencyKey: uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("второй депозит: %v", err)
	}
	if balance != 1500 {
		t.Fatalf("ожидали 1500, получили %d", balance)
	}
}

func TestWithdraw(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()
	walletID := uuid.New()

	if _, err := repo.ApplyOperation(ctx, domain.WalletOperation{
		WalletID: walletID, Type: domain.OperationDeposit, Amount: 300, IdempotencyKey: uuid.New().String(),
	}); err != nil {
		t.Fatalf("депозит: %v", err)
	}

	balance, err := repo.ApplyOperation(ctx, domain.WalletOperation{
		WalletID: walletID, Type: domain.OperationWithdraw, Amount: 100, IdempotencyKey: uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("вывод: %v", err)
	}
	if balance != 200 {
		t.Fatalf("ожидали 200, получили %d", balance)
	}
}

func TestWithdraw_InsufficientFunds(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()
	walletID := uuid.New()

	if _, err := repo.ApplyOperation(ctx, domain.WalletOperation{
		WalletID: walletID, Type: domain.OperationDeposit, Amount: 100, IdempotencyKey: uuid.New().String(),
	}); err != nil {
		t.Fatalf("депозит: %v", err)
	}

	_, err := repo.ApplyOperation(ctx, domain.WalletOperation{
		WalletID: walletID, Type: domain.OperationWithdraw, Amount: 101, IdempotencyKey: uuid.New().String(),
	})
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("ожидали ErrInsufficientFunds, получили %v", err)
	}

	// Баланс не должен измениться.
	balance, err := repo.GetBalance(ctx, walletID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if balance != 100 {
		t.Fatalf("баланс изменился после неудачного вывода: %d", balance)
	}
}

func TestWithdraw_WalletNotFound(t *testing.T) {
	repo := setup(t)

	_, err := repo.ApplyOperation(context.Background(), domain.WalletOperation{
		WalletID: uuid.New(), Type: domain.OperationWithdraw, Amount: 100, IdempotencyKey: uuid.New().String(),
	})
	if !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("ожидали ErrWalletNotFound, получили %v", err)
	}
}

func TestGetBalance_NotFound(t *testing.T) {
	repo := setup(t)

	_, err := repo.GetBalance(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("ожидали ErrWalletNotFound, получили %v", err)
	}
}

func TestApplyOperation_IdempotentRetry(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()
	walletID := uuid.New()
	key := uuid.New().String()

	first, err := repo.ApplyOperation(ctx, domain.WalletOperation{
		WalletID: walletID, Type: domain.OperationDeposit, Amount: 700, IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("операция: %v", err)
	}

	// Ретрай с тем же ключом: баланс не меняется, ответ успешный.
	second, err := repo.ApplyOperation(ctx, domain.WalletOperation{
		WalletID: walletID, Type: domain.OperationDeposit, Amount: 700, IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("ретрай не должен возвращать ошибку: %v", err)
	}
	if first != 700 || second != 700 {
		t.Fatalf("баланс изменился дважды: first=%d second=%d", first, second)
	}

	balance, err := repo.GetBalance(ctx, walletID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if balance != 700 {
		t.Fatalf("итоговый баланс %d, ожидали 700 (двойное зачисление!)", balance)
	}
}

// 1000 конкурентных операций по ОДНОМУ кошельку.
// Итоговый баланс должен сойтись точно, ни одна операция не должна
// завершиться внутренней ошибкой.
func TestApplyOperation_Concurrent(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()
	walletID := uuid.New()

	const (
		deposits       = 500
		withdraws      = 500
		depositAmount  = 10
		withdrawAmount = 5
		initial        = 100000
	)

	// Стартовый баланс.
	if _, err := repo.ApplyOperation(ctx, domain.WalletOperation{
		WalletID: walletID, Type: domain.OperationDeposit, Amount: initial, IdempotencyKey: uuid.New().String(),
	}); err != nil {
		t.Fatalf("начальный депозит: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, deposits+withdraws)

	run := func(opType domain.OperationType, amount int64) {
		defer wg.Done()
		_, err := repo.ApplyOperation(ctx, domain.WalletOperation{
			WalletID: walletID, Type: opType, Amount: amount, IdempotencyKey: uuid.New().String(),
		})
		if err != nil {
			errCh <- fmt.Errorf("%s: %w", opType, err)
		}
	}

	for i := 0; i < deposits; i++ {
		wg.Add(1)
		go run(domain.OperationDeposit, depositAmount)
	}
	for i := 0; i < withdraws; i++ {
		wg.Add(1)
		go run(domain.OperationWithdraw, withdrawAmount)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("операция завершилась ошибкой: %v", err)
	}

	want := int64(initial + deposits*depositAmount - withdraws*withdrawAmount)
	got, err := repo.GetBalance(ctx, walletID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if got != want {
		t.Fatalf("баланс не сошёлся: получили %d, ожидали %d (потерянные обновления!)", got, want)
	}
}
