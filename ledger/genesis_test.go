package ledger

import "testing"

// Phase 1 step 5: the genesis entry defines the network domain, is
// deterministic across nodes, and foreign stamps are rejected.
func TestGenesisDefinesDomain(t *testing.T) {
	l := New()
	g, err := l.AppendGenesis(`{"kind":"genesis","network":"trial-1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if g.Index != 0 || g.Domain != "" {
		t.Fatalf("genesis entry malformed: %+v", g)
	}
	d, ok := l.Domain()
	if !ok || d != g.Hash {
		t.Fatalf("domain = %q, %v; want genesis hash %q", d, ok, g.Hash)
	}
	if _, err := l.AppendGenesis("again"); err == nil {
		t.Fatal("second genesis must be refused")
	}
	l2 := New()
	g2, err := l2.AppendGenesis(`{"kind":"genesis","network":"trial-1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if g2.Hash != g.Hash {
		t.Fatal("same genesis payload must produce the same domain on every node")
	}
	l.AppendSigned(d, "acctkey", 1, "deposit|100", "sigbytes")
	if err := CheckDomain(l.Entries(), d); err != nil {
		t.Fatalf("own chain should pass the domain check: %v", err)
	}
	if err := CheckDomain(l.Entries(), "foreign-domain"); err == nil {
		t.Fatal("foreign domain must fail the check")
	}
	l.entries[1].Domain = "forged"
	if err := CheckDomain(l.Entries(), d); err == nil {
		t.Fatal("an entry stamped with a foreign domain must fail the check")
	}
}
