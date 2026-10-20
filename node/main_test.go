package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HerodotusTree/grove-network/identity"
	"github.com/HerodotusTree/grove-network/ledger"
	"github.com/HerodotusTree/grove-network/vault"
)

const testGenesis = `{"kind":"genesis","network":"test"}`

func testDomain() string {
	probe := ledger.New()
	g, err := probe.AppendGenesis(testGenesis)
	if err != nil {
		panic(err)
	}
	return g.Hash
}

func newTestServer(t *testing.T) *server {
	return newTestServerGenesis(t, testGenesis)
}
func newTestServerGenesis(t *testing.T, genesis string) *server {
	t.Helper()
	dir := t.TempDir()
	s := &server{
		v:         vault.New(ledger.New()),
		nonces:    map[string]uint64{},
		dataDir:   dir,
		chainPath: dir + "/chain.json",
	}
	if err := s.applyGenesis(genesis); err != nil {
		t.Fatalf("test genesis: %v", err)
	}
	return s
}

// signReq builds a signed request body for op as the holder of privHex.
func signReq(t *testing.T, op, privHex, account string, amount int64, nonce uint64) []byte {
	t.Helper()
	sig, err := identity.Sign(privHex, identity.PayloadFor(testDomain(), op, account, amount, nonce))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	body, _ := json.Marshal(signedReq{Account: account, Amount: amount, Nonce: nonce, Signature: sig})
	return body
}

func postDeposit(s *server, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/deposit", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleDeposit(rec, req)
	return rec
}

func TestCheckAuthValid(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()
	sig, _ := identity.Sign(priv, identity.PayloadFor(testDomain(), identity.OpDeposit, pub, 100, 1))

	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.checkAuth(identity.OpDeposit, signedReq{Account: pub, Amount: 100, Nonce: 1, Signature: sig})
	if err != nil {
		t.Fatalf("valid op rejected: %v", err)
	}
}

func TestCheckAuthBadNonce(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()

	s.mu.Lock()
	defer s.mu.Unlock()
	// First op must be nonce 1, not 2.
	sig, _ := identity.Sign(priv, identity.PayloadFor(testDomain(), identity.OpDeposit, pub, 100, 2))
	if err := s.checkAuth(identity.OpDeposit, signedReq{Account: pub, Amount: 100, Nonce: 2, Signature: sig}); err == nil {
		t.Fatal("nonce 2 accepted as first op")
	}
	// Nonce 0 is never valid.
	sig, _ = identity.Sign(priv, identity.PayloadFor(testDomain(), identity.OpDeposit, pub, 100, 0))
	if err := s.checkAuth(identity.OpDeposit, signedReq{Account: pub, Amount: 100, Nonce: 0, Signature: sig}); err == nil {
		t.Fatal("nonce 0 accepted")
	}
}

func TestCheckAuthReplayRejected(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()
	sig, _ := identity.Sign(priv, identity.PayloadFor(testDomain(), identity.OpDeposit, pub, 100, 1))
	req := signedReq{Account: pub, Amount: 100, Nonce: 1, Signature: sig}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkAuth(identity.OpDeposit, req); err != nil {
		t.Fatalf("first op rejected: %v", err)
	}
	// Simulate the accepted op advancing the nonce.
	s.nonces[pub] = 1
	if err := s.checkAuth(identity.OpDeposit, req); err == nil {
		t.Fatal("replayed nonce 1 accepted")
	}
}

func TestCheckAuthBadSignature(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()
	// Sign a different amount than the one claimed.
	sig, _ := identity.Sign(priv, identity.PayloadFor(testDomain(), identity.OpDeposit, pub, 999, 1))

	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.checkAuth(identity.OpDeposit, signedReq{Account: pub, Amount: 100, Nonce: 1, Signature: sig})
	if err == nil {
		t.Fatal("mismatched signature accepted")
	}
	// And a signature from a different key entirely.
	priv2, _, _ := identity.Generate()
	sig2, _ := identity.Sign(priv2, identity.PayloadFor(testDomain(), identity.OpDeposit, pub, 100, 1))
	if err := s.checkAuth(identity.OpDeposit, signedReq{Account: pub, Amount: 100, Nonce: 1, Signature: sig2}); err == nil {
		t.Fatal("wrong-key signature accepted")
	}
}

