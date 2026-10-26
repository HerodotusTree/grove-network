package ledger

import (
	"encoding/json"
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
// never merged.
//
// Crash atomicity (issue #1, torn adopt): the snapshot rename and the
// journal truncate are two durable steps, and a crash between them
// leaves the NEW snapshot beside the OLD chain's journal — a state
// Load would otherwise refuse as a snapshot/journal conflict, bricking
// the node. Adopt therefore commits in this order:
//
//  1. write the commit marker (adopted length + tip hash),
//  2. write the new snapshot (temp file + rename),
//  3. truncate the journal (the old chain's journal must not survive),
//  4. delete the marker,
//  5. only then replace the in-memory entries.
//
// A crash after step 2 leaves marker + snapshot + stale journal, which
// Load reconciles (marker match authorizes discarding the journal). A
// crash before step 2 leaves a stale marker beside the OLD snapshot
// and journal, which Load drops and then loads normally. If step 3
// fails after a successful rename, Adopt returns an error and the
// in-memory ledger is left untouched; the disk state is still
// recoverable at restart via the marker, but the node should be
// restarted before serving new ops — otherwise a later op appended to
// the stale journal would be discarded along with it at the
// reconciling restart.
//
// Callers must still validate a candidate fully (prefix + staged
// replay) BEFORE Adopt; Adopt itself checks hashes only, and
// hash-validity alone never authorized a chain.
func (l *Ledger) Adopt(entries []Entry, path string) error {
	if err := VerifyEntries(entries); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := writeAdoptMarker(path, entries); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
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
	if err := f.Close(); err != nil {
		return err
	}
	_ = os.Remove(adoptMarkerPath(path))
	l.entries = append([]Entry(nil), entries...)
	l.journaled = uint64(len(entries))
	return nil
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
