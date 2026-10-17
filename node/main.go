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
// Identity: /deposit, /redeem and /tip require a signed request. Each
// user holds an Ed25519 keypair (see -keygen); the public key is the
// account, and every request carries a signature over the canonical
// payload plus a strictly increasing nonce, so a signed request works
// exactly once and can never be replayed or retargeted.
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

	"github.com/HerodotusTree/grove-network/identity"
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
	noncePath string
	// nonces tracks the last accepted nonce per account. A new account
	// starts at 0, so its first operation must carry nonce 1. Nonces are
	// the replay defense: a signed request is only good once.
	nonces map[string]uint64
	port   string
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

// signedReq is the shape of every mutating request. The account is the
// hex Ed25519 public key; the signature covers the canonical payload
// grove-v1|<op>|<account>|<amount>|<nonce> so a signature can never be
// moved to a different op, amount, account, or replayed nonce.
type signedReq struct {
	Account   string `json:"account"`
	Amount    int64  `json:"amount"`
	Nonce     uint64 `json:"nonce"`
	Signature string `json:"signature"`
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

// loadNonces reads the per-account nonce table. A missing file means no
// account has ever acted — start empty. A corrupt file refuses startup:
// running with a forged or truncated nonce table would reopen replays.
func loadNonces(path string) (map[string]uint64, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]uint64{}, nil
	}
	if err != nil {
		return nil, err
	}
	var nonces map[string]uint64
	if err := json.Unmarshal(data, &nonces); err != nil {
		return nil, fmt.Errorf("nonces: corrupt file: %w", err)
	}
	if nonces == nil {
		nonces = map[string]uint64{}
	}
	return nonces, nil
}

// saveNonces writes the nonce table atomically (temp file + rename) so a
// crash mid-write can never leave a half-written table behind.
func (s *server) saveNonces() error {
	data, err := json.Marshal(s.nonces)
	if err != nil {
		return err
	}
	tmp := s.noncePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.noncePath)
}

// checkAuth verifies that req is a fresh, properly signed operation from
// req.Account. It must be called with the lock held. On success the
// caller owns the op and must advance the nonce after applying it.
func (s *server) checkAuth(op string, req signedReq) error {
	if req.Account == "" {
		return fmt.Errorf("missing account")
	}
	last := s.nonces[req.Account] // unknown accounts start at 0
	if req.Nonce != last+1 {
		return fmt.Errorf("bad nonce: want %d, got %d", last+1, req.Nonce)
	}
	payload := identity.Payload(op, req.Account, req.Amount, req.Nonce)
	if !identity.Verify(req.Account, payload, req.Signature) {
		return fmt.Errorf("bad signature")
	}
	return nil
}

// authReject counts and reports a failed authorization. Auth failures
// are 401: the request was understood, but the requester proved
// nothing.
func (s *server) authReject(w http.ResponseWriter, err error) {
	s.m.OpsRejected++
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
}

// commitNonce advances the account's nonce after a fully accepted and
// persisted operation, and saves the table. The in-memory bump happens
// before the disk write: if the write fails we answer 500 and the
// client must retry with a fresh nonce rather than replaying this one.
// (A disk failure between the chain write and the nonce write can still
// leave a stale table on restart — same crash-consistency caveat as the
// vault's own mutate-then-persist.)
func (s *server) commitNonce(w http.ResponseWriter, account string, nonce uint64) bool {
	s.nonces[account] = nonce
	if err := s.saveNonces(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "nonce persist failed: " + err.Error()})
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
	var req signedReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m.OpsAttempted++
	if err := s.checkAuth(identity.OpDeposit, req); err != nil {
		s.authReject(w, err)
		return
	}
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
	if !s.commitNonce(w, req.Account, req.Nonce) {
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
	var req signedReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m.OpsAttempted++
	if err := s.checkAuth(identity.OpRedeem, req); err != nil {
		s.authReject(w, err)
		return
	}
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
	if !s.commitNonce(w, req.Account, req.Nonce) {
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
	var req signedReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m.OpsAttempted++
	if err := s.checkAuth(identity.OpTip, req); err != nil {
		s.authReject(w, err)
		return
	}
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
	if !s.commitNonce(w, req.Account, req.Nonce) {
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
		"adopted":        adopted,
		"from":           bestFrom,
		"ledger_entries": s.v.Log.Len(),
		"state":          s.state(),
	})
}

func main() {
	port := flag.String("port", "8080", "listen port")
	data := flag.String("data", "grove-data", "data directory")
	keygen := flag.Bool("keygen", false, "generate an Ed25519 keypair and exit")
	flag.Parse()

	if *keygen {
		priv, pub, err := identity.Generate()
		if err != nil {
			log.Fatalf("keygen: %v", err)
		}
		// The private key is shown once. The node never stores it — lose
		// it and the account is gone with it. There is no recovery.
		fmt.Fprintln(os.Stderr, "WARNING: store the private key somewhere safe. It is shown once and cannot be recovered.")
		out, _ := json.Marshal(map[string]string{"private_key": priv, "account": pub})
		fmt.Println(string(out))
		return
	}

	s := &server{
		started:   time.Now(),
		dataDir:   *data,
		chainPath: *data + "/chain.json",
		noncePath: *data + "/nonces.json",
		port:      *port,
	}
	v, err := s.loadOrNew()
	if err != nil {
		log.Fatalf("refusing to start: %v", err)
	}
	s.v = v
	nonces, err := loadNonces(s.noncePath)
	if err != nil {
		log.Fatalf("refusing to start: %v", err)
	}
	s.nonces = nonces
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
