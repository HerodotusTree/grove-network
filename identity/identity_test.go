package identity

import (
	"strings"
	"testing"
)

func TestRoundtrip(t *testing.T) {
	priv, pub, err := Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(priv) != 64 || len(pub) != 64 {
		t.Fatalf("want 64 hex chars each, got priv=%d pub=%d", len(priv), len(pub))
	}
	p := Payload(OpDeposit, pub, 100, 1)
	sig, err := Sign(priv, p)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if !Verify(pub, p, sig) {
		t.Fatal("valid signature did not verify")
	}
}

func TestPayloadFormat(t *testing.T) {
	got := Payload(OpRedeem, "abcd", 25, 7)
	want := "grove-v1|redeem|abcd|25|7"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTamperedPayloadFails(t *testing.T) {
	priv, pub, _ := Generate()
	sig, _ := Sign(priv, Payload(OpDeposit, pub, 100, 1))
	// Change the amount: the signature must not follow it.
	if Verify(pub, Payload(OpDeposit, pub, 999, 1), sig) {
		t.Fatal("signature verified against a tampered amount")
	}
	// Change the op: a deposit signature must not authorize a redeem.
	if Verify(pub, Payload(OpRedeem, pub, 100, 1), sig) {
		t.Fatal("deposit signature verified as a redeem")
	}
	// Change the nonce: no replay with a different nonce.
	if Verify(pub, Payload(OpDeposit, pub, 100, 2), sig) {
		t.Fatal("signature verified against a different nonce")
	}
	// Change the account: a signature must not move to another account.
	_, pub2, _ := Generate()
	if Verify(pub2, Payload(OpDeposit, pub, 100, 1), sig) {
		t.Fatal("signature verified under a different account")
	}
}

func TestTamperedSigFails(t *testing.T) {
	priv, pub, _ := Generate()
	p := Payload(OpTip, pub, 5, 3)
	sig, _ := Sign(priv, p)
	// Flip the last hex digit.
	bad := sig[:len(sig)-1] + "0"
	if bad == sig {
		bad = sig[:len(sig)-1] + "1"
	}
	if Verify(pub, p, bad) {
		t.Fatal("tampered signature verified")
	}
}

func TestMalformedInputsFailCleanly(t *testing.T) {
	priv, pub, _ := Generate()
	p := Payload(OpDeposit, pub, 100, 1)
	sig, _ := Sign(priv, p)

	cases := map[string]struct{ account, payload, sg string }{
		"empty account":      {"", p, sig},
		"non-hex account":    {"zzzz", p, sig},
		"short account":      {"abcd", p, sig},
		"long account":       {strings.Repeat("ab", 33), p, sig},
		"empty signature":    {pub, p, ""},
		"non-hex signature":  {pub, p, "zzzz"},
		"short signature":    {pub, p, "abcd"},
		"empty payload":      {pub, "", sig},
		"all empty":          {"", "", ""},
		"odd-length hex sig": {pub, p, sig[:len(sig)-1]},
	}
	for name, c := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s: Verify panicked: %v", name, r)
				}
			}()
			if Verify(c.account, c.payload, c.sg) {
				t.Fatalf("%s: malformed input verified", name)
			}
		}()
	}
}

func TestHexCaseInsensitive(t *testing.T) {
	priv, pub, _ := Generate()
	p := Payload(OpDeposit, pub, 100, 1)
	sig, _ := Sign(priv, p)
	// Uppercase hex decodes to the same key bytes: same key, still valid.
	if !Verify(strings.ToUpper(pub), p, sig) {
		t.Fatal("uppercase hex of the same account did not verify")
	}
}

func TestSignRejectsBadKey(t *testing.T) {
	if _, err := Sign("zzzz", "payload"); err == nil {
		t.Fatal("Sign accepted non-hex key")
	}
	if _, err := Sign("abcd", "payload"); err == nil {
		t.Fatal("Sign accepted short key")
	}
	// 64 hex chars decode to 32 bytes only if valid hex; 128 zeros = 64 bytes, wrong size.
	if _, err := Sign(strings.Repeat("0", 128), "payload"); err == nil {
		t.Fatal("Sign accepted wrong-length key")
	}
}

func TestKeysAreUnique(t *testing.T) {
	_, pub1, _ := Generate()
	_, pub2, _ := Generate()
	if pub1 == pub2 {
		t.Fatal("two generated accounts collided")
	}
}
