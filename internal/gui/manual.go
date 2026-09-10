// Manuelle Steuerung tab: pick a device + use case, send a free-form
// Watt value directly via the use-case handler and watch Soll/Ist live -
// independent of the Testlauf/Report machinery, for quick field checks
// ("funktioniert die Kommunikation überhaupt").
package gui

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	spineapi "github.com/enbility/spine-go/api"

	"github.com/derHofib/EECheck/internal/eebus"
	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/orchestrator"
)

type manualTabState struct {
	a *App

	mu      sync.Mutex
	options []orchestrator.LiveEntity // index-aligned with deviceSelect.Options

	deviceSelect *widget.Select
	ucSelect     *widget.Select
	nominalLabel *widget.Label
	currentLabel *widget.Label
	valueEntry   *widget.Entry
	activeCheck  *widget.Check
	statusLabel  *widget.Label

	stopPoll chan struct{}
}

func (a *App) buildManualTab() fyne.CanvasObject {
	m := &manualTabState{a: a}
	a.manual = m

	m.deviceSelect = widget.NewSelect(nil, func(string) { m.onDeviceChanged() })
	m.ucSelect = widget.NewSelect(nil, func(string) { m.onUseCaseChanged() })
	m.nominalLabel = widget.NewLabel("Nominalleistung: -")
	m.currentLabel = widget.NewLabel("Aktueller Wert: -")
	m.valueEntry = widget.NewEntry()
	m.valueEntry.SetPlaceHolder("Wert in Watt, z.B. 3000")
	m.activeCheck = widget.NewCheck("Limit aktiv setzen", nil)
	m.activeCheck.SetChecked(true)
	m.statusLabel = widget.NewLabel("")

	quickBtns := container.NewHBox()
	for _, pct := range []float64{0, 30, 60, 100} {
		pct := pct
		quickBtns.Add(widget.NewButton(fmt.Sprintf("%.0f %%", pct), func() {
			m.mu.Lock()
			entity, ucID, ok := m.currentSelectionLocked()
			m.mu.Unlock()
			if !ok {
				return
			}
			handler, ok := m.a.orch.Handler(ucID)
			if !ok {
				return
			}
			nominal, err := handler.NominalMaxW(entity)
			if err != nil || nominal <= 0 {
				m.statusLabel.SetText("Nominalleistung nicht bekannt - Wert manuell eingeben.")
				return
			}
			m.valueEntry.SetText(fmt.Sprintf("%.0f", nominal*pct/100))
		}))
	}

	sendBtn := widget.NewButton("Senden", func() { m.send() })
	sendBtn.Importance = widget.HighImportance

	refreshBtn := widget.NewButton("Geräte aktualisieren", func() { m.refreshDevices() })

	form := widget.NewForm(
		widget.NewFormItem("Gerät", m.deviceSelect),
		widget.NewFormItem("Anwendungsfall", m.ucSelect),
	)

	content := container.NewVBox(
		widget.NewLabelWithStyle("Manuelle Steuerung", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Direkt einen Wert an ein Gerät senden, ohne einen vollständigen Testlauf/Bericht zu erzeugen."),
		container.NewHBox(refreshBtn),
		form,
		m.nominalLabel,
		m.currentLabel,
		widget.NewLabel("Schnellwahl (% der Nominalleistung):"),
		quickBtns,
		m.valueEntry,
		m.activeCheck,
		sendBtn,
		m.statusLabel,
	)

	m.refreshDevices()
	m.a.core.Subscribe(func(eebus.Event) { fyne.Do(m.refreshDevices) })

	return container.NewVScroll(content)
}

func (m *manualTabState) refreshDevices() {
	live := m.a.orch.LiveEntities()

	m.mu.Lock()
	m.options = live
	m.mu.Unlock()

	labels := make([]string, len(live))
	for i, e := range live {
		labels[i] = fmt.Sprintf("%s (%s)", m.a.knownDeviceName(e.SKI), e.SKI)
	}
	current := m.deviceSelect.Selected
	m.deviceSelect.Options = labels
	if contains(labels, current) {
		m.deviceSelect.SetSelected(current)
	} else if len(labels) > 0 {
		m.deviceSelect.SetSelected(labels[0])
	} else {
		m.deviceSelect.ClearSelected()
		m.ucSelect.Options = nil
		m.ucSelect.ClearSelected()
	}
	m.deviceSelect.Refresh()
}

// selectDevice is called from the Dashboard's "Manuell steuern" button.
func (m *manualTabState) selectDevice(ski string) {
	m.refreshDevices()
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.options {
		if e.SKI == ski {
			m.deviceSelect.SetSelected(m.deviceSelect.Options[i])
			return
		}
	}
}

func (m *manualTabState) onDeviceChanged() {
	m.mu.Lock()
	idx := indexOf(m.deviceSelect.Options, m.deviceSelect.Selected)
	var ucLabels []string
	if idx >= 0 && idx < len(m.options) {
		for _, uc := range m.options[idx].UseCases {
			ucLabels = append(ucLabels, string(uc))
		}
	}
	m.mu.Unlock()

	m.ucSelect.Options = ucLabels
	if len(ucLabels) > 0 {
		m.ucSelect.SetSelected(ucLabels[0])
	} else {
		m.ucSelect.ClearSelected()
	}
	m.ucSelect.Refresh()
	m.onUseCaseChanged()
}

func (m *manualTabState) currentSelectionLocked() (spineapi.EntityRemoteInterface, domainmodel.UseCaseID, bool) {
	idx := indexOf(m.deviceSelect.Options, m.deviceSelect.Selected)
	if idx < 0 || idx >= len(m.options) {
		return nil, "", false
	}
	if m.ucSelect.Selected == "" {
		return nil, "", false
	}
	return m.options[idx].Entity, domainmodel.UseCaseID(m.ucSelect.Selected), true
}

func (m *manualTabState) onUseCaseChanged() {
	m.stopPolling()

	m.mu.Lock()
	entity, ucID, ok := m.currentSelectionLocked()
	m.mu.Unlock()
	if !ok {
		m.nominalLabel.SetText("Nominalleistung: -")
		m.currentLabel.SetText("Aktueller Wert: -")
		return
	}
	handler, ok := m.a.orch.Handler(ucID)
	if !ok {
		return
	}

	if nominal, err := handler.NominalMaxW(entity); err == nil && nominal > 0 {
		m.nominalLabel.SetText(fmt.Sprintf("Nominalleistung: %.0f W", nominal))
	} else {
		m.nominalLabel.SetText("Nominalleistung: nicht verfügbar")
	}

	m.startPolling(entity, ucID)
}

func (m *manualTabState) startPolling(entity spineapi.EntityRemoteInterface, ucID domainmodel.UseCaseID) {
	stop := make(chan struct{})
	m.stopPoll = stop

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			handler, ok := m.a.orch.Handler(ucID)
			if ok {
				if value, active, err := handler.CurrentLimitW(entity); err == nil {
					text := fmt.Sprintf("Aktueller Wert: %.0f W (aktiv: %v)", value, active)
					fyne.Do(func() { m.currentLabel.SetText(text) })
				}
			}
			select {
			case <-ticker.C:
			case <-stop:
				return
			}
		}
	}()
}

