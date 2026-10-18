package ledger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Journaling: the snapshot at path holds the compacted chain, and
// journal.jsonl beside it holds newer entries, one JSON object per line.
// Save appends only entries not yet journaled (O(1) per call); every
// compactEvery journaled entries the snapshot is rewritten atomically
// and the journal truncated, so restarts stay fast and the journal
// stays bounded.

const compactEvery = 1000

func journalPath(path string) string {
	return filepath.Join(filepath.Dir(path), "journal.jsonl")
}

// Save appends any not-yet-journaled entries to the journal, fsyncs, and
// compacts when the journal grows past compactEvery entries.
func (l *Ledger) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if l.journaled > uint64(len(l.entries)) {
		return fmt.Errorf("save: journaled %d beyond %d entries", l.journaled, len(l.entries))
	}
	if pending := l.entries[l.journaled:]; len(pending) > 0 {
		f, err := os.OpenFile(journalPath(path), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		for _, e := range pending {
			b, err := json.Marshal(e)
			if err != nil {
				f.Close()
				return err
			}
			if _, err := f.Write(append(b, '\n')); err != nil {
				f.Close()
				return err
			}
		}
		if err := f.Sync(); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		l.journaled = uint64(len(l.entries))
	}
	return l.maybeCompact(path)
}

// maybeCompact rewrites the snapshot (atomically) and truncates the
// journal once the journal holds compactEvery entries.
func (l *Ledger) maybeCompact(path string) error {
	n, err := countLines(journalPath(path))
	if err != nil || n < compactEvery {
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
	return os.Truncate(journalPath(path), 0)
}

func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return bytes.Count(data, []byte{'\n'}), nil
}

// Load reads the snapshot and replays the journal, verifying every link.
// A missing chain returns an os.IsNotExist error (fresh start). A torn
// final journal line — a crash mid-write — is truncated; any other
// corruption refuses to start.
func Load(path string) (*Ledger, error) {
	l := &Ledger{}
	data, err := os.ReadFile(path)
	snapMissing := false
	switch {
	case err == nil:
		var entries []Entry
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, fmt.Errorf("load: bad snapshot: %w", err)
		}
		l.entries = entries
		if err := l.Verify(); err != nil {
			return nil, fmt.Errorf("load: snapshot broken: %w", err)
		}
	case os.IsNotExist(err):
		snapMissing = true
	default:
		return nil, err
	}
	if jerr := l.replayJournal(journalPath(path)); jerr != nil {
		if os.IsNotExist(jerr) && snapMissing {
			return nil, err // nothing on disk: fresh start
		}
		if !os.IsNotExist(jerr) {
			return nil, jerr
		}
	}
	l.journaled = uint64(len(l.entries))
	return l, nil
}

// replayJournal appends journaled entries after verifying each link.
func (l *Ledger) replayJournal(jp string) error {
	data, err := os.ReadFile(jp)
	if err != nil {
		return err
	}
	// A completed journal record always ends in a newline. Only an
	// unterminated tail can be a torn write; malformed completed records
	// are corruption and must not be silently dropped.
	completeEnd := bytes.LastIndexByte(data, '\n') + 1
	lines := bytes.Split(data[:completeEnd], []byte{'\n'})
	lines = lines[:len(lines)-1] // trailing separator, not an entry
	snapshotLen := uint64(len(l.entries))
	for i, line := range lines {
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			return fmt.Errorf("load: journal line %d corrupt: %w", i, err)
		}
		// Compaction replaces the snapshot before truncating the journal.
		// A process interruption between those writes leaves an overlap.
		// Skip only records already in the loaded snapshot, and require an
		// exact match so a conflicting history is never hidden.
		if e.Index < snapshotLen {
			if e != l.entries[e.Index] {
				return fmt.Errorf("load: journal line %d conflicts with snapshot entry %d", i, e.Index)
			}
			continue
		}
		if err := l.appendVerified(e); err != nil {
			return fmt.Errorf("load: journal line %d: %w", i, err)
		}
	}
	if completeEnd < len(data) {
		if err := os.Truncate(jp, int64(completeEnd)); err != nil {
			return fmt.Errorf("load: torn journal tail, truncate failed: %w", err)
		}
	}
	return nil
}

// appendVerified adds one entry after checking its index, hash, and link.
func (l *Ledger) appendVerified(e Entry) error {
	want := uint64(len(l.entries))
	if e.Index != want {
		return fmt.Errorf("index %d, want %d", e.Index, want)
	}
	if e.Hash != e.computeHash() {
		return fmt.Errorf("entry %d: hash mismatch", e.Index)
	}
	if want == 0 {
		if e.PrevHash != "" {
			return fmt.Errorf("genesis must have empty prev_hash")
		}
	} else if e.PrevHash != l.entries[want-1].Hash {
		return fmt.Errorf("entry %d: prev_hash mismatch", e.Index)
	}
	l.entries = append(l.entries, e)
	return nil
}
