// Package gui implements the native Fyne UI: a persistent tab bar
// (Dashboard, Test, Manuelle Steuerung, Anlage & Kunde) rather than
// full-window screen swaps, so the operator always has device status,
// the current Anlage/Kunde context, and navigation in view.
package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"github.com/derHofib/EECheck/internal/eebus"
	"github.com/derHofib/EECheck/internal/messagelog"
	"github.com/derHofib/EECheck/internal/orchestrator"
	"github.com/derHofib/EECheck/internal/report"
	"github.com/derHofib/EECheck/internal/store"
)

// App wires the GUI to the backend and holds the (built-once) tab content.
type App struct {
	fyneApp fyne.App
	win     fyne.Window

	core      *eebus.Core
	orch      *orchestrator.Orchestrator
	devices   *store.DeviceStore
	runs      *store.RunStore
	metaStore *store.MetaStore
	msgLog    *messagelog.Log
	dataDir   string
	meta      report.Meta

	selected map[string]bool // SKI -> selected for the next test run
	status   *connStatus

	tabs *container.AppTabs

	// Dashboard widgets that need updating from outside dashboard.go
	anlageHeader *anlageHeaderWidget

	// Test tab: single swappable content area within the tab
	testStack *fyne.Container

	// Manual tab: rebuilt device/use-case list on device changes
	manual *manualTabState
}

// NewApp builds the GUI shell over an already-started backend.
func NewApp(core *eebus.Core, orch *orchestrator.Orchestrator, devices *store.DeviceStore, runs *store.RunStore, metaStore *store.MetaStore, msgLog *messagelog.Log, dataDir string) *App {
	return &App{
		core:      core,
		orch:      orch,
		devices:   devices,
		runs:      runs,
		metaStore: metaStore,
		msgLog:    msgLog,
		dataDir:   dataDir,
		meta:      metaStore.Load(),
		selected:  make(map[string]bool),
		status:    newConnStatus(),
	}
}

// Run opens the main window and blocks until it is closed.
func (a *App) Run() {
	a.fyneApp = app.NewWithID("de.eecheck.steuerboxsimulator")
	a.win = a.fyneApp.NewWindow("EECheck – Steuerbox-Simulator")
	a.win.Resize(fyne.NewSize(1180, 780))

	dashboardRefresh := func() {}
	a.status.attach(a.core, func() { fyne.Do(dashboardRefresh) })
	a.core.Subscribe(func(eebus.Event) { fyne.Do(dashboardRefresh) })

	dashboardTab, refreshFn := a.buildDashboardTab()
	dashboardRefresh = refreshFn

	a.testStack = container.NewStack(a.buildTestConfigView())
	testTab := a.testStack

	manualTab := a.buildManualTab()
	anlageTab := a.buildAnlageTab()

	a.tabs = container.NewAppTabs(
		container.NewTabItem("Dashboard", dashboardTab),
		container.NewTabItem("Test", testTab),
		container.NewTabItem("Manuelle Steuerung", manualTab),
		container.NewTabItem("Anlage & Kunde", anlageTab),
	)
	a.tabs.SetTabLocation(container.TabLocationTop)

	a.win.SetContent(a.tabs)
	a.win.ShowAndRun()
}

// showTab switches to the tab at the given index (0=Dashboard, 1=Test,
// 2=Manuelle Steuerung, 3=Anlage & Kunde).
func (a *App) showTab(index int) {
	if a.tabs != nil {
		a.tabs.SelectIndex(index)
	}
}

// setTestContent swaps only the Test tab's inner content (config <-> live
// monitor <-> results), leaving Dashboard/Manuell/Anlage untouched.
func (a *App) setTestContent(c fyne.CanvasObject) {
	a.testStack.Objects = []fyne.CanvasObject{c}
	a.testStack.Refresh()
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

func (a *App) saveMeta(m report.Meta) {
	a.meta = m
	_ = a.metaStore.Save(m)
	if a.anlageHeader != nil {
		a.anlageHeader.update(m)
	}
}
