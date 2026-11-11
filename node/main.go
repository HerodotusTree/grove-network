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
	"io"
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
	mu      sync.Mutex
	v       *vault.Vault
	started time.Time
	m       Metrics
	peers   []string
	// untrustedPeers holds peers registered by unauthenticated POST
	// /peers. They are listed and fetched like trusted peers, but
	// they can never supply a fresh node's first history: only
	// operator-configured peers (peers, via -trusted-peer) can
	// bootstrap a node that holds no history beyond genesis
	// (AUTH-02 bootstrap trust, Proof Log Entry 22). Fetch is not
	// adoption: an untrusted peer's chain is still pulled and
	// checked, it just cannot become a fresh node's state.
	untrustedPeers []string
	// trustedGenesis, when non-empty, is the operator's out-of-band
	// entry-0 hash a bootstrap candidate must carry. It is a second
	// pin beside the genesis config file, which already pins the
	// domain through CheckDomain; the flag pins the candidate's
	// entry-0 hash independently of the local config.
	trustedGenesis string
	dataDir        string
	chainPath      string
	// nonces tracks the last accepted nonce per account. A new account
	// starts at 0, so its first operation must carry nonce 1. Nonces are
	// the replay defense: a signed request is only good once.
	nonces map[string]uint64
	port   string
	// role is "sequencer" (the one writer) or "follower" (read-only,
	// pulls only from the sequencer). Empty means sequencer: the
	// single-node trial default. Set from -role in main.
	role string
	// sequencerURL is a follower's configured pull source (-sequencer).
	sequencerURL string
	// nodeClass and nodeName are the node's declared identity from
	// its genesis config file ("node_class": "personal"|"business",
	// "node_name": free text) - the declaration the node web page
	// exists to state. Empty nodeClass means undeclared: never
	// assumed, never defaulted. See web/ and INTEGRATION.md.
	nodeClass string
	nodeName  string
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
// genesisConfig is the node's view of the genesis config file. The
// payload itself stays opaque to the ledger - entry 0 stores it
// verbatim and its hash is the domain - but the node reads its own
// declared identity (class and name) out of the same file, so the
// declaration lives in exactly one place: the operator's config.
type genesisConfig struct {
	NodeClass string `json:"node_class"`
	NodeName  string `json:"node_name"`
}

// parseGenesisConfig reads the declared identity out of a genesis
// payload. It never fails: a payload that is not JSON, or a config
// without the fields, simply declares nothing.
func parseGenesisConfig(payload string) genesisConfig {
	var cfg genesisConfig
	_ = json.Unmarshal([]byte(payload), &cfg)
	return cfg
}

func (s *server) applyGenesis(payload string) error {
	// The declared identity travels with the config file, whatever
	// path startup takes from here. The payload passed to the
	// ledger below is the operator's bytes, untouched.
	cfg := parseGenesisConfig(payload)
	s.nodeClass, s.nodeName = cfg.NodeClass, cfg.NodeName
	probe := ledger.New()
	g, err := probe.AppendGenesis(payload)
	if err != nil {
		return err
	}
	domain := g.Hash
	if s.v.Log.Len() == 0 {
		if s.role == "follower" {
			// Followers never create a genesis (P5): entry 0 is
			// adopted from the sequencer. Until then the node is
			// "awaiting entry 0" - domain known from the config
			// file, chain empty, writes refused by role (N5).
			s.v.Domain = domain
			return nil
		}
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

// acceptWrites enforces the sequencer role (N1): exactly one node
// writes. Followers refuse every mutating operation - their chain
// changes only by adopting the sequencer's history through /sync.
func (s *server) acceptWrites(w http.ResponseWriter) bool {
	if s.role == "follower" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "follower nodes are read-only; writes go to the sequencer"})
		return false
	}
	return true
}

func (s *server) handleDeposit(w http.ResponseWriter, r *http.Request) {
	if !s.acceptWrites(w) {
		return
	}
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
	if !s.acceptWrites(w) {
		return
	}
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
	if !s.acceptWrites(w) {
		return
	}
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
	m.Peers = len(s.peers) + len(s.untrustedPeers)
	writeJSON(w, http.StatusOK, m)
}

// allPeers joins the trusted and untrusted peer lists for listing
// and fetching. Trust is decided at adoption, not at listing.
func allPeers(trusted, untrusted []string) []string {
	out := make([]string, 0, len(trusted)+len(untrusted))
	return append(append(out, trusted...), untrusted...)
}

