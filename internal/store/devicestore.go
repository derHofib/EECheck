// Package store persists application data (known/paired devices, test run
// history) to local JSON files, independent of the SHIP/SPINE stack's own
// in-process pairing state. eebus-go/ship-go do not persist "which SKIs are
// known" across restarts on their own (see docs/05-recherche-antworten.md,
// section 5) — we own that list and re-register known SKIs with the
// protocol core on every startup so a re-test never requires re-pairing.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/derHofib/EECheck/internal/model"
)

// KnownDevice is the persisted record for a device the operator has paired
// at least once. It intentionally does not store any certificate/key
// material - that stays inside ship-go's own TLS trust handling.
type KnownDevice struct {
	SKI         string           `json:"ski"`
	ShipID      string           `json:"shipId"`
	Brand       string           `json:"brand"`
	Model       string           `json:"model"`
	Name        string           `json:"name"`
	Role        model.DeviceRole `json:"role"`
	FirstPaired time.Time        `json:"firstPaired"`
	LastSeen    time.Time        `json:"lastSeen"`
}

// DeviceStore is a thread-safe, file-backed registry of known devices.
type DeviceStore struct {
	path string

	mu      sync.Mutex
	devices map[string]KnownDevice // keyed by SKI
}

// Open loads the device store from dataDir (creating it empty if absent).
func Open(dataDir string) (*DeviceStore, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	s := &DeviceStore{
		path:    filepath.Join(dataDir, "devices.json"),
		devices: make(map[string]KnownDevice),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *DeviceStore) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var list []KnownDevice
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range list {
		s.devices[d.SKI] = d
	}
	return nil
}

func (s *DeviceStore) saveLocked() error {
	list := make([]KnownDevice, 0, len(s.devices))
	for _, d := range s.devices {
		list = append(list, d)
	}
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

// Upsert records or updates a known device and persists the store.
func (s *DeviceStore) Upsert(d KnownDevice) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.devices[d.SKI]; ok && !existing.FirstPaired.IsZero() {
		d.FirstPaired = existing.FirstPaired
	} else if d.FirstPaired.IsZero() {
		d.FirstPaired = time.Now()
	}
	s.devices[d.SKI] = d
	return s.saveLocked()
}

// Remove forgets a device (operator explicitly "un-pairs" it).
func (s *DeviceStore) Remove(ski string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.devices, ski)
	return s.saveLocked()
}

// All returns all known devices.
func (s *DeviceStore) All() []KnownDevice {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]KnownDevice, 0, len(s.devices))
	for _, d := range s.devices {
		list = append(list, d)
	}
	return list
}

// Get returns a known device by SKI.
func (s *DeviceStore) Get(ski string) (KnownDevice, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[ski]
	return d, ok
}
