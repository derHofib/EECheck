// Package model holds the generic, protocol-agnostic domain types shared
// across discovery, pairing, use-case handlers, the orchestrator and the GUI.
package model

import "time"

// DeviceRole is the coarse role of a paired EEBus device, inferred from its
// SPINE entity type. It drives which use-case handlers are offered for a
// device in the GUI.
type DeviceRole string

const (
	RoleConsumer DeviceRole = "consumer" // Wallbox, Wärmepumpe, ...
	RoleProducer DeviceRole = "producer" // PV-Wechselrichter
	RoleStorage  DeviceRole = "storage"  // Batteriespeicher (kann Consumer+Producer sein)
	RoleUnknown  DeviceRole = "unknown"
)

// ConnectionState mirrors the coarse pairing/connection lifecycle of a
// device, independent of the underlying SHIP ConnectionState enum so the
// GUI and report layers don't need to import ship-go.
type ConnectionState string

const (
	StateDiscovered        ConnectionState = "discovered"
	StatePairingInProgress ConnectionState = "pairing"
	StateTrusted           ConnectionState = "trusted"
	StateConnected         ConnectionState = "connected"
	StateDisconnected      ConnectionState = "disconnected"
	StateTrustDenied       ConnectionState = "trust_denied"
	StateError             ConnectionState = "error"
)

// UseCaseID identifies a supported EEBus use case handled by this tool.
// Kept as a string type (not an eebus-go type) so internal/model has no
// dependency on the protocol libraries.
type UseCaseID string

const (
	UseCaseLPC UseCaseID = "LPC" // Limitation of Power Consumption
	UseCaseLPP UseCaseID = "LPP" // Limitation of Power Production
)

// Device is a discovered and/or paired EEBus device (the "Anlage"-Komponente
// under test), as far as the Steuerbox-Simulator needs to know about it.
type Device struct {
	SKI    string
	ShipID string

	Brand string
	Model string
	Name  string // display name: Brand + Model, or ShipID fallback

	Role      DeviceRole
	UseCases  []UseCaseID // use cases the entity actually advertises support for
	State     ConnectionState
	LastError string

	FirstSeen  time.Time
	LastSeen   time.Time
	PairedAt   *time.Time
	KnownDevice bool // true if this SKI was loaded from the local device store (skip trust confirmation)
}

// Key returns the stable identifier used to reference a device (its SKI).
func (d *Device) Key() string { return d.SKI }
