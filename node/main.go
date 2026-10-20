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
// Peers: POST /peers {"url":"http://host:port"} registers a peer. The URL
// must be a bare http(s) base URL — no path, query, fragment, or
// userinfo — because sync appends "/ledger" to it verbatim; anything
// else is rejected at registration (issue #7). POST /sync pulls each
// peer's chain and adopts the longest valid one, without holding the op
// lock during the network fetches (issue #6). A chain that fails
// verification is refused, never merged — no trust required, only math.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
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

// applyGenesis binds the node to its network. The genesis payload
// comes from the operator's genesis config file (ruling 2: genesis
// config is a file). On a fresh chain it writes entry 0; on an
// existing chain it requires the chain to belong to the configured
// genesis (CheckDomain) - a chain with the wrong genesis file refuses
// to start (A3). The vault stamps the resulting domain into every
// entry it records.
func (s *server) applyGenesis(payload string) error {
	probe := ledger.New()
	g, err := probe.AppendGenesis(payload)
	if err != nil {
		return err
	}
	domain := g.Hash
	if s.v.Log.Len() == 0 {
		if _, err := s.v.Log.AppendGenesis(payload); err != nil {
			return err
		}
		s.v.Domain = domain
		return s.v.Log.Save(s.chainPath)
	}
	if err := ledger.CheckDomain(s.v.Log.Entries(), domain); err != nil {
		return err
	}
	s.v.Domain = domain
	return nil
}

// rebuildState restores the vault by replaying the whole ledger from
// genesis (vault.Rebuild): per-account balances and R/C/Floor are
// derived from the authenticated entries, and a chain whose recorded
// numbers do not recompute refuses to start. The v0.3 shortcut -
// trusting the last entry's recorded state - is gone: it let a single
// forged entry rewrite the vault's reality.
func (s *server) rebuildState(v *vault.Vault) error {
	return v.Rebuild()
}

// signedReq is the shape of every mutating request. The account is the
// hex Ed25519 public key; the signature covers the canonical payload
// grove-v2|<domain>|<op>|<account>|<amount>|<nonce> so a signature can
// never be moved to a different op, amount, account, replayed nonce,
// or network.
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

// deriveNonces rebuilds the nonce table from the ledger: every entry
// carries its account and nonce, so the last entry per account sets
// the table. The ledger is the only durable authorization record -
// there is no nonce file (ATOM-01).
func (s *server) deriveNonces() {
	s.nonces = map[string]uint64{}
	for _, e := range s.v.Log.Entries() {
		if e.Account != "" {
			s.nonces[e.Account] = e.Nonce
		}
	}
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
	payload := identity.PayloadFor(s.v.Domain, op, req.Account, req.Amount, req.Nonce)
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

// commitNonce advances the account's nonce cache after a fully
// accepted and persisted operation. The cache is not the record: the
// ledger entry itself carries the nonce, and startup rebuilds the
// cache from the chain (deriveNonces). There is no second write, so
// chain and nonce state can never disagree on disk (ATOM-01).
func (s *server) commitNonce(w http.ResponseWriter, account string, nonce uint64) bool {
	s.nonces[account] = nonce
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
	if err := s.v.DepositFor(req.Account, req.Nonce, req.Signature, req.Amount); err != nil {
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
	payout, err := s.v.RedeemFor(req.Account, req.Nonce, req.Signature, req.Amount)
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
	if err := s.v.TipFor(req.Account, req.Nonce, req.Signature, req.Amount); err != nil {
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
	switch r.Method {
	case http.MethodGet:
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"peers": s.peers})
	case http.MethodPost:
		// Decode and validate BEFORE taking the op lock. The request
		// body is network I/O: holding s.mu across it would let any
		// unauthenticated client freeze deposits, redeems, tips,
		// /state and /metrics with a stalled body (issue #6 class).
		// Same pattern as the signed handlers below.
		var req peerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON, want {\"url\": ...}"})
			return
		}
		// Issue #7: the peer URL is attacker-influenced input that the
		// node later fetches on demand, so it is validated up front and
		// stored in canonical form.
		canonical, err := validatePeerURL(req.URL)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, p := range s.peers {
			if p == canonical {
				writeJSON(w, http.StatusOK, map[string]any{"peers": s.peers})
				return
			}
		}
		s.peers = append(s.peers, canonical)
		writeJSON(w, http.StatusOK, map[string]any{"peers": s.peers})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET or POST only"})
	}
}

