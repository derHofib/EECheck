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

// showDiscoveryDialog opens the live mDNS discovery list as an overlay
// dialog on top of the current tab, so the persistent Dashboard/Test/
// Manuell/Anlage tabs stay in place. Trust confirmation here is
// certificate/SKI-based, not PIN-based - see docs/05-recherche-antworten.md
// section 4 for why the dialog shows the SKI and a trust decision rather
// than a PIN entry field.
func (a *App) showDiscoveryDialog() {
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

	unsubscribe := func() {}
	unsubscribe = a.subscribeDiscovery(func(ev eebus.Event) {
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

	content := container.NewBorder(
		widget.NewLabel("Gefundene EEBus-Geräte im lokalen Netz:"),
		statusLabel,
		nil, nil,
		list,
	)

	d := dialog.NewCustom("Neues Gerät suchen", "Schließen", content, a.win)
	d.Resize(fyne.NewSize(760, 520))
	d.SetOnClosed(unsubscribe)
	d.Show()
}

// subscribeDiscovery is a thin wrapper so the dialog's subscription is
// clearly scoped (Core.Subscribe itself has no unsubscribe - the returned
// func here just stops forwarding events to this particular dialog).
func (a *App) subscribeDiscovery(fn func(eebus.Event)) func() {
	active := true
	a.core.Subscribe(func(ev eebus.Event) {
		if active {
			fn(ev)
		}
	})
	return func() { active = false }
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