func (s *server) handlePeers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"peers": allPeers(s.peers, s.untrustedPeers)})
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
		for _, p := range allPeers(s.peers, s.untrustedPeers) {
			if p == canonical {
				writeJSON(w, http.StatusOK, map[string]any{"peers": allPeers(s.peers, s.untrustedPeers)})
				return
			}
		}
		// Unauthenticated registrants are untrusted by construction:
		// they may be fetched, but they can never bootstrap a fresh
		// node (see handleSync's bootstrap rule).
		s.untrustedPeers = append(s.untrustedPeers, canonical)
		writeJSON(w, http.StatusOK, map[string]any{"peers": allPeers(s.peers, s.untrustedPeers)})
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
// Sync fetch bounds (issue #13, ported from the daily cycle's
// Oct 24 hardening pass).
const (
	maxPeerLedgerBytes   = 1 << 20 // 1 MiB of peer /ledger body
	maxPeerLedgerEntries = 10000
)

func fetchPeerLedger(client *http.Client, peer string) ([]ledger.Entry, error) {
	resp, err := client.Get(peer + "/ledger")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer %s: unexpected status %d", peer, resp.StatusCode)
	}
	// PORTED from the daily cycle's Oct 24 sync hardening (issue #13):
	// a peer's /ledger body is capped before decode, and the decoded
	// chain length is capped as well. Test-scale sizing, not a
	// production capacity claim.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPeerLedgerBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPeerLedgerBytes {
		return nil, fmt.Errorf("peer %s: ledger body %d bytes exceeds %d-byte cap", peer, len(body), maxPeerLedgerBytes)
	}
	var entries []ledger.Entry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}
	if len(entries) > maxPeerLedgerEntries {
		return nil, fmt.Errorf("peer %s: ledger has %d entries, exceeds %d-entry cap", peer, len(entries), maxPeerLedgerEntries)
	}
	return entries, nil
}

// verifyExtensionSignatures checks the authorship of candidate
// entries beyond the local history: each one must be a signed op
// whose signature verifies against this network's domain, exactly
// as checkAuth would demand had the op been submitted directly.
// An unsigned extension entry, or one whose signature was made for
// a different domain, refuses the whole candidate.
func verifyExtensionSignatures(extension []ledger.Entry, domain string) error {
	for _, e := range extension {
		var op vault.Op
		if err := json.Unmarshal([]byte(e.Payload), &op); err != nil {
			return fmt.Errorf("entry %d: unparseable payload: %w", e.Index, err)
		}
		if op.Kind == "genesis" {
			continue
		}
		if e.Account == "" || e.Signature == "" {
			return fmt.Errorf("entry %d: unsigned extension entry", e.Index)
		}
		if !identity.Verify(e.Account, identity.PayloadFor(domain, op.Kind, e.Account, op.Amount, e.Nonce), e.Signature) {
			return fmt.Errorf("entry %d: signature does not verify for this domain", e.Index)
		}
	}
	return nil
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
	peers := allPeers(s.peers, s.untrustedPeers)
	trusted := map[string]bool{}
	for _, p := range s.peers {
		trusted[p] = true
	}
	if s.role == "follower" {
		// Followers pull only from the configured sequencer (N1):
		// peer registrations mean nothing to them. The sequencer
		// is operator-configured, so it is trusted by construction.
		peers = []string{s.sequencerURL}
		trusted[s.sequencerURL] = true
	}
	trustedGenesis := s.trustedGenesis
	localEntries := s.v.Log.Entries()
	localLen := len(localEntries)
	// Branch chains always begin with the genesis entry, so len <= 1
	// means the node holds no history beyond genesis: len 0 is a
	// follower awaiting entry 0, len 1 is genesis only. Only fresh
	// nodes are governed by the bootstrap rule below.
	fresh := localLen <= 1
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
		// PORTED from the daily cycle's Oct 25 adoption gate (issue
		// #1, Proof Log Entry 18): Rebuild replays the ops but never
		// checks their signatures, so a signature captured on another
		// network could be re-homed into an entry stamped with this
		// network's domain and adopted. Every extension entry beyond
		// our own history must carry a signature that verifies
		// against THIS network's domain (grove-v2 payload binding).
		if err := verifyExtensionSignatures(entries[localLen:], domain); err != nil {
			continue
		}
		// AUTH-02 adoption trust (Proof Log Entries 22 and 23): a
		// node adopts ONLY from a trusted peer, at any height.
		// Entry 22 was the bootstrap form: a node holding no
		// history beyond genesis must not adopt a stranger's fully
		// self-signed history — it verifies against the public
		// domain, because the gate authenticates entries, and they
		// ARE authenticated, by the stranger himself. Entry 23 is
		// the same hole one step later: a non-empty node adopting
		// a stranger's genuinely signed but unfunded extension and
		// ratcheting its floor. A signature proves an account
		// authorized an op, not that the op was funded or that the
		// peer is honest, so peer trust — not signature validity —
		// is the adoption decision at every height. Untrusted
		// peers are still fetched and listed (fetching is not
		// adoption); their candidates simply never win. Followers
		// are unaffected: their only peer is the
		// operator-configured sequencer, trusted by construction
		// above.
		if !trusted[peer] {
			continue
		}
		if fresh && trustedGenesis != "" && entries[0].Hash != trustedGenesis {
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
		// The node's declared identity, from its genesis config.
		// Additive: the account view above is unchanged. The node
		// web page reads its class from here (INTEGRATION.md).
		"node_class": s.nodeClass,
		"node_name":  s.nodeName,
	})
}

