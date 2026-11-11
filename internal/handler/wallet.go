package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/rutteddino/wallet-service/internal/domain"
	"github.com/rutteddino/wallet-service/internal/service"
)

type operationRequest struct {
	WalletID      uuid.UUID `json:"walletId"`
	OperationType string    `json:"operationType"`
	Amount        int64     `json:"amount"`
}

type balanceResponse struct {
	WalletID uuid.UUID `json:"walletId"`
	Balance  int64     `json:"balance"`
}

// WalletHandler обслуживает HTTP-эндпоинты кошельков.
type WalletHandler struct {
	svc *service.WalletService
}

// NewWalletHandler создаёт WalletHandler.
func NewWalletHandler(svc *service.WalletService) *WalletHandler {
	return &WalletHandler{svc: svc}
}

// NewWalletRouter собирает роутер сервиса с middleware.
// Порядок middleware осмысленный: RequestID генерирует id, логгер его использует,
// Recoverer перехватывает панику внутри, а лог всё равно записывается.
func NewWalletRouter(h *WalletHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(loggingMiddleware)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/wallet", h.ApplyOperation)
		r.Get("/wallets/{walletId}", h.GetBalance)
	})

	// Liveness probe для docker healthcheck и оркестраторов
	r.Get("/health", h.Health)

	return r
}

// Health обрабатывает GET /health: приложение живо и принимает запросы.
func (h *WalletHandler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ApplyOperation обрабатывает POST /api/v1/wallet.
func (h *WalletHandler) ApplyOperation(w http.ResponseWriter, r *http.Request) {
	var req operationRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body "+err.Error())
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	balance, err := h.svc.ApplyOperation(r.Context(), domain.WalletOperation{
		WalletID:       req.WalletID,
		Type:           domain.OperationType(req.OperationType),
		Amount:         req.Amount,
		IdempotencyKey: idempotencyKey})

	if err != nil {
		h.writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, balanceResponse{WalletID: req.WalletID, Balance: balance})
}

// GetBalance обрабатывает GET /api/v1/wallets/{walletId}.
func (h *WalletHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	walletID, err := uuid.Parse(chi.URLParam(r, "walletId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid walletId: must be UUID")
		return
	}

	balance, err := h.svc.GetBalance(r.Context(), walletID)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, balanceResponse{WalletID: walletID, Balance: balance})
}

func (h *WalletHandler) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrWalletNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, domain.ErrInvalidOperation):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("internal error", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
