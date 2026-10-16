package ledger

// VerifyEntries checks a set of entries as a chain without owning them.
// Used when adopting a peer's chain.
func VerifyEntries(entries []Entry) error {
	l := &Ledger{entries: entries}
	return l.Verify()
}

// Adopt replaces the ledger's entries with the given ones, but only
// after verifying them. A chain that fails verification is refused,
// never merged.
func (l *Ledger) Adopt(entries []Entry) error {
	if err := VerifyEntries(entries); err != nil {
		return err
	}
	l.entries = append([]Entry(nil), entries...)
	return nil
}

