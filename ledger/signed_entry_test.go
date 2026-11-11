package ledger

import "testing"

// Phase 1 step 2: a signed entry's identity fields are covered by its hash.
// Tampering with the signature (or account, or nonce) after the fact must
// break verification - that is what makes the ledger preserve evidence.
func TestSignedEntryTamperBreaksVerify(t *testing.T) {
	l := New()
	l.Append("genesis")
	e := l.AppendSigned("domhash", "acctkey", 1, "deposit|100", "sigbytes")
	if e.Domain != "domhash" || e.Account != "acctkey" || e.Nonce != 1 || e.Signature != "sigbytes" {
		t.Fatalf("signed fields not stored: %+v", e)
	}
	if err := l.Verify(); err != nil {
		t.Fatalf("clean chain should verify: %v", err)
	}
	l.entries[1].Signature = "forged"
	if err := l.Verify(); err == nil {
		t.Fatal("tampered signature must fail verification")
	}
}
