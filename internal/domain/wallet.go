package domain

import (
	"errors"
	"time"
	"uuid"
)

// Wallet это один кошелёк. Баланс хранится в минорных единицах.
type Wallet struct {
	ID        uuid.UUID
	Balance   int64 // В минорных единицах (копейках)
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OperationType это тип операции, меняющей баланс.
type OperationType string

// Известные типы операций.
const (
	OperationDeposit  OperationType = "DEPOSIT"
	OperationWithdraw OperationType = "WITHDRAW"
)

// Valid сообщает, является ли t известным типом операции.
func (t OperationType) Valid() bool {
	return t == OperationDeposit || t == OperationWithdraw
}

// WalletOperation это запрос на изменение баланса кошелька.
type WalletOperation struct {
	WalletID       uuid.UUID
	Type           OperationType
	Amount         int64 // В минорных единицах (копейках)
	IdempotencyKey string
}

// Доменные ошибки
var (
	ErrWalletNotFound    = errors.New("wallet not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidOperation  = errors.New("invalid operation")
)
