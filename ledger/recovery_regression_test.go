package ledger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoveryAfterInterruptedCompaction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")
	l := New()
	var journal bytes.Buffer
	for i := 0; i < compactEvery; i++ {
		e := l.Append("entry")
		if err := json.NewEncoder(&journal).Encode(e); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := l.Dump()
	if err != nil {
		t.Fatal(err)
	}
	// Exact disk state after snapshot rename but before journal truncation.
	// Both files are valid and contain the same committed prefix.
	if err := os.WriteFile(path, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath(path), journal.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("valid overlapping snapshot/journal after compaction crash rejected: %v", err)
	}
	if got.Len() != compactEvery {
		t.Fatalf("recovered entries=%d want=%d", got.Len(), compactEvery)
	}
}

func TestRecoveryRejectsCompletedCorruptTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")
	l := New()
	for i := 0; i < 3; i++ {
		l.Append("committed entry")
	}
	if err := l.Save(path); err != nil {
		t.Fatal(err)
	}
	jp := journalPath(path)
	b, err := os.ReadFile(jp)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	lines[2] = `{invalid complete record}`
	corrupt := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(jp, []byte(corrupt), 0600); err != nil {
		t.Fatal(err)
	}
	recovered, err := Load(path)
	if err != nil {
		return
	} // Desired: refuse a newline-terminated corrupt record.
	remaining, err := os.ReadFile(jp)
	if err != nil {
		t.Fatal(err)
	}
	lostLen := recovered.Len()
	recovered.Append("new entry")
	if err := recovered.Save(path); err != nil {
		t.Fatal(err)
	}
	_, secondErr := Load(path)
	t.Errorf("completed corrupt final record silently dropped: recovered=%d of 3; bad bytes retained=%t; after appending next entry restart=%v",
		lostLen, string(remaining) == corrupt, secondErr)
}

func TestRecoveryTruncatesUnterminatedCompleteJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")
	l := New()
	l.Append("committed")
	if err := l.Save(path); err != nil {
		t.Fatal(err)
	}
	// A process can stop after writing the JSON but before the newline.
	// Save never acknowledges that incomplete record.
	entry := l.Append("uncommitted")
	tail, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(journalPath(path), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := f.Write(tail)
	closeErr := f.Close()
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Len() != 1 {
		t.Fatalf("unterminated record recovered: got %d entries, want 1", got.Len())
	}
	got.Append("next committed")
	if err := got.Save(path); err != nil {
		t.Fatal(err)
	}
	restarted, err := Load(path)
	if err != nil {
		t.Fatalf("restart after recovery and append: %v", err)
	}
	if restarted.Len() != 2 {
		t.Fatalf("got %d entries, want 2", restarted.Len())
	}
}

func TestRecoveryOverlappingJournalThenNewEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")
	l := New()
	first := l.Append("first")
	second := l.Append("second")
	snapshot, err := l.Dump()
	if err != nil {
		t.Fatal(err)
	}
	third := l.Append("third")
	var journal bytes.Buffer
	for _, entry := range []Entry{first, second, third} {
		if err := json.NewEncoder(&journal).Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath(path), journal.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Len() != 3 || got.entries[2] != third {
		t.Fatalf("overlap plus new entry was not recovered: %+v", got.entries)
	}
	got.Append("fourth")
	if err := got.Save(path); err != nil {
		t.Fatal(err)
	}
	restarted, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Len() != 4 {
		t.Fatalf("restarted entries=%d, want 4", restarted.Len())
	}
}

func TestRecoveryRejectsConflictingSnapshotOverlap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")
	l := New()
	entry := l.Append("original")
	snapshot, err := l.Dump()
	if err != nil {
		t.Fatal(err)
	}
	entry.Payload = "conflicting"
	entry.Hash = entry.computeHash()
	var journal bytes.Buffer
	if err := json.NewEncoder(&journal).Encode(entry); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath(path), journal.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("conflicting overlap accepted")
	}
}

func TestRecoveryRejectsDuplicateNewJournalEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")
	l := New()
	l.Append("snapshot entry")
	snapshot, err := l.Dump()
	if err != nil {
		t.Fatal(err)
	}
	entry := l.Append("new journal entry")
	var journal bytes.Buffer
	for i := 0; i < 2; i++ {
		if err := json.NewEncoder(&journal).Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath(path), journal.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("duplicate new journal entry accepted")
	}
}

// TestRecoveryTruncatesTornTailAfterMultipleJournaledEntries pins the
// crash-mid-write case on a non-trivial journal: three committed entries
// are durably journaled, then the process dies partway through writing
// the fourth. Only the unterminated tail may be discarded; the three
// complete lines must replay intact, the torn bytes must be truncated
// from disk, and a later Save must not re-journal the recovered prefix
// or duplicate anything.
func TestRecoveryTruncatesTornTailAfterMultipleJournaledEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")
	l := New()
	for _, p := range []string{"first", "second", "third"} {
		l.Append(p)
	}
	if err := l.Save(path); err != nil {
		t.Fatal(err)
	}
	// Fourth entry committed in memory but never fsync'd: a torn write
	// stops mid-object with no trailing newline.
	entry := l.Append("fourth-torn")
	tail, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(journalPath(path), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(tail[:len(tail)/2]); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("multi-entry journal with torn tail rejected: %v", err)
	}
	if got.Len() != 3 {
		t.Fatalf("recovered %d entries, want 3 (torn fourth must be dropped)", got.Len())
	}
	for i, want := range []string{"first", "second", "third"} {
		if got.entries[i].Payload != want {
			t.Fatalf("entry %d payload=%q, want %q", i, got.entries[i].Payload, want)
		}
	}
	// The torn bytes must be gone from disk: the next append writes its
	// newline-terminated record at the true end of the file.
	rest, err := os.ReadFile(journalPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) > 0 && rest[len(rest)-1] != '\n' {
		t.Fatal("torn tail was not truncated from the journal file")
	}
	// The in-memory journal accounting must also be right: Save may only
	// journal entries not yet durable (index 3), never the recovered 0-2.
	got.Append("fourth-redone")
	if err := got.Save(path); err != nil {
		t.Fatal(err)
	}
	restarted, err := Load(path)
	if err != nil {
		t.Fatalf("restart after recovery and append: %v", err)
	}
	if restarted.Len() != 4 {
		t.Fatalf("restarted entries=%d, want 4", restarted.Len())
	}
	if restarted.entries[3].Payload != "fourth-redone" {
		t.Fatalf("entry 3 payload=%q, want %q", restarted.entries[3].Payload, "fourth-redone")
	}
	if err := restarted.Verify(); err != nil {
		t.Fatalf("recovered chain failed verification: %v", err)
	}
}
