// Package gui implements the native Fyne UI described in
// docs/03-ui-design.md: Dashboard, Discovery & Pairing,
// Szenario-Konfiguration, Live-Monitor and Ergebnis/Report, navigated
// within a single window (docs/03-ui-design.md leaves single- vs.
// multi-window open; a single window with swappable content is the
// simpler, more native-feeling choice for a focused on-site tool and is
// what this implementation uses).
package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/derHofib/EECheck/internal/eebus"
	"github.com/derHofib/EECheck/internal/messagelog"
	"github.com/derHofib/EECheck/internal/orchestrator"
	"github.com/derHofib/EECheck/internal/report"
	"github.com/derHofib/EECheck/internal/store"
)

// App wires the GUI to the backend (protocol core, orchestrator,
// persistence) and manages which screen is currently shown.
type App struct {
	fyneApp fyne.App
	win     fyne.Window

	core    *eebus.Core
	orch    *orchestrator.Orchestrator
	devices *store.DeviceStore
	runs    *store.RunStore
	msgLog  *messagelog.Log
	dataDir string
	meta    report.Meta

	selected map[string]bool // SKI -> selected for the next test run, kept across screens
}

// NewApp builds the GUI shell over an already-started backend.
func NewApp(core *eebus.Core, orch *orchestrator.Orchestrator, devices *store.DeviceStore, runs *store.RunStore, msgLog *messagelog.Log, dataDir string) *App {
	return &App{
		core:     core,
		orch:     orch,
		devices:  devices,
		runs:     runs,
		msgLog:   msgLog,
		dataDir:  dataDir,
		meta:     report.Meta{Installer: "EECheck-Anwender"},
		selected: make(map[string]bool),
	}
}

// Run opens the main window and blocks until it is closed.
func (a *App) Run() {
	a.fyneApp = app.NewWithID("de.eecheck.steuerboxsimulator")
	a.win = a.fyneApp.NewWindow("EECheck – Steuerbox-Simulator")
	a.win.Resize(fyne.NewSize(1080, 720))

	a.core.Subscribe(func(eebus.Event) {
		// Any discovery/pairing/connection change may affect what the
		// currently shown screen displays; the cheapest correct behaviour
		// for a field tool is to just refresh the dashboard-derived views
		// the next time the operator navigates there. Screens that need
		// live updates (Discovery) subscribe themselves.
	})

	a.ShowDashboard()
	a.win.ShowAndRun()
}

func (a *App) setContent(c fyne.CanvasObject) {
	a.win.SetContent(c)
}

// knownDeviceName returns the best display name we have for a SKI, falling
// back to the SKI itself.
func (a *App) knownDeviceName(ski string) string {
	if kd, ok := a.devices.Get(ski); ok && kd.Name != "" {
		return kd.Name
	}
	for _, d := range a.core.DiscoveredDevices() {
		if d.Ski == ski {
			if d.Brand != "" || d.Model != "" {
				return fmt.Sprintf("%s %s", d.Brand, d.Model)
			}
			return d.Name
		}
	}
	return ski
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	return d.Round(time.Second).String()
}
