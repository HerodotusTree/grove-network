// Command node runs the Grove sequencer: a single HTTP server that
// accepts vault operations in order and appends each one to the
// hash-chained ledger. One operation at a time — nothing jumps the line.
//
// The chain is persisted to grove-data/chain.json after every operation
// and reloaded on startup, so the node remembers across restarts.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/HerodotusTree/grove-network/ledger"
	"github.com/HerodotusTree/grove-network/vault"
)

const dataDir = "grove-data"
const chainPath = "grove-data/chain.json"

type server struct {
	mu sync.Mutex
	v  *vault.Vault
}

// loadOrNew loads the persisted chain if it exists and rebuilds the
// vault's state from the last recorded operation. A broken chain on
// disk refuses to start — the node will not run on lies.
func loadOrNew() (*vault.Vault, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}
	l, err := ledger.Load(chainPath)
	if err != nil {
		if os.IsNotExist(err) {
			return vault.New(ledger.New()), nil
		}
		return nil, err
	}
	v := vault.New(l)
	if n := l.Len(); n > 0 {
		var op vault.Op
		if err := json.Unmarshal([]byte(l.Entries()[n-1].Payload), &op); err != nil {
			return nil, fmt.Errorf("replay: %w", err)
		}
		v.R, v.C = op.R, op.C
	}
	return v, nil
}

type amountReq struct {
	Amount int64 `json:"amount"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *server) state() map[string]int64 {
	return map[string]int64{
		"reserves": s.v.R,
		"claims":   s.v.C,
		"surplus":  s.v.Surplus(),
	}
}

// persist saves the chain. Called with the lock held, after every
// accepted operation.
func (s *server) persist(w http.ResponseWriter) bool {
	if err := s.v.Log.Save(chainPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist failed: " + err.Error()})
		return false
	}
	return true
}

func (s *server) handleDeposit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	var req amountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.v.Deposit(req.Amount); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	if !s.persist(w) {
		return
	}
	writeJSON(w, http.StatusOK, s.state())
}

func (s *server) handleRedeem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	var req amountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	payout, err := s.v.Redeem(req.Amount)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	if !s.persist(w) {
		return
	}
	resp := s.state()
	resp["payout"] = payout
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, s.state())
}

func (s *server) handleLedger(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.v.Log.Verify(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "chain broken: " + err.Error()})
		return
	}
	dump, _ := s.v.Log.Dump()
	w.Header().Set("Content-Type", "application/json")
	w.Write(dump)
}

func main() {
	v, err := loadOrNew()
	if err != nil {
		log.Fatalf("refusing to start: %v", err)
	}
	s := &server{v: v}
	http.HandleFunc("/deposit", s.handleDeposit)
	http.HandleFunc("/redeem", s.handleRedeem)
	http.HandleFunc("/state", s.handleState)
	http.HandleFunc("/ledger", s.handleLedger)
	fmt.Println("Grove sequencer listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
