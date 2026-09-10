package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	shipapi "github.com/enbility/ship-go/api"

	"github.com/derHofib/EECheck/internal/eebus"
	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/store"
)

// ShowDiscovery renders screen 2 from docs/03-ui-design.md: a live mDNS
// discovery list with a per-device "Pairing starten" action. Trust
// confirmation here is certificate/SKI-based, not PIN-based - see
// docs/05-recherche-antworten.md section 4 for why the UI shows the SKI
// and a trust decision rather than a PIN entry field.
func (a *App) ShowDiscovery() {
	statusLabel := widget.NewLabel("")
	var services []shipapi.RemoteMdnsService
	var list *widget.List

	refresh := func() {
		services = a.core.DiscoveredDevices()
		if list != nil {
			list.Refresh()
		}
	}
	refresh()

	list = widget.NewList(
		func() int { return len(services) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("Gerät")
			pairBtn := widget.NewButton("Pairing starten", nil)
			return container.NewBorder(nil, nil, nil, pairBtn, label)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			svc := services[id]
			root := obj.(*fyne.Container)
			label := root.Objects[0].(*widget.Label)
			pairBtn := root.Objects[1].(*widget.Button)

			role := domainmodel.RoleFromMdnsType(svc.Type)
			label.SetText(fmt.Sprintf("%s %s   SKI: %s   Typ: %s (%s)", svc.Brand, svc.Model, svc.Ski, svc.Type, role))

			if _, known := a.devices.Get(svc.Ski); known {
				pairBtn.SetText("Bereits gepairt")
				pairBtn.Disable()
			} else {
				pairBtn.SetText("Pairing starten")
				pairBtn.Enable()
				pairBtn.OnTapped = func() {
					a.startPairing(svc, statusLabel)
				}
			}
		},
	)

	a.core.Subscribe(func(ev eebus.Event) {
		switch ev.Kind {
		case eebus.EventDiscoveryUpdated:
			fyne.Do(refresh)
		case eebus.EventPairingProgress:
			fyne.Do(func() { statusLabel.SetText(fmt.Sprintf("SKI %s: %s", ev.SKI, ev.Message)) })
		case eebus.EventTrustDenied:
			fyne.Do(func() { statusLabel.SetText(fmt.Sprintf("SKI %s: Vertrauen abgelehnt (%s)", ev.SKI, ev.Message)) })
		case eebus.EventDeviceConnected:
			fyne.Do(func() {
				statusLabel.SetText(fmt.Sprintf("SKI %s: verbunden", ev.SKI))
				refresh()
			})
		}
	})

	backBtn := widget.NewButton("Zurück zum Dashboard", func() { a.ShowDashboard() })

	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Discovery & Pairing", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel("Gefundene EEBus-Geräte im lokalen Netz:"),
		),
		container.NewVBox(statusLabel, backBtn),
		nil, nil,
		list,
	)
	a.setContent(content)
}

func (a *App) startPairing(svc shipapi.RemoteMdnsService, statusLabel *widget.Label) {
	dialog.ShowConfirm(
		"Gerät vertrauen?",
		fmt.Sprintf("Diesem Gerät vertrauen und pairen?\n\nMarke/Modell: %s %s\nSKI: %s\nShip-ID: %s",
			svc.Brand, svc.Model, svc.Ski, svc.ShipID),
		func(confirmed bool) {
			if !confirmed {
				return
			}
			a.core.Pair(svc.Ski, svc.ShipID)
			statusLabel.SetText(fmt.Sprintf("SKI %s: Pairing gestartet …", svc.Ski))

			role := domainmodel.RoleFromMdnsType(svc.Type)
			name := fmt.Sprintf("%s %s", svc.Brand, svc.Model)
			_ = a.devices.Upsert(store.KnownDevice{
				SKI:      svc.Ski,
				ShipID:   svc.ShipID,
				Brand:    svc.Brand,
				Model:    svc.Model,
				Name:     name,
				Role:     role,
				LastSeen: time.Now(),
			})
		},
		a.win,
	)
}
