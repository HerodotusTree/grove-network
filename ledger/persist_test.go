package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func appendN(l *Ledger, n int, prefix string) {
	for i := 0; i < n; i++ {
		l.Append(prefix + " entry")
	}
}

func readJournal(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "journal.jsonl"))
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	return string(b)
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")

	l := New()
	appendN(l, 5, "deposit")
	if err := l.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Len() != 5 {
		t.Fatalf("want 5 entries, got %d", got.Len())
	}
	if err := got.Verify(); err != nil {
		t.Fatalf("reloaded chain failed verify: %v", err)
	}
	for i := range 5 {
		a, b := l.entries[i], got.entries[i]
		if a.Hash != b.Hash || a.Payload != b.Payload || a.Index != b.Index || a.PrevHash != b.PrevHash {
			t.Fatalf("entry %d differs after reload", i)
		}
	}

	// Saving again with nothing new must be a no-op, not an error.
	if err := got.Save(path); err != nil {
		t.Fatalf("second save: %v", err)
	}
}

// TestTornTailTruncatedOnLoad simulates a crash mid-write: a partial
// final journal line with no trailing newline. Load must recover all
// complete entries and cut the torn tail off the journal file.
func TestTornTailTruncatedOnLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")

	l := New()
	appendN(l, 3, "deposit")
	if err := l.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	jp := filepath.Join(dir, "journal.jsonl")
	f, err := os.OpenFile(jp, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	if _, err := f.WriteString(`{"index":3,"pay`); err != nil { // torn: no newline
		t.Fatalf("write torn bytes: %v", err)
	}
	f.Close()

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load with torn tail: %v", err)
	}
	if got.Len() != 3 {
		t.Fatalf("want 3 recovered entries, got %d", got.Len())
	}
	if err := got.Verify(); err != nil {
		t.Fatalf("recovered chain failed verify: %v", err)
	}

	tail := readJournal(t, dir)
	if strings.Contains(tail, `{"index":3,"pay`) {
		t.Fatalf("torn bytes still in journal: %q", tail)
	}
	if !strings.HasSuffix(tail, "\n") {
		t.Fatalf("journal should end with a newline, got %q", tail)
	}
	if n := strings.Count(tail, "\n"); n != 3 {
		t.Fatalf("want 3 journal lines after truncation, got %d", n)
	}
}

// TestTornTailOnlyLine covers a torn write that is the journal's entire
// content: with the snapshot holding the compacted entries, recovery
// must keep them and empty the journal, not fail.
func TestTornTailOnlyLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")
	jp := filepath.Join(dir, "journal.jsonl")

	l := New()
	appendN(l, 2, "deposit")
	// Hand-roll a compacted state: snapshot holds the entries, journal
	// holds only torn bytes from a crash on the very next write.
	snap, err := l.Dump()
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	if err := os.WriteFile(path, snap, 0600); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	if err := os.WriteFile(jp, []byte(`{"index":2,"time`), 0600); err != nil {
		t.Fatalf("write torn journal: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load with torn-only journal: %v", err)
	}
	if got.Len() != 2 {
		t.Fatalf("want 2 entries, got %d", got.Len())
	}
	if err := got.Verify(); err != nil {
		t.Fatalf("recovered chain failed verify: %v", err)
	}
	if tail := readJournal(t, dir); tail != "" {
		t.Fatalf("torn-only journal not truncated, got %q", tail)
	}
}

// TestCorruptMiddleJournalRefuses: corruption in a non-final journal
// line is not a torn tail — Load must refuse to start, not silently
// drop history.
func TestCorruptMiddleJournalRefuses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")

	l := New()
	appendN(l, 3, "deposit")
	if err := l.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	jp := filepath.Join(dir, "journal.jsonl")
	lines := strings.Split(strings.TrimSuffix(readJournal(t, dir), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 journal lines, got %d", len(lines))
	}
	lines[1] = `{"index": 1, "payload": "corrupted` // invalid JSON mid-journal
	if err := os.WriteFile(jp, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatalf("write corrupt journal: %v", err)
	}

	got, err := Load(path)
	if err == nil {
		t.Fatalf("load accepted corrupt journal; got %d entries, want refusal", got.Len())
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("want a corruption error, got: %v", err)
	}
}

// TestLoadMissingChainIsFreshStart documents the contract in Load's
// doc comment: nothing on disk yields an os.IsNotExist error.
func TestLoadMissingChainIsFreshStart(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(filepath.Join(dir, "chain.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("want os.IsNotExist error for fresh start, got ledger=%v err=%v", got, err)
	}
}
