package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/report"
)

// anlageHeaderWidget shows the current Anlage/Kunde context at the top of
// the Dashboard tab, kept in sync from the Anlage & Kunde tab via
// App.saveMeta.
type anlageHeaderWidget struct {
	title *widget.Label
	sub   *widget.Label
}

func newAnlageHeaderWidget(m report.Meta) (*anlageHeaderWidget, fyne.CanvasObject) {
	title := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	sub := widget.NewLabel("")
	h := &anlageHeaderWidget{title: title, sub: sub}
	h.update(m)
	return h, container.NewVBox(title, sub, widget.NewSeparator())
}

func (h *anlageHeaderWidget) update(m report.Meta) {
	name := m.Customer
	if name == "" {
		name = "(kein Kunde hinterlegt – siehe Tab \"Anlage & Kunde\")"
	}
	h.title.SetText("Anlage/Kunde: " + name)
	addr := m.SiteAddress
	if addr == "" {
		addr = "-"
	}
	h.sub.SetText("Netzanschlusspunkt: " + addr)
}

// buildDashboardTab builds screen 1: Anlagen-Kontext, gepairte Geräte mit
// Live-Status, und eine kompakte Live-Nachrichtenansicht. Returns the tab
// content and a refresh function for the device list (called on core/status
// events).
func (a *App) buildDashboardTab() (fyne.CanvasObject, func()) {
	header, headerBox := newAnlageHeaderWidget(a.meta)
	a.anlageHeader = header

	discoverBtn := widget.NewButton("Neues Gerät suchen", func() { a.showDiscoveryDialog() })
	discoverBtn.Importance = widget.HighImportance
	editAnlageBtn := widget.NewButton("Anlage & Kunde bearbeiten", func() { a.showTab(3) })
	startRunBtn := widget.NewButton("Testlauf starten", func() { a.showTab(1) })

	var live []liveDeviceRow
	var list *widget.List

	refresh := func() {
		live = a.buildLiveDeviceRows()
		if list != nil {
			list.Refresh()
		}
	}

	list = widget.NewList(
		func() int { return len(live) },
		func() fyne.CanvasObject {
			dot := canvas.NewCircle(statusColorPending)
			dotBox := container.NewGridWrap(fyne.NewSize(14, 14), dot)
			check := widget.NewCheck("", func(bool) {})
			nameLabel := widget.NewLabel("Gerät")
			nameLabel.TextStyle = fyne.TextStyle{Bold: true}
			subLabel := widget.NewLabel("")
			subLabel.TextStyle = fyne.TextStyle{Italic: true}
			manualBtn := widget.NewButton("Manuell steuern", nil)
			left := container.NewHBox(dotBox, check)
			return container.NewBorder(nil, nil, left, manualBtn, container.NewVBox(nameLabel, subLabel))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row := live[id]
			root := obj.(*fyne.Container)
			vbox := root.Objects[0].(*fyne.Container)
			left := root.Objects[1].(*fyne.Container)
			manualBtn := root.Objects[2].(*widget.Button)

			dotBox := left.Objects[0].(*fyne.Container)
			dot := dotBox.Objects[0].(*canvas.Circle)
			check := left.Objects[1].(*widget.Check)
			nameLabel := vbox.Objects[0].(*widget.Label)
			subLabel := vbox.Objects[1].(*widget.Label)

			dot.FillColor = a.status.Color(row.SKI)
			dot.Refresh()
			nameLabel.SetText(fmt.Sprintf("%s   (%s)", row.Name, row.SKI))
			lastResult := "noch nicht getestet"
			if row.LastResult != "" {
				lastResult = row.LastResult
			}
			subLabel.SetText(fmt.Sprintf("Status: %s   |   Anwendungsfälle: %s   |   letzter Test: %s",
				a.status.Label(row.SKI), useCaseListLabel(row.UseCases), lastResult))
			check.OnChanged = func(v bool) { a.selected[row.SKI] = v }
			check.SetChecked(a.selected[row.SKI])
			manualBtn.OnTapped = func() {
				a.manual.selectDevice(row.SKI)
				a.showTab(2)
			}
		},
	)

	liveLog := widget.NewMultiLineEntry()
	liveLog.Disable()
	liveLog.Wrapping = fyne.TextWrapWord
	a.startLiveLogTail(liveLog, 300)

	top := container.NewVBox(
		headerBox,
		container.NewHBox(discoverBtn, startRunBtn, editAnlageBtn),
		widget.NewLabelWithStyle("Gepairte Geräte", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	deviceSection := container.NewBorder(top, nil, nil, nil, list)
	logSection := container.NewBorder(
		widget.NewLabelWithStyle("Live-Kommunikation (alle Geräte)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil, liveLog)

	split := container.NewHSplit(deviceSection, logSection)
	split.Offset = 0.55

	refresh()
	return split, refresh
}

type liveDeviceRow struct {
	SKI        string
	Name       string
	UseCases   []domainmodel.UseCaseID
	LastResult string
}

func (a *App) buildLiveDeviceRows() []liveDeviceRow {
	live := a.orch.LiveEntities()
	lastResults := a.lastResultsBySKI()
	rows := make([]liveDeviceRow, 0, len(live))
	for _, e := range live {
		rows = append(rows, liveDeviceRow{
			SKI:        e.SKI,
			Name:       a.knownDeviceName(e.SKI),
			UseCases:   e.UseCases,
			LastResult: lastResults[e.SKI],
		})
	}
	return rows
}

// lastResultsBySKI scans the run history for each device's most recent
// verdict, newest first (store.RunStore.All already returns newest-first).
func (a *App) lastResultsBySKI() map[string]string {
	result := make(map[string]string)
	for _, run := range a.runs.All() {
		for ski, status := range run.DeviceResults {
			if _, ok := result[ski]; !ok {
				result[ski] = string(status)
			}
		}
	}
	return result
}

// startLiveLogTail subscribes to the message log for the app's lifetime and
// keeps out showing only the last maxLines lines, newest at the bottom.
func (a *App) startLiveLogTail(out *widget.Entry, maxLines int) {
	entries, _ := a.msgLog.Subscribe()
	go func() {
		var lines []string
		for e := range entries {
			line := fmt.Sprintf("[%s] %s  %s/%s", e.Timestamp.Format("15:04:05.000"), e.SKI, e.EventType, e.Function)
			lines = append(lines, line)
			if len(lines) > maxLines {
				lines = lines[len(lines)-maxLines:]
			}
			text := strings.Join(lines, "\n")
			fyne.Do(func() { out.SetText(text) })
		}
	}()
}

func useCaseListLabel(ids []domainmodel.UseCaseID) string {
	if len(ids) == 0 {
		return "keiner erkannt"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = string(id)
	}
	return strings.Join(parts, " + ")
}
