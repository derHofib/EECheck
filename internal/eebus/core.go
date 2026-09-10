// Package eebus wraps eebus-go/ship-go/spine-go into the generic
// "Protokoll-Core" described in docs/02-architektur.md: a device-type
// agnostic API to discover, pair and talk to EEBus devices, on top of which
// the LPC/LPP use-case handlers and the orchestrator are built.
//
// This Core always acts as the "Energy Guard" (Steuerbox/CEM) actor - see
// docs/05-recherche-antworten.md for why that is the verified counterpart
// role to a Controllable System device (Wallbox, PV-Wechselrichter, ...).
package eebus

import (
	"fmt"
	"sync"
	"time"

	eebusapi "github.com/enbility/eebus-go/api"
	"github.com/enbility/eebus-go/service"
	shipapi "github.com/enbility/ship-go/api"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"

	domainmodel "github.com/derHofib/EECheck/internal/model"
)

// Config holds everything needed to stand up the local Steuerbox identity.
type Config struct {
	DataDir      string // where our own certificate and pairing state live
	Port         int    // 0 = let eebus-go pick its default
	DeviceBrand  string
	DeviceModel  string
	SerialNumber string
}

// EventKind is a coarse classification of core-level events, used by the
// orchestrator and GUI to react without depending on ship-go/spine-go types.
type EventKind string

const (
	EventDiscoveryUpdated  EventKind = "discovery_updated"
	EventPairingProgress   EventKind = "pairing_progress"
	EventDeviceConnected   EventKind = "device_connected"
	EventDeviceDisconnected EventKind = "device_disconnected"
	EventTrustDenied       EventKind = "trust_denied"
	EventUseCaseSupport    EventKind = "usecase_support" // a remote entity started/stopped advertising a use case
)

// Event is delivered to the Core's subscriber (normally the orchestrator)
// whenever something discovery/pairing/connection related happens.
type Event struct {
	Kind    EventKind
	SKI     string
	Message string
}

// Core is the generic EEBus protocol layer. It is safe for concurrent use.
type Core struct {
	cfg       Config
	myService *service.Service
	localEntity spineapi.EntityLocalInterface

	mu        sync.Mutex
	discovered map[string]shipapi.RemoteMdnsService // ski -> last seen mDNS record
	listeners  []func(Event)

	useCases map[domainmodel.UseCaseID]UseCaseAdapter
}

// UseCaseAdapter is implemented by internal/usecase/lpc and internal/usecase/lpp.
// It lets the Core stay ignorant of concrete use-case wiring: each adapter
// knows how to add itself to the local entity and how to report which
// remote entities currently advertise support for it.
type UseCaseAdapter interface {
	ID() domainmodel.UseCaseID
	Install(localEntity spineapi.EntityLocalInterface, eventCB eebusapi.EntityEventCallback) error
	SupportedEntities() []spineapi.EntityRemoteInterface
}

// NewCore creates the Steuerbox identity (loading or generating our own
// certificate) and prepares the eebus-go service. Call AddUseCase for each
// use-case handler, then Start.
func NewCore(cfg Config) (*Core, error) {
	if cfg.DeviceBrand == "" {
		cfg.DeviceBrand = "EECheck"
	}
	if cfg.DeviceModel == "" {
		cfg.DeviceModel = "Steuerbox-Simulator"
	}
	if cfg.SerialNumber == "" {
		cfg.SerialNumber = "EECHECK-0001"
	}

	certificate, err := LoadOrCreateIdentity(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("loading Steuerbox identity: %w", err)
	}

	c := &Core{
		cfg:        cfg,
		discovered: make(map[string]shipapi.RemoteMdnsService),
		useCases:   make(map[domainmodel.UseCaseID]UseCaseAdapter),
	}

	// PairingModeAnnouncer: we (the Steuerbox) initiate the pairing
	// handshake towards devices the operator selects from the discovery
	// list, matching the operator-driven "Pairing starten" flow in
	// docs/03-ui-design.md and the eebus-go reference Steuerbox example
	// (examples/controlbox) - see docs/05-recherche-antworten.md section 3.
	pairingConfig := shipapi.NewPairingConfig(shipapi.PairingModeAnnouncer, nil)

	configuration, err := eebusapi.NewConfiguration(
		"EECheck", cfg.DeviceBrand, cfg.DeviceModel, cfg.SerialNumber,
		[]shipapi.DeviceCategoryType{shipapi.DeviceCategoryTypeGridConnectionHub},
		model.DeviceTypeTypeElectricitySupplySystem,
		[]model.EntityTypeType{model.EntityTypeTypeGridGuard},
		cfg.Port, certificate, time.Second*60, pairingConfig, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("building eebus configuration: %w", err)
	}

	c.myService = service.NewService(configuration, c)
	c.myService.SetLogging(&nopLogger{})

	if err := c.myService.Setup(); err != nil {
		return nil, fmt.Errorf("setting up eebus service: %w", err)
	}

	c.localEntity = c.myService.LocalDevice().EntityForType(model.EntityTypeTypeGridGuard)
	if c.localEntity == nil {
		return nil, fmt.Errorf("local GridGuard entity was not created")
	}

	return c, nil
}

