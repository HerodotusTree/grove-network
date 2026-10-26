package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The commit marker is what separates a torn adopt (recoverable) from a
// corrupted journal (must refuse). Both branches are pinned here so a
// later change cannot silently collapse them into one rule.
func TestAdoptMarkerReconcilesTornAdopt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")

	adopted := New()
	adopted.Append(`{"kind":"deposit","amount":700,"reserves_after":700,"claims_after":700,"floor_after":0}`)
	adopted.Append(`{"kind":"deposit","amount":700,"reserves_after":1400,"claims_after":1400,"floor_after":0}`)
	data, err := adopted.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	// Stale journal from the pre-adopt chain, conflicting at index 0.
	if err := os.WriteFile(journalPath(path), []byte("{\"index\":0,\"timestamp\":1,\"payload\":\"old\",\"prev_hash\":\"\",\"hash\":\"x\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Marker matching the snapshot, as Adopt writes before its rename.
	entries := adopted.Entries()
	marker, _ := json.Marshal(adoptMarker{Entries: len(entries), Tip: entries[len(entries)-1].Hash})
	if err := os.WriteFile(adoptMarkerPath(path), marker, 0600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("torn adopt with matching marker refused to start: %v", err)
	}
	if got.Len() != 2 {
		t.Fatalf("recovered entries=%d, want 2 (adopted snapshot)", got.Len())
	}
	if _, err := os.Stat(adoptMarkerPath(path)); !os.IsNotExist(err) {
		t.Fatal("marker was not cleaned up after reconciliation")
	}
}

func TestStaleMarkerDoesNotAuthorizeJournalDiscard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.json")

	l := New()
	l.Append("original")
	snapshot, err := l.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	// Conflicting journal line, as in
	// TestRecoveryRejectsConflictingSnapshotOverlap.
	entry := l.Entries()[0]
	entry.Payload = "conflicting"
	entry.Hash = entry.computeHash()
	line, _ := json.Marshal(entry)
	if err := os.WriteFile(journalPath(path), append(line, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	// A marker that does NOT match the snapshot (adopt never committed).
	marker, _ := json.Marshal(adoptMarker{Entries: 9, Tip: "deadbeef"})
	if err := os.WriteFile(adoptMarkerPath(path), marker, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("conflicting journal accepted on the strength of a stale marker")
	}
}
