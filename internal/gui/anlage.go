// Anlage & Kunde tab: master data (Errichter, Techniker, Kunde,
// Netzanschlusspunkt, Notizen) that is persisted and stamped onto every
// PDF-Prüfprotokoll (internal/report.Meta).
package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/derHofib/EECheck/internal/report"
)

func (a *App) buildAnlageTab() fyne.CanvasObject {
	installerEntry := widget.NewEntry()
	technicianEntry := widget.NewEntry()
	customerEntry := widget.NewEntry()
	siteEntry := widget.NewEntry()
	notesEntry := widget.NewMultiLineEntry()
	notesEntry.Wrapping = fyne.TextWrapWord
	notesEntry.SetMinRowsVisible(4)

	load := func(m report.Meta) {
		installerEntry.SetText(m.Installer)
		technicianEntry.SetText(m.Technician)
		customerEntry.SetText(m.Customer)
		siteEntry.SetText(m.SiteAddress)
		notesEntry.SetText(m.Notes)
	}
	load(a.meta)

	statusLabel := widget.NewLabel("")

	saveBtn := widget.NewButton("Speichern", func() {
		m := report.Meta{
			Installer:   installerEntry.Text,
			Technician:  technicianEntry.Text,
			Customer:    customerEntry.Text,
			SiteAddress: siteEntry.Text,
			Notes:       notesEntry.Text,
		}
		a.saveMeta(m)
		statusLabel.SetText("Gespeichert – wird für alle künftigen Prüfprotokolle verwendet.")
	})
	saveBtn.Importance = widget.HighImportance

	form := widget.NewForm(
		widget.NewFormItem("Errichter/Betrieb", installerEntry),
		widget.NewFormItem("Durchgeführt von", technicianEntry),
		widget.NewFormItem("Kunde", customerEntry),
		widget.NewFormItem("Anlage / Netzanschlusspunkt", siteEntry),
		widget.NewFormItem("Notizen", notesEntry),
	)

	content := container.NewVBox(
		widget.NewLabelWithStyle("Anlage & Kunde", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Diese Angaben erscheinen auf jedem PDF-Prüfprotokoll (Einzelgerät und Gesamtbericht)."),
		form,
		saveBtn,
		statusLabel,
	)

	return container.NewVScroll(content)
}
