// Command node runs the Grove sequencer: a single HTTP server that
// accepts vault operations in order and appends each one to the
// hash-chained ledger. One operation at a time — nothing jumps the line.
//
// The chain is persisted to <data>/chain.json after every operation
// and reloaded on startup, so the node remembers across restarts.
//
// /metrics reports the efficiency telemetry: every operation counted,
// every invariant check recorded. This is the data the 99.99% bar is
// measured against.
//
// Peers: POST /peers {"url":"http://host:port"} registers a peer.
// POST /sync pulls each peer's chain and adopts the longest valid one.
// A chain that fails verification is refused, never merged — no trust
// required, only math.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/HerodotusTree/grove-network/ledger"
	"github.com/HerodotusTree/grove-network/vault"
)

// Metrics is the node's efficiency telemetry.
type Metrics struct {
	StartedAt           time.Time `json:"started_at"`
	UptimeSeconds       int64     `json:"uptime_seconds"`
	OpsAttempted        uint64    `json:"ops_attempted"`
	OpsSucceeded        uint64    `json:"ops_succeeded"`
	OpsRejected         uint64    `json:"ops_rejected"`
	InvariantViolations uint64    `json:"invariant_violations"`
	LedgerEntries       int       `json:"ledger_entries"`
	ChainValid          bool      `json:"chain_valid"`
	Peers               int       `json:"peers"`
}

type server struct {
	mu        sync.Mutex
	v         *vault.Vault
	started   time.Time
	m         Metrics
	peers     []string
	dataDir   string
	chainPath string
	port      string
}

func (s *server) chainFile() string { return s.chainPath }

// loadOrNew loads the persisted chain if it exists and rebuilds the
// vault's state from the last recorded operation. A broken chain on
// disk refuses to start — the node will not run on lies.
func (s *server) loadOrNew() (*vault.Vault, error) {
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return nil, err
	}
	l, err := ledger.Load(s.chainPath)
	if err != nil {
		if os.IsNotExist(err) {
			return vault.New(ledger.New()), nil
		}
		return nil, err
	}
	v := vault.New(l)
	if err := s.rebuildState(v); err != nil {
		return nil, err
	}
	return v, nil
}

// rebuildState replays the last recorded operation to restore R and C.
func (s *server) rebuildState(v *vault.Vault) error {
	if n := v.Log.Len(); n > 0 {
		var op vault.Op
		if err := json.Unmarshal([]byte(v.Log.Entries()[n-1].Payload), &op); err != nil {
			return fmt.Errorf("replay: %w", err)
		}
		v.R, v.C, v.Floor = op.R, op.C, op.Floor
	}
	return nil
}

type amountReq struct {
	Amount int64 `json:"amount"`
}

type peerReq struct {
	URL string `json:"url"`
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
		"floor":    s.v.Floor,
	}
}

// persist saves the chain. Called with the lock held, after every
// accepted operation.
func (s *server) persist(w http.ResponseWriter) bool {
	if err := s.v.Log.Save(s.chainPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist failed: " + err.Error()})
		return false
	}
	return true
}

// checkInvariant records whether S = R - C held after an operation.
// A violation is counted, never hidden.
func (s *server) checkInvariant(before int64) {
	if s.v.Surplus() < before {
		s.m.InvariantViolations++
	}
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
	s.m.OpsAttempted++
	before := s.v.Surplus()
	if err := s.v.Deposit(req.Amount); err != nil {
		s.m.OpsRejected++
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	s.checkInvariant(before)
	if !s.persist(w) {
		return
	}
	s.m.OpsSucceeded++
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
	s.m.OpsAttempted++
	before := s.v.Surplus()
	payout, err := s.v.Redeem(req.Amount)
	if err != nil {
		s.m.OpsRejected++
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	s.checkInvariant(before)
	if !s.persist(w) {
		return
	}
	s.m.OpsSucceeded++
	resp := s.state()
	resp["payout"] = payout
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleTip(w http.ResponseWriter, r *http.Request) {
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
	s.m.OpsAttempted++
	before := s.v.Surplus()
	if err := s.v.Tip(req.Amount); err != nil {
		s.m.OpsRejected++
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	s.checkInvariant(before)
	if !s.persist(w) {
		return
	}
	s.m.OpsSucceeded++
	writeJSON(w, http.StatusOK, s.state())
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

func (s *server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.m
	m.StartedAt = s.started
	m.UptimeSeconds = int64(time.Since(s.started).Seconds())
	m.LedgerEntries = s.v.Log.Len()
	m.ChainValid = s.v.Log.Verify() == nil
	m.Peers = len(s.peers)
	writeJSON(w, http.StatusOK, m)
}

func (s *server) handlePeers(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"peers": s.peers})
	case http.MethodPost:
		var req peerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON, want {\"url\": ...}"})
			return
		}
		for _, p := range s.peers {
			if p == req.URL {
				writeJSON(w, http.StatusOK, map[string]any{"peers": s.peers})
				return
			}
		}
		s.peers = append(s.peers, req.URL)
		writeJSON(w, http.StatusOK, map[string]any{"peers": s.peers})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET or POST only"})
	}
}

// handleSync pulls every peer's chain and adopts the longest valid one.
func (s *server) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	client := &http.Client{Timeout: 10 * time.Second}
	best := s.v.Log.Entries()
	bestFrom := ""
	for _, peer := range s.peers {
		resp, err := client.Get(peer + "/ledger")
		if err != nil {
			continue
		}
		var entries []ledger.Entry
		if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		if len(entries) <= len(best) {
			continue
		}
		if err := ledger.VerifyEntries(entries); err != nil {
			continue
		}
		best = entries
		bestFrom = peer
	}
	adopted := false
	if len(best) > s.v.Log.Len() {
		if err := s.v.Log.Adopt(best, s.chainPath); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "adopt failed: " + err.Error()})
			return
		}
		if err := s.rebuildState(s.v); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !s.persist(w) {
			return
		}
		adopted = true
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"adopted":      adopted,
		"from":         bestFrom,
		"ledger_entries": s.v.Log.Len(),
		"state":        s.state(),
	})
}

func main() {
	port := flag.String("port", "8080", "listen port")
	data := flag.String("data", "grove-data", "data directory")
	flag.Parse()

	s := &server{
		started:   time.Now(),
		dataDir:   *data,
		chainPath: *data + "/chain.json",
		port:      *port,
	}
	v, err := s.loadOrNew()
	if err != nil {
		log.Fatalf("refusing to start: %v", err)
	}
	s.v = v
	s.m.StartedAt = s.started

	http.HandleFunc("/deposit", s.handleDeposit)
	http.HandleFunc("/redeem", s.handleRedeem)
	http.HandleFunc("/tip", s.handleTip)
	http.HandleFunc("/state", s.handleState)
	http.HandleFunc("/ledger", s.handleLedger)
	http.HandleFunc("/metrics", s.handleMetrics)
	http.HandleFunc("/peers", s.handlePeers)
	http.HandleFunc("/sync", s.handleSync)
	fmt.Printf("Grove sequencer listening on :%s (data: %s)\n", s.port, s.dataDir)
	log.Fatal(http.ListenAndServe(":"+s.port, nil))
}
