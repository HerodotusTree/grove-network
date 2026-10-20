// Package ledger implements the Grove's append-only hash-chained log.
// Every entry commits to the hash of the entry before it, so any
// tampering with history breaks the chain and is detectable.
package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Entry is a single record in the chain.
type Entry struct {
	Index     uint64 `json:"index"`
	Timestamp int64  `json:"timestamp"` // unix seconds; informational only
	Domain    string `json:"domain"`    // genesis hash this entry belongs to (empty only for the genesis entry)
	Account   string `json:"account"`   // signer's Ed25519 public key, hex
	Nonce     uint64 `json:"nonce"`     // signer's sequential nonce
	Payload   string `json:"payload"`
	Signature string `json:"signature"` // Ed25519 signature over the canonical payload, hex
	PrevHash  string `json:"prev_hash"`
	Hash      string `json:"hash"`
}

func (e *Entry) computeHash() string {
	h := sha256.New()
	// The hash covers every committed field EXCEPT the entry's own Hash field.
	fmt.Fprintf(h, "%d|%d|%s|%s|%d|%s|%s|%s", e.Index, e.Timestamp, e.Domain, e.Account, e.Nonce, e.Payload, e.Signature, e.PrevHash)
	return hex.EncodeToString(h.Sum(nil))
}

// Ledger is an in-memory hash-chained log.
type Ledger struct {
	entries   []Entry
	journaled uint64 // leading entries already durable (snapshot or journal)
}

// New returns an empty ledger.
func New() *Ledger { return &Ledger{} }

// Append adds a payload as a new entry chained to the previous hash.
func (l *Ledger) Append(payload string) Entry {
	return l.appendEntry("", "", 0, payload, "")
}

// AppendSigned adds a fully identified entry: the network it belongs to
// (domain), who authorized it (account), the signer's sequential nonce,
// and the signature. Phase 1 entries are created this way so the ledger
// itself preserves the evidence that replay verification needs.
func (l *Ledger) AppendSigned(domain, account string, nonce uint64, payload, signature string) Entry {
	return l.appendEntry(domain, account, nonce, payload, signature)
}

func (l *Ledger) appendEntry(domain, account string, nonce uint64, payload, signature string) Entry {
	var prev string
	if n := len(l.entries); n > 0 {
		prev = l.entries[n-1].Hash
	}
	e := Entry{
		Index:     uint64(len(l.entries)),
		Timestamp: time.Now().Unix(),
		Domain:    domain,
		Account:   account,
		Nonce:     nonce,
		Payload:   payload,
		Signature: signature,
		PrevHash:  prev,
	}
	e.Hash = e.computeHash()
	l.entries = append(l.entries, e)
	return e
}

// Verify checks every link. Nil means the chain is intact.
func (l *Ledger) Verify() error {
	for i, e := range l.entries {
		if e.Hash != e.computeHash() {
			return fmt.Errorf("entry %d: hash mismatch (tampered?)", i)
		}
		if i == 0 {
			if e.PrevHash != "" {
				return fmt.Errorf("entry 0: genesis must have empty prev_hash")
			}
			continue
		}
		if e.PrevHash != l.entries[i-1].Hash {
			return fmt.Errorf("entry %d: prev_hash does not match entry %d hash", i, i-1)
		}
	}
	return nil
}

// Len returns the number of entries.
func (l *Ledger) Len() int { return len(l.entries) }

// Entries returns a copy of the chain.
func (l *Ledger) Entries() []Entry { return append([]Entry(nil), l.entries...) }

// Dump serializes the chain to JSON.
func (l *Ledger) Dump() ([]byte, error) { return json.MarshalIndent(l.entries, "", "  ") }