func (m *manualTabState) stopPolling() {
	if m.stopPoll != nil {
		close(m.stopPoll)
		m.stopPoll = nil
	}
}

func (m *manualTabState) send() {
	m.mu.Lock()
	entity, ucID, ok := m.currentSelectionLocked()
	m.mu.Unlock()
	if !ok {
		m.statusLabel.SetText("Bitte Gerät und Anwendungsfall wählen.")
		return
	}
	handler, ok := m.a.orch.Handler(ucID)
	if !ok {
		m.statusLabel.SetText("Kein Handler für diesen Anwendungsfall installiert.")
		return
	}
	value, err := strconv.ParseFloat(m.valueEntry.Text, 64)
	if err != nil {
		m.statusLabel.SetText("Ungültiger Wert.")
		return
	}

	m.statusLabel.SetText("Sende …")
	err = handler.WriteLimitW(entity, value, m.activeCheck.Checked, 0, func(accepted bool, errMsg string) {
		fyne.Do(func() {
			if accepted {
				m.statusLabel.SetText(fmt.Sprintf("Angenommen: %.0f W (aktiv: %v)", value, m.activeCheck.Checked))
			} else {
				m.statusLabel.SetText("Abgelehnt: " + errMsg)
			}
		})
	})
	if err != nil {
		m.statusLabel.SetText("Fehler beim Senden: " + err.Error())
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}
