package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
)

// Phase 1 step 11 (STATUS-01): /status reports an account's balance
// and nonce plus the ledger position - read-only, informational only.
func TestStatusReportsAccountAndPosition(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()
	if rec := postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 100, 1)); rec.Code != http.StatusOK {
		t.Fatalf("deposit: %d", rec.Code)
	}
	get := func(account string) map[string]any {
		rec := httptest.NewRecorder()
		s.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/status?account="+account, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status for %q: %d", account, rec.Code)
		}
		var m map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	m := get(pub)
	if m["balance"] != float64(100) || m["nonce"] != float64(1) {
		t.Fatalf("status wrong: %v", m)
	}
	if m["height"] != float64(2) || m["tip"] == "" || m["domain"] != testDomain() {
		t.Fatalf("position wrong: %v", m)
	}
	m = get("deadbeef")
	if m["balance"] != float64(0) || m["nonce"] != float64(0) {
		t.Fatalf("unknown account should read zero: %v", m)
	}
	rec := httptest.NewRecorder()
	s.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing account: got %d, want 400", rec.Code)
	}
}
