package identity

import "testing"

// Phase 1 step 3: the v2 payload binds the network domain, and the format
// break is clean - a signature over the v2 string must not verify against
// the legacy v1 string, nor against a v2 string for a different domain.
func TestPayloadV2DomainBindingAndCleanBreak(t *testing.T) {
	priv, pub, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	p2 := PayloadFor("genesisABC", OpDeposit, pub, 100, 1)
	sig, err := Sign(priv, p2)
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(pub, p2, sig) {
		t.Fatal("v2 signature should verify against the v2 payload")
	}
	p1 := Payload(OpDeposit, pub, 100, 1)
	if Verify(pub, p1, sig) {
		t.Fatal("v2 signature must not verify against the legacy v1 payload")
	}
	other := PayloadFor("genesisXYZ", OpDeposit, pub, 100, 1)
	if Verify(pub, other, sig) {
		t.Fatal("signature must not cross domains")
	}
}