// AddUseCase installs a use-case adapter (LPC/LPP) on the local entity and
// wires its events back through the Core's generic Event stream.
func (c *Core) AddUseCase(adapter UseCaseAdapter) error {
	cb := func(ski string, device spineapi.DeviceRemoteInterface, entity spineapi.EntityRemoteInterface, event eebusapi.EventType) {
		c.emit(Event{Kind: EventUseCaseSupport, SKI: ski, Message: string(event)})
	}
	if err := adapter.Install(c.localEntity, cb); err != nil {
		return fmt.Errorf("installing use case %s: %w", adapter.ID(), err)
	}
	c.mu.Lock()
	c.useCases[adapter.ID()] = adapter
	c.mu.Unlock()
	return nil
}

// Subscribe registers a listener for Core events (discovery, pairing,
// connection lifecycle). Intended to be called once by the orchestrator.
func (c *Core) Subscribe(fn func(Event)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listeners = append(c.listeners, fn)
}

func (c *Core) emit(e Event) {
	c.mu.Lock()
	listeners := append([]func(Event){}, c.listeners...)
	c.mu.Unlock()
	for _, l := range listeners {
		l(e)
	}
}

// Start begins mDNS discovery and the SHIP server.
func (c *Core) Start() error { return c.myService.Start() }

// Shutdown stops discovery and closes all connections.
func (c *Core) Shutdown() { c.myService.Shutdown() }

// LocalEntity exposes the local Steuerbox entity for use-case adapters that
// need it directly (installed via AddUseCase in the usual case).
func (c *Core) LocalEntity() spineapi.EntityLocalInterface { return c.localEntity }

// DiscoveredDevices returns the last known mDNS view of all visible EEBus
// services, regardless of pairing state.
func (c *Core) DiscoveredDevices() []shipapi.RemoteMdnsService {
	c.mu.Lock()
	defer c.mu.Unlock()
	list := make([]shipapi.RemoteMdnsService, 0, len(c.discovered))
	for _, d := range c.discovered {
		list = append(list, d)
	}
	return list
}

// Pair explicitly trusts and connects to a discovered device. This marks
// the SKI as trusted in the SHIP hub immediately (see
// docs/05-recherche-antworten.md section 4/5 for why this is the correct,
// verified call for an operator-driven "Pairing starten" action rather than
// the passive/global pairing-mode toggle).
func (c *Core) Pair(ski, shipID string) {
	identity := shipapi.NewServiceIdentity(ski, "", shipID)
	c.myService.RegisterRemoteService(identity)
}

// Forget removes a device from the SHIP hub's trusted list. Combine with
// removing it from the local device store (internal/store) to fully
// "un-pair" it.
func (c *Core) Forget(ski string) {
	c.myService.UnregisterRemoteService(shipapi.NewServiceIdentity(ski, "", ""))
}

// UseCaseSupportedEntities returns the remote entities currently advertising
// support for the given use case, e.g. to populate the Szenario-Konfiguration
// screen for a paired device.
func (c *Core) UseCaseSupportedEntities(id domainmodel.UseCaseID) []spineapi.EntityRemoteInterface {
	c.mu.Lock()
	adapter, ok := c.useCases[id]
	c.mu.Unlock()
	if !ok {
		return nil
	}
	return adapter.SupportedEntities()
}

