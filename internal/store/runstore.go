package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/derHofib/EECheck/internal/orchestrator"
)

// RunRecord is the persisted summary of a finished test run, for the
// Dashboard's "Liste vergangener Testläufe" (docs/03-ui-design.md, screen 1)
// and for showing each device's last result on the Dashboard tab.
type RunRecord struct {
	ID            string                            `json:"id"`
	StartedAt     time.Time                         `json:"startedAt"`
	EndedAt       time.Time                         `json:"endedAt"`
	SiteAddress   string                            `json:"siteAddress"`
	Status        orchestrator.RunStatus            `json:"status"`
	DeviceNames   []string                          `json:"deviceNames"`
	DeviceResults map[string]orchestrator.RunStatus `json:"deviceResults"` // SKI -> Ergebnis
	ReportDir     string                            `json:"reportDir"`     // directory holding the exported PDFs + raw log for this run
}

// RunStore is a thread-safe, file-backed list of finished test runs.
type RunStore struct {
	path string

	mu   sync.Mutex
	runs map[string]RunRecord
}

// OpenRunStore loads (or creates) the run history in dataDir.
func OpenRunStore(dataDir string) (*RunStore, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	s := &RunStore{path: filepath.Join(dataDir, "runs.json"), runs: make(map[string]RunRecord)}
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var list []RunRecord
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	for _, r := range list {
		s.runs[r.ID] = r
	}
	return s, nil
}

// Save records or updates a run and persists the store.
func (s *RunStore) Save(r RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[r.ID] = r
	list := make([]RunRecord, 0, len(s.runs))
	for _, run := range s.runs {
		list = append(list, run)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].StartedAt.After(list[j].StartedAt) })
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// All returns all recorded runs, most recent first.
func (s *RunStore) All() []RunRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]RunRecord, 0, len(s.runs))
	for _, r := range s.runs {
		list = append(list, r)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].StartedAt.After(list[j].StartedAt) })
	return list
}
