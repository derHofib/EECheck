package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/orchestrator"
)

// ShowDashboard renders screen 1 from docs/03-ui-design.md: currently
// known devices, "Neues Gerät suchen", multi-select for a new test run,
// and the history of past test runs.
func (a *App) ShowDashboard() {
	live := a.orch.LiveEntities()

	deviceList := widget.NewList(
		func() int { return len(live) },
		func() fyne.CanvasObject {
			check := widget.NewCheck("", func(bool) {})
			label := widget.NewLabel("Gerät")
			label.TextStyle = fyne.TextStyle{Bold: true}
			sub := widget.NewLabel("Anwendungsfälle")
			sub.TextStyle = fyne.TextStyle{Italic: true}
			return container.NewBorder(nil, nil, check, nil, container.NewVBox(label, sub))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			e := live[id]
			name := a.knownDeviceName(e.SKI)

			root := obj.(*fyne.Container)
			check := root.Objects[1].(*widget.Check)
			vbox := root.Objects[0].(*fyne.Container)
			label := vbox.Objects[0].(*widget.Label)
			sub := vbox.Objects[1].(*widget.Label)

			label.SetText(fmt.Sprintf("%s   (%s)", name, e.SKI))
			sub.SetText("Anwendungsfälle: " + useCaseListLabel(e.UseCases))
			check.OnChanged = func(v bool) { a.selected[e.SKI] = v }
			check.SetChecked(a.selected[e.SKI])
		},
	)

	discoverBtn := widget.NewButton("Neues Gerät suchen", func() {
		a.ShowDiscovery()
	})
	discoverBtn.Importance = widget.HighImportance

	startRunBtn := widget.NewButton("Testlauf starten", func() {
		chosen := make([]orchestrator.LiveEntity, 0)
		for _, e := range live {
			if a.selected[e.SKI] {
				chosen = append(chosen, e)
			}
		}
		if len(chosen) == 0 {
			dialog.ShowInformation("Keine Geräte ausgewählt", "Bitte mindestens ein Gerät für den Testlauf auswählen.", a.win)
			return
		}
		a.ShowScenarioConfig(chosen)
	})

	if len(live) == 0 {
		startRunBtn.Disable()
	}

	runHistory := a.runs.All()
	historyList := widget.NewList(
		func() int { return len(runHistory) },
		func() fyne.CanvasObject {
			return widget.NewLabel("Testlauf")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			r := runHistory[id]
			label := obj.(*widget.Label)
			label.SetText(fmt.Sprintf("%s   %s   %s   %s",
				r.StartedAt.Format("02.01.2006 15:04"), r.SiteAddress, deviceNamesLabel(r.DeviceNames), string(r.Status)))
		},
	)

	top := container.NewHBox(discoverBtn, startRunBtn)
	deviceSection := container.NewBorder(
		widget.NewLabelWithStyle("Gepairte Geräte", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil, deviceList)
	historySection := container.NewBorder(
		widget.NewLabelWithStyle("Vergangene Testläufe", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil, historyList)

	split := container.NewVSplit(deviceSection, historySection)
	split.Offset = 0.6

	content := container.NewBorder(top, nil, nil, nil, split)
	a.setContent(content)
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

func deviceNamesLabel(names []string) string {
	if len(names) == 0 {
		return "-"
	}
	return strings.Join(names, ", ")
}
