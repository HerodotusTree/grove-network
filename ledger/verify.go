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
