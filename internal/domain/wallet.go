package domain

import (
	"errors"
	"time"
	"uuid"
)

type Wallet struct {
	ID        uuid.UUID
	Balance   int64 // В минорных единицах (копейках)
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OperationType string

const (
	OperationDeposit  OperationType = "DEPOSIT"
	OperationWithdraw OperationType = "WITHDRAW"
)

func (t OperationType) Valid() bool {
	return t == OperationDeposit || t == OperationWithdraw
}

type WalletOperation struct {
	WalletID uuid.UUID
	Type     OperationType
	Amount   int64 // В минорных единицах (копейках)
}

// Доменные ошибки
var (
	ErrWalletNotFound    = errors.New("wallet not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidOperation  = errors.New("invalid operation")
)