// validatePeerURL enforces the peer contract {"url":"http://host:port"}.
// handleSync appends "/ledger" to the registered string verbatim, so the
// demonstrated SSRF (issue #7) rode on a URL path prefix that steered the
// fetch to an attacker-chosen destination. Registration therefore
// accepts only a bare http(s) base URL and rejects:
//   - non-http(s) schemes (gopher/file/ftp have no business here),
//   - userinfo (user:pass@ smuggling and credential leakage),
//   - fragments and queries (never part of the fetch target, but they
//     make logs and dedup lie),
//   - non-empty paths (the prefix that carried the demonstrated attack).
//
// It returns the canonical base URL, so "http://h/" and "http://h"
// dedup to the same peer. This is deliberately syntactic: it closes the
// demonstrated vector, not all SSRF (a bare http://internal-host:port is
// still fetchable). Full peer authorization policy is AUTH-02's call;
// defense in depth (revalidation at fetch time, no redirect following)
// sits below.
func validatePeerURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("bad peer URL: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("bad peer URL: scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("bad peer URL: missing host")
	}
	if u.Opaque != "" {
		return "", fmt.Errorf("bad peer URL: opaque URL not allowed")
	}
	if u.User != nil {
		return "", fmt.Errorf("bad peer URL: userinfo not allowed")
	}
	if u.Fragment != "" || u.RawFragment != "" {
		return "", fmt.Errorf("bad peer URL: fragment not allowed")
	}
	if u.RawQuery != "" {
		return "", fmt.Errorf("bad peer URL: query not allowed")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("bad peer URL: path not allowed, want a bare http://host:port base URL")
	}
	u.Path, u.RawPath = "", ""
	u.Fragment, u.RawFragment = "", ""
	u.RawQuery = ""
	// A bare "?" sets ForceQuery, which u.String() would preserve as a
	// trailing "?" — registering "http://h" and "http://h?" as two
	// peers and, worse, turning the fetch into "http://h?/ledger"
	// (GET / with query "/ledger") instead of the contracted
	// GET /ledger. A bare base URL never carries a query marker.
	u.ForceQuery = false
	return u.String(), nil
}

// fetchPeerLedger GETs peer+"/ledger" and decodes the entry list. Any
// network error, non-200 status, or undecodable body means the peer is
// skipped — sync never trusts the network, it only adopts a longer chain
// that verifies.
func fetchPeerLedger(client *http.Client, peer string) ([]ledger.Entry, error) {
	resp, err := client.Get(peer + "/ledger")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer %s: unexpected status %d", peer, resp.StatusCode)
	}
	var entries []ledger.Entry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// handleSync pulls every peer's chain and adopts the longest valid one.
