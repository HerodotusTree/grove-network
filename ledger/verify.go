package ledger

import (
	"os"
	"path/filepath"
)

// VerifyEntries checks a set of entries as a chain without owning them.
// Used when adopting a peer's chain.
func VerifyEntries(entries []Entry) error {
	l := &Ledger{entries: entries}
	return l.Verify()
}

// Adopt replaces the ledger's entries with the given ones, but only
// after verifying them. A chain that fails verification is refused,
// never merged. It then writes a fresh snapshot and clears the journal:
// the old journal belonged to the old chain and must not survive.
func (l *Ledger) Adopt(entries []Entry, path string) error {
	if err := VerifyEntries(entries); err != nil {
		return err
	}
	l.entries = append([]Entry(nil), entries...)
	l.journaled = uint64(len(entries))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := l.Dump()
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	f, err := os.OpenFile(journalPath(path), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	return f.Close()
}

// FromEntries returns a ledger holding the given entries without
// adopting them - used to stage-verify a candidate chain (replay it
// through a scratch vault) before any swap is allowed.
func FromEntries(entries []Entry) *Ledger {
	return &Ledger{entries: append([]Entry(nil), entries...)}
}

// IsPrefixExtension reports whether candidate begins with every local
// entry, hash for hash, and continues past them. The entry hash covers
// every committed field (H12), so equal hashes mean identical entries.
// Phase 1 adoption accepts only prefix extensions: a node's
// acknowledged history is never rewritten by sync. True forks are
// Phase 2.
func IsPrefixExtension(local, candidate []Entry) bool {
	if len(candidate) <= len(local) {
		return false
	}
	for i, e := range local {
		if candidate[i].Hash != e.Hash {
			return false
		}
	}
	return true
}
