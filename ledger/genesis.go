package ledger

import "fmt"

// Genesis handling (Phase 1): entry 0 is the network's birth
// certificate. Its payload comes from the shared genesis config file,
// its Domain field is empty, and its hash IS the network domain that
// every later entry carries and every signature binds.

// AppendGenesis writes entry 0 on an empty ledger. The timestamp is
// fixed at 0 so the genesis entry - and therefore the domain - is
// byte-identical on every node that shares the same genesis payload:
// a wall-clock timestamp would fork the network at birth.
func (l *Ledger) AppendGenesis(payload string) (Entry, error) {
	if len(l.entries) != 0 {
		return Entry{}, fmt.Errorf("genesis: ledger already has %d entries", len(l.entries))
	}
	e := Entry{
		Index:   0,
		Payload: payload,
	}
	e.Hash = e.computeHash()
	l.entries = append(l.entries, e)
	return e, nil
}

// Domain returns the network domain: the hash of entry 0. The second
// return value is false on an empty ledger (no genesis yet).
func (l *Ledger) Domain() (string, bool) {
	if len(l.entries) == 0 {
		return "", false
	}
	return l.entries[0].Hash, true
}

// CheckDomain verifies that a chain belongs to the network identified
// by domain: entry 0 must hash to domain and carry an empty Domain
// field, and every later entry must carry domain. It checks belonging
// only; chain integrity is Verify's job.
func CheckDomain(entries []Entry, domain string) error {
	if len(entries) == 0 {
		return fmt.Errorf("domain: empty chain")
	}
	g := entries[0]
	if g.Domain != "" {
		return fmt.Errorf("domain: genesis entry must carry empty domain, got %q", g.Domain)
	}
	if g.Hash != domain {
		return fmt.Errorf("domain: genesis hash %s does not match configured domain %s", g.Hash, domain)
	}
	for i, e := range entries[1:] {
		if e.Domain != domain {
			return fmt.Errorf("domain: entry %d carries domain %q, want %q", i+1, e.Domain, domain)
		}
	}
	return nil
}
