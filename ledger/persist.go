package ledger

import (
	"encoding/json"
	"fmt"
	"os"
)

// Save writes the chain to disk as JSON.
func (l *Ledger) Save(path string) error {
	data, err := l.Dump()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Load reads a chain from disk and verifies it before returning it.
// A broken chain on disk is refused — the node will not start on lies.
func Load(path string) (*Ledger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("load: bad file: %w", err)
	}
	l := &Ledger{entries: entries}
	if err := l.Verify(); err != nil {
		return nil, fmt.Errorf("load: chain broken: %w", err)
	}
	return l, nil
}