//
// Hardened (issues #6, #7): the peer list and the local chain length are
// snapshotted under the op lock, but the lock is NOT held during the
// outbound peer fetches — a slow or malicious peer can no longer freeze
// deposits, redeems, tips, /state or /metrics (#6). Fetches never follow
// redirects, so a peer cannot 302 the node onto an internal destination
// (#7). Adoption happens back under the lock, and only if nothing was
// appended locally while the fetch was in flight: a longer peer chain
// must never silently discard operations that landed during the network
// wait.
func (s *server) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	s.mu.Lock()
	peers := append([]string(nil), s.peers...)
	localEntries := s.v.Log.Entries()
	localLen := len(localEntries)
	domain := s.v.Domain
	s.mu.Unlock()

	// The op lock is released from here until adoption. No code path
	// below does network I/O while holding s.mu.
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	var best []ledger.Entry
	bestFrom := ""
	for _, peer := range peers {
		// Defense in depth: never fetch a peer we would not register
		// today (stored before validation existed, or injected past the
		// handler) — skip it instead.
		if _, err := validatePeerURL(peer); err != nil {
			continue
		}
		entries, err := fetchPeerLedger(client, peer)
		if err != nil {
			continue
		}
		if len(entries) <= localLen || len(entries) <= len(best) {
			continue
		}
		if err := ledger.VerifyEntries(entries); err != nil {
			continue
		}
		// Phase 1 adoption gate (rulings 9 + 14): ADOPTABLE is a
		// subset of BOOTABLE. The candidate must belong to this
		// network (CheckDomain), must extend our own history
		// without rewriting it (prefix extension), and must replay
		// cleanly through the full transition rules on a scratch
		// vault (Rebuild) - all before it comes near the live chain.
		if err := ledger.CheckDomain(entries, domain); err != nil {
			continue
		}
		if !ledger.IsPrefixExtension(localEntries, entries) {
			continue
		}
		scratch := vault.New(ledger.FromEntries(entries))
		if err := scratch.Rebuild(); err != nil {
			continue
		}
		best = entries
		bestFrom = peer
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	adopted := false
	if len(best) > localLen && s.v.Log.Len() == localLen {
		if err := s.v.Log.Adopt(best, s.chainPath); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "adopt failed: " + err.Error()})
			return
		}
		if err := s.rebuildState(s.v); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.deriveNonces()
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

// handleStatus serves STATUS-01: a read-only, informational view of
// one account - its balance and consumed nonce - plus the node's
// current canonical ledger position (height and tip hash). It
// authorizes nothing; it only reports what the ledger proves.
func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	account := r.URL.Query().Get("account")
	if account == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account query parameter required"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tip := ""
	if n := s.v.Log.Len(); n > 0 {
		tip = s.v.Log.Entries()[n-1].Hash
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account": account,
		"balance": s.v.Balances[account],
		"nonce":   s.nonces[account],
		"height":  s.v.Log.Len(),
		"tip":     tip,
		"domain":  s.v.Domain,
	})
}

func main() {
	port := flag.String("port", "8080", "listen port")
	data := flag.String("data", "grove-data", "data directory")
	keygen := flag.Bool("keygen", false, "generate an Ed25519 keypair and exit")
	genesis := flag.String("genesis", "", "path to the genesis config file (required)")
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

	if *genesis == "" {
		log.Fatalf("refusing to start: -genesis <file> is required; a Phase 1 node has no default network")
	}
	rawGenesis, err := os.ReadFile(*genesis)
	if err != nil {
		log.Fatalf("refusing to start: read genesis: %v", err)
	}
	genesisPayload := strings.TrimSpace(string(rawGenesis))

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
	if err := s.applyGenesis(genesisPayload); err != nil {
		log.Fatalf("refusing to start: %v", err)
	}
	s.deriveNonces()
	s.m.StartedAt = s.started

	http.HandleFunc("/deposit", s.handleDeposit)
	http.HandleFunc("/redeem", s.handleRedeem)
	http.HandleFunc("/tip", s.handleTip)
	http.HandleFunc("/state", s.handleState)
	http.HandleFunc("/status", s.handleStatus)
	http.HandleFunc("/ledger", s.handleLedger)
	http.HandleFunc("/metrics", s.handleMetrics)
	http.HandleFunc("/peers", s.handlePeers)
	http.HandleFunc("/sync", s.handleSync)
	fmt.Printf("Grove sequencer listening on :%s (data: %s)\n", s.port, s.dataDir)
	log.Fatal(http.ListenAndServe(":"+s.port, nil))
}
