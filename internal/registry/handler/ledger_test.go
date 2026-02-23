package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/handler"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/trustledger"
	"go.uber.org/zap"
)

func setupLedgerRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ledger := trustledger.New()
	h := handler.NewLedgerHandler(ledger, zap.NewNop())
	v1 := r.Group("/api/v1")
	h.Register(v1)
	return r
}

func TestLedgerOverview_200(t *testing.T) {
	router := setupLedgerRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	entries := int(resp["entries"].(float64))
	if entries != 1 { // genesis
		t.Errorf("expected 1 entry (genesis), got %d", entries)
	}
}

func TestLedgerVerify_200(t *testing.T) {
	router := setupLedgerRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledger/verify", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["valid"] != true {
		t.Errorf("expected valid=true, got %v", resp["valid"])
	}
}

func TestLedgerGetEntry_200_genesis(t *testing.T) {
	router := setupLedgerRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledger/entries/0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLedgerGetEntry_404(t *testing.T) {
	router := setupLedgerRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledger/entries/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestLedgerGetEntry_400_invalidIdx(t *testing.T) {
	router := setupLedgerRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledger/entries/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
