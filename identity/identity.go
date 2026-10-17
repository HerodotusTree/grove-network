// Package identity gives every Grove user a cryptographic account:
// an Ed25519 keypair whose public key IS the account id. The node
// never sees a private key and stores no passwords — a request is
// authorized if and only if it carries a valid signature over the
// canonical payload for the operation being attempted.
//
// This is authentication, not balances: the vault's accounting stays
// aggregate. Identity answers "who may act", never "who owns what".
package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// PayloadVersion pins the signing format. If the format ever changes,
// bump this so old signatures cannot be replayed against new code.
const PayloadVersion = "grove-v1"

// Ops that may be signed. Kept as constants so a typo'd op name can
// never produce a valid payload.
const (
	OpDeposit = "deposit"
	OpRedeem  = "redeem"
	OpTip     = "tip"
)

// Payload builds the canonical string that gets signed:
// grove-v1|<op>|<accountHex>|<amount>|<nonce>
// Every field is positional and unambiguous, so the same operation can
// never be re-signed into a different one, and the same signature can
// never authorize a different amount or a replayed nonce.
func Payload(op, accountHex string, amount int64, nonce uint64) string {
	return strings.Join([]string{
		PayloadVersion,
		op,
		accountHex,
		strconv.FormatInt(amount, 10),
		strconv.FormatUint(nonce, 10),
	}, "|")
}

// Generate creates a fresh Ed25519 keypair. privHex is the 32-byte seed
// as hex — the secret. pubHex is the 32-byte public key as hex — the
// account id, safe to share. The private key is shown once at creation;
// the node never stores it.
func Generate() (privHex, pubHex string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("identity: generate: %w", err)
	}
	return hex.EncodeToString(priv.Seed()), hex.EncodeToString(pub), nil
}

// Sign signs payload with the private seed. It returns the 64-byte
// signature as hex. Any malformed key is an error, never a panic.
func Sign(privHex, payload string) (sigHex string, err error) {
	seed, err := hex.DecodeString(privHex)
	if err != nil {
		return "", fmt.Errorf("identity: bad private key hex: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return "", fmt.Errorf("identity: private key must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(priv, []byte(payload))
	return hex.EncodeToString(sig), nil
}

// Verify reports whether sigHex is a valid signature over payload by the
// holder of the private key behind accountHex. Malformed inputs return
// false — verification failure and input failure are the same answer:
// not authorized. It never panics.
func Verify(accountHex, payload, sigHex string) bool {
	pub, err := hex.DecodeString(accountHex)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pub), []byte(payload), sig)
}