// stringListFlag collects a repeatable string flag (-trusted-peer).
type stringListFlag []string

func (f *stringListFlag) String() string { return strings.Join(*f, ",") }
func (f *stringListFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

func main() {
	port := flag.String("port", "8080", "listen port")
	data := flag.String("data", "grove-data", "data directory")
	keygen := flag.Bool("keygen", false, "generate an Ed25519 keypair and exit")
	genesis := flag.String("genesis", "", "path to the genesis config file (required)")
	role := flag.String("role", "sequencer", "node role: sequencer (the one writer) or follower (read-only)")
	sequencer := flag.String("sequencer", "", "follower only: base URL of the sequencer to pull from")
	trustedPeers := stringListFlag{}
	flag.Var(&trustedPeers, "trusted-peer", "operator-trusted peer base URL (repeatable); only trusted peers can supply a fresh node's first history")
	trustedGenesis := flag.String("trusted-genesis", "", "entry-0 hash a bootstrap candidate must carry (empty: any trusted peer's genesis)")
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
	if *role != "sequencer" && *role != "follower" {
		log.Fatalf("refusing to start: -role must be sequencer or follower, got %q", *role)
	}
	if *role == "follower" && *sequencer == "" {
		log.Fatalf("refusing to start: follower role requires -sequencer <url>, its only pull source")
	}
	rawGenesis, err := os.ReadFile(*genesis)
	if err != nil {
		log.Fatalf("refusing to start: read genesis: %v", err)
	}
	genesisPayload := strings.TrimSpace(string(rawGenesis))

	// Trusted peers are canonicalized at startup, the same way
	// registrations are canonicalized at POST /peers: a trailing
	// slash must never make a trusted peer unrecognizable at
	// adoption time.
	canonicalTrusted := make([]string, 0, len(trustedPeers))
	for _, p := range trustedPeers {
		c, err := validatePeerURL(p)
		if err != nil {
			log.Fatalf("refusing to start: -trusted-peer %q: %v", p, err)
		}
		canonicalTrusted = append(canonicalTrusted, c)
	}

	s := &server{
		started:        time.Now(),
		dataDir:        *data,
		chainPath:      *data + "/chain.json",
		port:           *port,
		role:           *role,
		sequencerURL:   *sequencer,
		peers:          canonicalTrusted,
		trustedGenesis: *trustedGenesis,
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
	// Node web page (class declaration, terms, live state), served
	// from the web/ folder. Pattern specificity, not registration
	// order, decides: Go's mux prefers the exact endpoint patterns
	// above, so "/" only catches what no endpoint claims. The
	// folder is relative to the node's working directory - run
	// the node from the repo root (or beside a web/ folder). See
	// INTEGRATION.md.
	http.Handle("/", http.FileServer(http.Dir("web")))
	fmt.Printf("Grove sequencer listening on :%s (data: %s)\n", s.port, s.dataDir)
	log.Fatal(http.ListenAndServe(":"+s.port, nil))
}
