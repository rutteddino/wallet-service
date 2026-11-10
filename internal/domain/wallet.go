package domain

import (
	"errors"
	"time"
	"uuid"
)

type Wallet struct {
	ID        uuid.UUID
	Balance   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OperationType string

const (
	OperationDeposit  OperationType = "DEPOSIT"
	OperationWithdraw OperationType = "WITHDRAW"
)

type WalletOperation struct {
	WalletID uuid.UUID
	Type     OperationType
	Amount   int64
}

// Доменые ошибки
var (
	ErrWalletNotFound    = errors.New("wallet not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidOperation  = errors.New("invalid operation")
)
