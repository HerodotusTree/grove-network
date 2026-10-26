package main

// Attacker pass (oscillation), 2026-10-22 — telemetry and registration
// angles on the hardened server. The #6/#7 sync fixes moved network I/O
// out from under the op lock and pinned the peer-URL contract, but two
// UNAUTHENTICATED endpoints still do attacker-scalable work while
// holding the single global op lock — or store attacker-scalable state
// that /sync then walks. Test assets only — no real value anywhere.

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HerodotusTree/grove-network/identity"
)

// M1. /metrics re-verifies the ENTIRE hash chain (O(n) SHA-256) while
// holding s.mu — the same single global lock that serializes deposits,
// redeems, tips, /state and /ledger. /metrics needs no signature, no
// account, no nonce. The chain grows by one entry per accepted op and is
// never pruned, so the cost of one unauthenticated GET grows forever.
// A client polling /metrics — exactly what the trial's own 99.99%
// monitoring loop does — serializes all sequencer operations behind
// repeated full-chain re-verification. Worse: the trial's efficiency
// telemetry is measured through the very endpoint whose cost is not
// counted anywhere in that telemetry.
//
// Desired: ChainValid is maintained incrementally (or verified outside
// the op lock, or /metrics is bounded/rate-limited) so an
// unauthenticated read can never freeze signed operations.
// Current: one GET /metrics pins the sequencer for ~Verify(n).
func TestAttack_MetricsVerifyHoldsOpLock(t *testing.T) {
	s := newTestServer(t)

	// Grow the chain the way the trial itself does: one entry per op.
	const n = 400000
	for i := 0; i < n; i++ {
		s.v.Log.Append(`{"kind":"tip","amount":1}`)
	}
	if err := s.v.Log.Verify(); err != nil {
		t.Fatalf("setup chain broken: %v", err)
	}

	// Baseline: how fast is a trivial op with no contention?
	priv, pub, _ := identity.Generate()
	base := postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 1, 1))
	if base.Code != http.StatusOK {
		t.Fatalf("baseline deposit: got %d", base.Code)
	}

	// Fire an unauthenticated /metrics, then immediately try a signed
	// deposit and a /state while it runs.
	metricsDone := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		rec := httptest.NewRecorder()
		s.handleMetrics(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("/metrics: got %d", rec.Code)
		}
		metricsDone <- time.Since(start)
	}()
	time.Sleep(50 * time.Millisecond) // let /metrics take the lock first

	start := time.Now()
	drec := postDeposit(s, signReq(t, identity.OpDeposit, priv, pub, 1, 2))
	depositElapsed := time.Since(start)
	if drec.Code != http.StatusOK {
		t.Fatalf("deposit during /metrics: got %d, body %s", drec.Code, drec.Body.String())
	}
	metricsElapsed := <-metricsDone
	t.Logf("/metrics on %d entries took %v; a signed deposit queued behind it took %v",
		n, metricsElapsed.Round(time.Millisecond), depositElapsed.Round(time.Millisecond))

	// The hole: an unauthenticated read froze a signed write for the
	// full duration of an O(chain-length) re-verification. A 250ms
	// threshold is far above any healthy lock hold time for a sequencer
	// (baseline deposit is microseconds-to-milliseconds).
	if depositElapsed < 250*time.Millisecond {
		t.Logf("no significant freeze observed (deposit %v) — machine too fast or chain too short to demonstrate", depositElapsed)
		return
	}
	t.Fatalf("HOLE DEMONSTRATED: unauthenticated GET /metrics held the global op lock for %v "+
		"re-verifying %d entries, freezing a signed deposit for %v. Cost grows with every op "+
		"the trial accepts; the trial's own monitoring loop polling /metrics serializes the "+
		"sequencer behind repeated full-chain verification.",
		metricsElapsed.Round(time.Millisecond), n, depositElapsed.Round(time.Millisecond))
}

// M2. POST /peers is unauthenticated and the peer list has no cap — and
// registration dedups with a LINEAR scan, so each registration costs
// O(len(peers)). /sync then issues one outbound fetch per registered
// peer (10s timeout each). Any network client can register thousands of
// peers and turn one unauthenticated POST /sync into N fetches of
// sequencer work, with N entirely attacker-chosen.
//
// Desired: a bound on the peer table (cap, or at least sublinear dedup)
// and a bound on total sync fetch work. Current: attacker-controlled
// peer count, one fetch per peer per sync, no cap anywhere.
func TestAttack_PeerListUnbounded(t *testing.T) {
	s := newTestServer(t)

	// N distinct local listeners; a shared counter proves how many
	// outbound fetches one /sync performs.
	var fetches atomic.Int64
	const peers = 200
	urls := make([]string, 0, peers)
	for i := 0; i < peers; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fetches.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
		}))
		urls = append(urls, "http://"+ln.Addr().String())
	}

	// Register all N — unauthenticated, no signature anywhere.
	start := time.Now()
	for _, u := range urls {
		body, _ := json.Marshal(peerReq{URL: u})
		rec := httptest.NewRecorder()
		s.handlePeers(rec, httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("register %s: got %d", u, rec.Code)
		}
	}
	regElapsed := time.Since(start)
	if len(s.peers) != peers {
		t.Fatalf("peer table holds %d entries, want %d (uncapped)", len(s.peers), peers)
	}
	t.Logf("registered %d unauthenticated peers in %v (no cap, linear-scan dedup)",
		peers, regElapsed.Round(time.Millisecond))

	// One unauthenticated /sync must touch every registered peer.
	start = time.Now()
	rec := httptest.NewRecorder()
	s.handleSync(rec, httptest.NewRequest(http.MethodPost, "/sync", nil))
	syncElapsed := time.Since(start)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: got %d", rec.Code)
	}
	got := fetches.Load()
	t.Logf("one unauthenticated /sync issued %d outbound fetches in %v", got, syncElapsed.Round(time.Millisecond))
	if got != peers {
		t.Fatalf("expected one fetch per registered peer (%d), got %d", peers, got)
	}

	// Deterministic core of the hole: sync work is linear in an
	// attacker-chosen, uncapped peer count, at up to 10s per peer
	// (client.Timeout) on the drip case. Even instant peers cost one
	// fetch each; there is no bound anywhere in the path.
	t.Fatalf("HOLE DEMONSTRATED: %d unauthenticated peer registrations forced one "+
		"unauthenticated POST /sync to issue %d outbound fetches (%v even for instant "+
		"peers; up to 10s per peer against a drip peer). The peer table is uncapped, "+
		"registration dedups with a linear scan, and sync cost is linear in the "+
		"attacker-chosen count.",
		peers, got, syncElapsed.Round(time.Millisecond))
}