// ---- eebusapi.ServiceReaderInterface implementation ----

func (c *Core) RemoteServiceConnected(_ eebusapi.ServiceInterface, identity shipapi.ServiceIdentity) {
	c.emit(Event{Kind: EventDeviceConnected, SKI: identity.SKI})
}

func (c *Core) RemoteServiceDisconnected(_ eebusapi.ServiceInterface, identity shipapi.ServiceIdentity) {
	c.emit(Event{Kind: EventDeviceDisconnected, SKI: identity.SKI})
}

func (c *Core) VisibleRemoteMdnsServicesUpdated(_ eebusapi.ServiceInterface, entries []shipapi.RemoteMdnsService) {
	c.mu.Lock()
	c.discovered = make(map[string]shipapi.RemoteMdnsService, len(entries))
	for _, e := range entries {
		c.discovered[e.Ski] = e
	}
	c.mu.Unlock()
	c.emit(Event{Kind: EventDiscoveryUpdated})
}

func (c *Core) ServiceUpdated(identity shipapi.ServiceIdentity) {
	c.emit(Event{Kind: EventPairingProgress, SKI: identity.SKI, Message: "service_updated"})
}

func (c *Core) ServicePairingDetailUpdate(identity shipapi.ServiceIdentity, detail *shipapi.ConnectionStateDetail) {
	msg := connectionStateLabel(detail.State())
	if detail.State() == shipapi.ConnectionStateRemoteDeniedTrust {
		c.emit(Event{Kind: EventTrustDenied, SKI: identity.SKI, Message: msg})
		return
	}
	c.emit(Event{Kind: EventPairingProgress, SKI: identity.SKI, Message: msg})
}

func (c *Core) ServiceAutoTrusted(_ eebusapi.ServiceInterface, identity shipapi.ServiceIdentity) {
	c.emit(Event{Kind: EventPairingProgress, SKI: identity.SKI, Message: "trusted"})
}

func (c *Core) ServiceAutoTrustFailed(_ eebusapi.ServiceInterface, identity shipapi.ServiceIdentity, reason error) {
	c.emit(Event{Kind: EventTrustDenied, SKI: identity.SKI, Message: reason.Error()})
}

func (c *Core) ServiceAutoTrustRemoved(_ eebusapi.ServiceInterface, identity shipapi.ServiceIdentity, reason string) {
	c.emit(Event{Kind: EventDeviceDisconnected, SKI: identity.SKI, Message: reason})
}

func connectionStateLabel(s shipapi.ConnectionState) string {
	switch s {
	case shipapi.ConnectionStateNone:
		return "none"
	case shipapi.ConnectionStateQueued:
		return "queued"
	case shipapi.ConnectionStateInitiated:
		return "initiated"
	case shipapi.ConnectionStateReceivedPairingRequest:
		return "received_pairing_request"
	case shipapi.ConnectionStateInProgress:
		return "in_progress"
	case shipapi.ConnectionStateTrusted:
		return "trusted"
	case shipapi.ConnectionStateCompleted:
		return "completed"
	case shipapi.ConnectionStateRemoteDeniedTrust:
		return "remote_denied_trust"
	case shipapi.ConnectionStateError:
		return "error"
	default:
		return "unknown"
	}
}

// nopLogger discards eebus-go/ship-go internal debug logging. Raw/decoded
// protocol traffic for the Prüfprotokoll is captured separately at the
// SPINE event level (internal/messagelog), not via this text logger - see
// docs/05-recherche-antworten.md section on message logging.
type nopLogger struct{}

func (l *nopLogger) Trace(...interface{})          {}
func (l *nopLogger) Tracef(string, ...interface{}) {}
func (l *nopLogger) Debug(...interface{})          {}
func (l *nopLogger) Debugf(string, ...interface{}) {}
func (l *nopLogger) Info(...interface{})           {}
func (l *nopLogger) Infof(string, ...interface{})  {}
func (l *nopLogger) Error(...interface{})          {}
func (l *nopLogger) Errorf(string, ...interface{}) {}
