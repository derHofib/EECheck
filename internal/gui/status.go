package gui

import (
	"image/color"
	"sync"
	"time"

	"github.com/derHofib/EECheck/internal/eebus"
)

// connStatus tracks the latest known connection/pairing status per SKI for
// display (Dashboard status dots), fed purely from internal/eebus.Core's
// event stream. It is a presentation-only concern; the protocol core stays
// unaware of it.
type connStatus struct {
	mu      sync.Mutex
	state   map[string]string
	updated map[string]time.Time
}

func newConnStatus() *connStatus {
	return &connStatus{state: make(map[string]string), updated: make(map[string]time.Time)}
}

// attach subscribes to core events once and keeps the status map current.
// onChange (optional) is called after every update, for the GUI to refresh.
func (c *connStatus) attach(core *eebus.Core, onChange func()) {
	core.Subscribe(func(ev eebus.Event) {
		if ev.SKI == "" {
			return
		}
		c.mu.Lock()
		switch ev.Kind {
		case eebus.EventDeviceConnected:
			c.state[ev.SKI] = "verbunden"
		case eebus.EventDeviceDisconnected:
			c.state[ev.SKI] = "getrennt"
		case eebus.EventTrustDenied:
			c.state[ev.SKI] = "Vertrauen abgelehnt"
		case eebus.EventPairingProgress:
			c.state[ev.SKI] = ev.Message
		}
		c.updated[ev.SKI] = time.Now()
		c.mu.Unlock()

		if onChange != nil {
			onChange()
		}
	})
}

// Label returns a short human status text for ski, or "unbekannt" if none
// has been observed yet this session.
func (c *connStatus) Label(ski string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.state[ski]; ok {
		return s
	}
	return "unbekannt"
}

// Color maps the tracked status to a Grün/Rot/Gelb dot per
// docs/03-ui-design.md's status-color convention.
func (c *connStatus) Color(ski string) color.Color {
	switch c.Label(ski) {
	case "verbunden", "trusted", "completed":
		return statusColorPass
	case "getrennt", "Vertrauen abgelehnt", "remote_denied_trust", "error":
		return statusColorFail
	case "unbekannt":
		return statusColorPending
	default:
		return statusColorRunning
	}
}
