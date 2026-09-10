// Package testdevice provides shared SHIP/SPINE bootstrap code for the
// standalone test fixtures in cmd/testwallbox and cmd/testspeicher. These
// simulate the "Controllable System" (device) side of LPC/LPP - a real
// wallbox or PV/storage inverter - purely so the Steuerbox-Simulator
// (internal/eebus, the "Energy Guard" side) can be exercised end-to-end
// over a real SHIP/SPINE connection without needing actual hardware.
//
// Trust handling here is deliberately maximally permissive (any Steuerbox
// is auto-accepted, no manual confirmation): these binaries are disposable
// local test fixtures, not devices meant to ship or to be exposed beyond a
// developer's own machine/network.
package testdevice

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	eebusapi "github.com/enbility/eebus-go/api"
	"github.com/enbility/eebus-go/service"
	shipapi "github.com/enbility/ship-go/api"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"

	"github.com/derHofib/EECheck/internal/eebus"
)

// Options configures one simulated device.
type Options struct {
	DataDir        string // certificate persists here, so the SKI stays stable across restarts
	Port           int
	Brand          string
	Model          string
	SerialNumber   string
	DeviceCategory shipapi.DeviceCategoryType
	DeviceType     model.DeviceTypeType
	EntityType     model.EntityTypeType
}

// Device is a running SHIP/SPINE service acting as a Controllable System.
type Device struct {
	Service     *service.Service
	LocalEntity spineapi.EntityLocalInterface
	Logger      *log.Logger
}

// New creates and sets up the device (certificate, SHIP service, local
// entity). Call AddUseCase-style setup on d.LocalEntity for the specific
// use cases (cs/lpc, cs/lpp) the caller wants to expose, then Start.
func New(opts Options) (*Device, error) {
	if err := os.MkdirAll(opts.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}

	certificate, err := eebus.LoadOrCreateIdentity(opts.DataDir)
	if err != nil {
		return nil, fmt.Errorf("loading identity: %w", err)
	}

	// PairingModeListener: a real device waits for the Steuerbox/CEM to
	// initiate pairing, rather than announcing itself to one - the mirror
	// image of internal/eebus.Core's PairingModeAnnouncer (see
	// docs/05-recherche-antworten.md section 3). Listener mode requires a
	// RingBufferPersistence for pairing replay protection.
	pairingConfig := shipapi.NewPairingConfig(shipapi.PairingModeListener, nil)
	ringBuffer := &fileRingBuffer{path: filepath.Join(opts.DataDir, "ring-buffer.json")}

	cfg, err := eebusapi.NewConfiguration(
		"EECheck", opts.Brand, opts.Model, opts.SerialNumber,
		[]shipapi.DeviceCategoryType{opts.DeviceCategory},
		opts.DeviceType,
		[]model.EntityTypeType{opts.EntityType},
		opts.Port, certificate, time.Second*60, pairingConfig, ringBuffer,
	)
	if err != nil {
		return nil, fmt.Errorf("building configuration: %w", err)
	}

	d := &Device{Logger: log.Default()}
	d.Service = service.NewService(cfg, d)
	d.Service.SetLogging(nopLogger{})

	if err := d.Service.Setup(); err != nil {
		return nil, fmt.Errorf("service setup: %w", err)
	}

	// Auto-accept any pairing request - see package doc.
	d.Service.SetAutoAccept(true)
	d.Service.UserIsAbleToApproveOrCancelPairingRequests(true)

	d.LocalEntity = d.Service.LocalDevice().EntityForType(opts.EntityType)
	if d.LocalEntity == nil {
		return nil, fmt.Errorf("local entity of type %s was not created", opts.EntityType)
	}

	return d, nil
}

// Start begins mDNS announcement and the SHIP server.
func (d *Device) Start() error { return d.Service.Start() }

// Shutdown stops discovery and closes all connections.
func (d *Device) Shutdown() { d.Service.Shutdown() }

// ---- eebusapi.ServiceReaderInterface: console logging only, always permissive ----

func (d *Device) RemoteServiceConnected(_ eebusapi.ServiceInterface, identity shipapi.ServiceIdentity) {
	d.Logger.Printf("[verbunden]  SKI %s", identity.SKI)
}

func (d *Device) RemoteServiceDisconnected(_ eebusapi.ServiceInterface, identity shipapi.ServiceIdentity) {
	d.Logger.Printf("[getrennt]   SKI %s", identity.SKI)
}

func (d *Device) VisibleRemoteMdnsServicesUpdated(_ eebusapi.ServiceInterface, _ []shipapi.RemoteMdnsService) {
}

func (d *Device) ServiceUpdated(identity shipapi.ServiceIdentity) {
	d.Logger.Printf("[service]    SKI %s aktualisiert", identity.SKI)
}

func (d *Device) ServicePairingDetailUpdate(identity shipapi.ServiceIdentity, detail *shipapi.ConnectionStateDetail) {
	d.Logger.Printf("[pairing]    SKI %s: Status %d", identity.SKI, detail.State())
}

func (d *Device) ServiceAutoTrusted(_ eebusapi.ServiceInterface, identity shipapi.ServiceIdentity) {
	d.Logger.Printf("[vertraut]   SKI %s", identity.SKI)
}

func (d *Device) ServiceAutoTrustFailed(_ eebusapi.ServiceInterface, identity shipapi.ServiceIdentity, reason error) {
	d.Logger.Printf("[fehler]     Vertrauen für SKI %s fehlgeschlagen: %v", identity.SKI, reason)
}

func (d *Device) ServiceAutoTrustRemoved(_ eebusapi.ServiceInterface, identity shipapi.ServiceIdentity, reason string) {
	d.Logger.Printf("[entfernt]   Vertrauen für SKI %s entfernt: %s", identity.SKI, reason)
}

type nopLogger struct{}

func (nopLogger) Trace(...interface{})          {}
func (nopLogger) Tracef(string, ...interface{}) {}
func (nopLogger) Debug(...interface{})          {}
func (nopLogger) Debugf(string, ...interface{}) {}
func (nopLogger) Info(...interface{})           {}
func (nopLogger) Infof(string, ...interface{})  {}
func (nopLogger) Error(...interface{})          {}
func (nopLogger) Errorf(string, ...interface{}) {}

// WaitForSignal blocks until SIGINT/SIGTERM, for a simple CLI test fixture
// main loop.
func WaitForSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}

// DefaultDataDir returns a per-fixture data directory under the user's
// config directory, so each test device keeps its own stable certificate.
func DefaultDataDir(name string) string {
	base, err := os.UserConfigDir()
	if err != nil {
		return name
	}
	return filepath.Join(base, "EECheck", name)
}

// fileRingBuffer persists the SHIP pairing replay-protection ring buffer to
// a local JSON file, so a restarted test device doesn't lose its replay
// history (a real, if minimal, implementation - not the no-op demo from
// eebus-go's own examples).
type fileRingBuffer struct {
	path string
}

type ringBufferState struct {
	Entries   []shipapi.DigestEntry `json:"entries"`
	NextIndex int                   `json:"nextIndex"`
}

func (r *fileRingBuffer) LoadRingBuffer() ([]shipapi.DigestEntry, int, error) {
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return make([]shipapi.DigestEntry, 100), 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	var state ringBufferState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, 0, err
	}
	return state.Entries, state.NextIndex, nil
}

func (r *fileRingBuffer) SaveRingBuffer(entries []shipapi.DigestEntry, nextIndex int) error {
	data, err := json.Marshal(ringBufferState{Entries: entries, NextIndex: nextIndex})
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}
