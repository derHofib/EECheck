package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/derHofib/EECheck/internal/report"
)

// MetaStore persists the operator-entered Anlage/Kunde master data
// (report.Meta) so it survives restarts and is reused for every report
// until the operator changes it (docs/03-ui-design.md follow-up: a
// dedicated "Anlage & Kunde" tab).
type MetaStore struct {
	path string
	mu   sync.Mutex
}

// OpenMetaStore prepares the store in dataDir.
func OpenMetaStore(dataDir string) (*MetaStore, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	return &MetaStore{path: filepath.Join(dataDir, "anlage-kunde.json")}, nil
}

// Load returns the persisted Meta, or a zero-value Meta if none was saved yet.
func (s *MetaStore) Load() report.Meta {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return report.Meta{}
	}
	var m report.Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return report.Meta{}
	}
	return m
}

// Save persists the given Meta.
func (s *MetaStore) Save(m report.Meta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
