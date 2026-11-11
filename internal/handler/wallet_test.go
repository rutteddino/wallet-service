package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/rutteddino/wallet-service/internal/domain"
	"github.com/rutteddino/wallet-service/internal/service"
)

// fakeRepo — ручной мок repository.WalletRepository.
type fakeRepo struct {
	lastOp  domain.WalletOperation
	balance int64
	err     error
}

func (f *fakeRepo) ApplyOperation(_ context.Context, op domain.WalletOperation) (int64, error) {
	f.lastOp = op
	return f.balance, f.err
}

func (f *fakeRepo) GetBalance(_ context.Context, _ uuid.UUID) (int64, error) {
	return f.balance, f.err
}

func newTestServer(repo *fakeRepo) *httptest.Server {
	svc := service.NewWalletService(repo)
	return httptest.NewServer(NewWalletRouter(NewWalletHandler(svc)))
}

func TestApplyOperation_HappyPath(t *testing.T) {
	repo := &fakeRepo{balance: 1500}
	srv := newTestServer(repo)
	defer srv.Close()

	walletID := uuid.New().String()
	body := `{"walletId":"` + walletID + `","operationType":"DEPOSIT","amount":500}`

	resp, err := http.Post(srv.URL+"/api/v1/wallet", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("запрос упал: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Request-Id"); got == "" {
		t.Error("ожидали заголовок X-Request-Id от middleware.RequestID")
	}
}

func TestApplyOperation_IdempotencyKeyFromHeader(t *testing.T) {
	repo := &fakeRepo{balance: 100}
	srv := newTestServer(repo)
	defer srv.Close()

	body := `{"walletId":"` + uuid.New().String() + `","operationType":"DEPOSIT","amount":100}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/wallet", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "my-key-42")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("запрос упал: %v", err)
	}
	resp.Body.Close()

	if repo.lastOp.IdempotencyKey != "my-key-42" {
		t.Fatalf("ключ из заголовка должен дойти до репозитория, получили %q", repo.lastOp.IdempotencyKey)
	}
}

func TestApplyOperation_BadRequests(t *testing.T) {
	repo := &fakeRepo{}
	srv := newTestServer(repo)
	defer srv.Close()

	tests := []struct {
		name string
		body string
	}{
		{"битый JSON", `{"walletId":`},
		{"невалидный UUID", `{"walletId":"not-a-uuid","operationType":"DEPOSIT","amount":100}`},
		{"неизвестный тип операции", `{"walletId":"` + uuid.New().String() + `","operationType":"TRANSFER","amount":100}`},
		{"нулевая сумма", `{"walletId":"` + uuid.New().String() + `","operationType":"DEPOSIT","amount":0}`},
		{"отрицательная сумма", `{"walletId":"` + uuid.New().String() + `","operationType":"WITHDRAW","amount":-5}`},
		{"лишнее поле (DisallowUnknownFields)", `{"walletId":"` + uuid.New().String() + `","operationType":"DEPOSIT","amount":100,"hack":true}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Post(srv.URL+"/api/v1/wallet", "application/json", strings.NewReader(tt.body))
			if err != nil {
				t.Fatalf("запрос упал: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("ожидали 400, получили %d", resp.StatusCode)
			}
		})
	}
}

func TestApplyOperation_DomainErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		repoErr    error
		wantStatus int
	}{
		{"нет кошелька", domain.ErrWalletNotFound, http.StatusNotFound},
		{"не хватает средств", domain.ErrInsufficientFunds, http.StatusUnprocessableEntity},
		{"внутренняя ошибка", errors.New("db is on fire"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{err: tt.repoErr}
			srv := newTestServer(repo)
			defer srv.Close()

			body := `{"walletId":"` + uuid.New().String() + `","operationType":"WITHDRAW","amount":100}`
			resp, err := http.Post(srv.URL+"/api/v1/wallet", "application/json", strings.NewReader(body))
			if err != nil {
				t.Fatalf("запрос упал: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("ожидали %d, получили %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestGetBalance(t *testing.T) {
	repo := &fakeRepo{balance: 999}
	srv := newTestServer(repo)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/wallets/" + uuid.New().String())
	if err != nil {
		t.Fatalf("запрос упал: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", resp.StatusCode)
	}
}

func TestGetBalance_InvalidUUID(t *testing.T) {
	repo := &fakeRepo{}
	srv := newTestServer(repo)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/wallets/not-a-uuid")
	if err != nil {
		t.Fatalf("запрос упал: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", resp.StatusCode)
	}
}

func TestGetBalance_NotFound(t *testing.T) {
	repo := &fakeRepo{err: domain.ErrWalletNotFound}
	srv := newTestServer(repo)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/wallets/" + uuid.New().String())
	if err != nil {
		t.Fatalf("запрос упал: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", resp.StatusCode)
	}
}