func TestCheckAuthMissingAccount(t *testing.T) {
	s := newTestServer(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkAuth(identity.OpDeposit, signedReq{Amount: 100, Nonce: 1, Signature: "ab"}); err == nil {
		t.Fatal("empty account accepted")
	}
}

func TestCheckAuthWrongOp(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()
	// A deposit signature presented as a redeem must fail.
	sig, _ := identity.Sign(priv, identity.PayloadFor(testDomain(), identity.OpDeposit, pub, 100, 1))

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkAuth(identity.OpRedeem, signedReq{Account: pub, Amount: 100, Nonce: 1, Signature: sig}); err == nil {
		t.Fatal("deposit signature accepted as redeem")
	}
}

func TestDepositEndToEnd(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()

	rec := postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 100, 1))
	if rec.Code != http.StatusOK {
		t.Fatalf("signed deposit: got %d, body %s", rec.Code, rec.Body.String())
	}
	if s.v.R != 100 || s.v.C != 100 {
		t.Fatalf("vault state wrong: R=%d C=%d", s.v.R, s.v.C)
	}
	if s.nonces[pub] != 1 {
		t.Fatalf("nonce not advanced: %d", s.nonces[pub])
	}

	// Replay the exact same body: must be 401, and the vault untouched.
	rec = postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 100, 1))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("replay: got %d, want 401", rec.Code)
	}
	if s.v.R != 100 {
		t.Fatalf("replay mutated the vault: R=%d", s.v.R)
	}

	// Bad signature: 401 and counted as rejected.
	badSig, _ := json.Marshal(signedReq{Account: pub, Amount: 50, Nonce: 2, Signature: "00"})
	rec = postDeposit(s, badSig)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad sig: got %d, want 401", rec.Code)
	}
	if s.m.OpsRejected == 0 {
		t.Fatal("auth failures not counted in OpsRejected")
	}

	// Next valid nonce works.
	rec = postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 50, 2))
	if rec.Code != http.StatusOK {
		t.Fatalf("second deposit: got %d, body %s", rec.Code, rec.Body.String())
	}
	if s.v.R != 150 {
		t.Fatalf("R=%d, want 150", s.v.R)
	}
}

func TestNoncesPersistAcrossRestart(t *testing.T) {
	s := newTestServer(t)
	priv, pub, _ := identity.Generate()
	rec := postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 100, 1))
	if rec.Code != http.StatusOK {
		t.Fatalf("deposit: %d", rec.Code)
	}
	// Simulate a restart: fresh server, same data dir. Nonce state is
	// rebuilt from the ledger alone - there is no nonce file anymore.
	s2 := &server{dataDir: s.dataDir, chainPath: s.chainPath}
	v, err := s2.loadOrNew()
	if err != nil {
		t.Fatalf("reload chain: %v", err)
	}
	s2.v = v
	if err := s2.applyGenesis(testGenesis); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	s2.deriveNonces()
	// The old nonce must still be rejected after restart.
	rec = postDeposit(s2, signReq(t, identity.OpDeposit, priv, pub, 100, 1))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("replay after restart: got %d, want 401", rec.Code)
	}
	// And the next nonce must work: the rebuilt table is exact.
	rec = postDeposit(s2, signReq(t, identity.OpDeposit, priv, pub, 50, 2))
	if rec.Code != http.StatusOK {
		t.Fatalf("next nonce after restart: got %d, body %s", rec.Code, rec.Body.String())
	}
}

// TestPeerURLValidation pins the registration contract {"url":
// "http://host:port"}: sync appends "/ledger" to the string verbatim,
// so only a bare http(s) base URL is accepted (issue #7).
func TestPeerURLValidation(t *testing.T) {
	valid := map[string]string{
		"http://127.0.0.1:8080":   "http://127.0.0.1:8080",
		"http://127.0.0.1:8080/":  "http://127.0.0.1:8080", // trailing slash canonicalized away
		"https://peer.example:99": "https://peer.example:99",
		"http://localhost:8081":   "http://localhost:8081",
	}
	for in, want := range valid {
		got, err := validatePeerURL(in)
		if err != nil {
			t.Errorf("validatePeerURL(%q): rejected: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("validatePeerURL(%q) = %q, want %q", in, got, want)
		}
	}
	invalid := []string{
		"",
		"not a url",
		"http://127.0.0.1:8080/internal-prefix", // the demonstrated SSRF vector: path prefix
		"http://127.0.0.1:8080/ledger",          // any path at all
		"http://127.0.0.1:8080/?q=1",            // query
		"http://127.0.0.1:8080/#frag",           // fragment
		"http://user:pass@127.0.0.1:8080/",      // userinfo
		"ftp://127.0.0.1:8080/",                 // non-http(s) scheme
		"gopher://127.0.0.1:70/",                // non-http(s) scheme
		"file:///etc/passwd",                    // non-http(s) scheme
		"http:///no-host",                       // missing host
		"http://127.0.0.1:8080/%2e%2e/x",        // encoded path traversal still has a path
	}
	for _, in := range invalid {
		if got, err := validatePeerURL(in); err == nil {
			t.Errorf("validatePeerURL(%q): accepted as %q, want rejection", in, got)
		}
	}
}
