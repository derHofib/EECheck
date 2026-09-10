// Package report implements the "Report-Engine" module from
// docs/02-architektur.md: it turns a finished orchestrator.TestRun into the
// Prüfprotokoll artifacts required by docs/01-anforderungen.md section 1.6 -
// a lossless NDJSON Rohlog (delegated to internal/messagelog.Log, which
// already writes it incrementally as messages arrive) and printable PDF
// reports, per device and as an Anlagen-Gesamtbericht.
package report

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"

	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/orchestrator"
	"github.com/derHofib/EECheck/internal/usecase"
)

// Meta is the operator-supplied context printed on every report, so the
// PDF is self-contained and useful as a VNB/Kunden-Nachweis without the
// tool (docs/01-anforderungen.md acceptance criteria).
type Meta struct {
	Installer   string // Errichter/Betrieb
	Technician  string // durchführende Person
	Customer    string
	SiteAddress string // Anlagenadresse / Netzanschlusspunkt
	Notes       string
}

func newPDF() (*fpdf.Fpdf, func(string) string) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	return pdf, tr
}

func writeHeader(pdf *fpdf.Fpdf, tr func(string) string, title string, meta Meta, generatedAt time.Time) {
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, tr(title), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.Ln(2)

	rows := [][2]string{
		{"Erstellt am", generatedAt.Format("02.01.2006 15:04:05")},
		{"Errichter", meta.Installer},
		{"Durchgeführt von", meta.Technician},
		{"Kunde", meta.Customer},
		{"Anlage / Netzanschlusspunkt", meta.SiteAddress},
	}
	for _, r := range rows {
		if r[1] == "" {
			continue
		}
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(55, 6, tr(r[0]+":"), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(0, 6, tr(r[1]), "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)
}

func writeVerdictBanner(pdf *fpdf.Fpdf, tr func(string) string, status orchestrator.RunStatus) {
	label, r, g, b := verdictStyle(status)
	pdf.SetFillColor(r, g, b)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 10, tr("Gesamtergebnis: "+label), "1", 1, "C", true, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(4)
}

func verdictStyle(status orchestrator.RunStatus) (label string, r, g, b int) {
	switch status {
	case orchestrator.StatusPassed:
		return "BESTANDEN", 0x2e, 0x7d, 0x32
	case orchestrator.StatusFailed:
		return "NICHT BESTANDEN", 0xc6, 0x28, 0x28
	default:
		return "FEHLER", 0xe6, 0x51, 0x00
	}
}

func writeToleranceNote(pdf *fpdf.Fpdf, tr func(string) string) {
	pdf.SetFont("Helvetica", "I", 8)
	pdf.MultiCell(0, 4, tr(
		"Hinweis: Toleranz- und Timeout-Werte sind konfigurierbare Software-Voreinstellungen "+
			"des Errichters, keine normativen Vorgaben der EEBUS-SPINE-Spezifikation. "+
			"Die je Testfall tatsächlich verwendeten Werte sind unten ausgewiesen."), "", "L", false)
	pdf.Ln(2)
}

func writeStepsTable(pdf *fpdf.Fpdf, tr func(string) string, steps []stepRow) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(230, 230, 230)
	headers := []struct {
		label string
		width float64
	}{
		{"Schritt", 22}, {"Soll [W]", 22}, {"Ist [W]", 22}, {"Toleranz [W]", 24},
		{"Quittiert", 20}, {"Ergebnis", 25}, {"Hinweis", 55},
	}
	for _, h := range headers {
		pdf.CellFormat(h.width, 7, tr(h.label), "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 8)
	for _, s := range steps {
		fill := false
		if !s.pass {
			pdf.SetFillColor(255, 230, 230)
			fill = true
		}
		pdf.CellFormat(22, 6, tr(s.name), "1", 0, "L", fill, 0, "")
		pdf.CellFormat(22, 6, fmt.Sprintf("%.0f", s.setpointW), "1", 0, "R", fill, 0, "")
		pdf.CellFormat(22, 6, s.actual, "1", 0, "R", fill, 0, "")
		pdf.CellFormat(24, 6, fmt.Sprintf("+/- %.0f", s.toleranceW), "1", 0, "R", fill, 0, "")
		pdf.CellFormat(20, 6, s.ack, "1", 0, "C", fill, 0, "")
		result := "bestanden"
		if !s.pass {
			result = "fehlgeschlagen"
		}
		pdf.CellFormat(25, 6, tr(result), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(55, 6, tr(s.reason), "1", 1, "L", fill, 0, "")
	}
	pdf.Ln(4)
}

type stepRow struct {
	name       string
	setpointW  float64
	actual     string
	toleranceW float64
	ack        string
	pass       bool
	reason     string
}

func toStepRows(results []usecase.StepResult) []stepRow {
	rows := make([]stepRow, 0, len(results))
	for _, r := range results {
		actual := "-"
		if r.ActualKnown {
			actual = fmt.Sprintf("%.0f", r.ActualW)
		}
		ack := "nein"
		if r.AckReceived && r.AckError == "" {
			ack = "ja"
		}
		rows = append(rows, stepRow{
			name:       r.Step.Name,
			setpointW:  r.SetpointW,
			actual:     actual,
			toleranceW: r.ToleranceUsed.AllowedDeviation(r.SetpointW),
			ack:        ack,
			pass:       r.Pass,
			reason:     r.FailReason,
		})
	}
	return rows
}

// GenerateDevicePDF writes the per-device Prüfprotokoll for run, covering
// every use case that was tested on that device.
func GenerateDevicePDF(w io.Writer, run *orchestrator.TestRun, device *orchestrator.DeviceRunResult, meta Meta) error {
	pdf, tr := newPDF()
	title := fmt.Sprintf("Prüfprotokoll – %s", device.Name)
	writeHeader(pdf, tr, title, meta, time.Now())

	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Gerät (SKI): %s", device.SKI)), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Testlauf-ID: %s   Zeitraum: %s – %s", run.ID,
		run.StartedAt.Format("15:04:05"), run.EndedAt.Format("15:04:05"))), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	writeVerdictBanner(pdf, tr, device.Status)
	writeToleranceNote(pdf, tr)

	for _, ucID := range sortedUseCaseIDs(device.UseCases) {
		uc := device.UseCases[ucID]
		pdf.SetFont("Helvetica", "B", 12)
		pdf.CellFormat(0, 8, tr(fmt.Sprintf("Anwendungsfall: %s  (Nominalleistung: %.0f W)", uc.UseCase, uc.NominalMaxW)), "", 1, "L", false, 0, "")
		if uc.Error != "" {
			pdf.SetFont("Helvetica", "", 9)
			pdf.MultiCell(0, 5, tr("Fehler: "+uc.Error), "", "L", false)
			pdf.Ln(2)
			continue
		}
		writeStepsTable(pdf, tr, toStepRows(uc.Steps))
	}

	return pdf.Output(w)
}

// GenerateOverallPDF writes the Anlagen-Gesamtbericht summarizing every
// device tested in run.
func GenerateOverallPDF(w io.Writer, run *orchestrator.TestRun, meta Meta) error {
	pdf, tr := newPDF()
	writeHeader(pdf, tr, "Anlagen-Gesamtprüfprotokoll", meta, time.Now())

	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Testlauf-ID: %s   Zeitraum: %s – %s", run.ID,
		run.StartedAt.Format("02.01.2006 15:04:05"), run.EndedAt.Format("15:04:05"))), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	writeVerdictBanner(pdf, tr, run.Status)

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(230, 230, 230)
	for _, h := range []struct {
		label string
		width float64
	}{{"Gerät", 60}, {"SKI", 55}, {"Anwendungsfälle", 45}, {"Ergebnis", 30}} {
		pdf.CellFormat(h.width, 7, tr(h.label), "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	for _, ski := range sortedDeviceSKIs(run.Devices) {
		d := run.Devices[ski]
		fill := d.Status != orchestrator.StatusPassed
		if fill {
			pdf.SetFillColor(255, 230, 230)
		}
		ucLabels := ""
		for i, id := range sortedUseCaseIDs(d.UseCases) {
			if i > 0 {
				ucLabels += ", "
			}
			ucLabels += string(id)
		}
		result := "bestanden"
		if !fill {
			// passed
		} else if d.Status == orchestrator.StatusError {
			result = "Fehler"
		} else {
			result = "nicht bestanden"
		}
		pdf.CellFormat(60, 6, tr(d.Name), "1", 0, "L", fill, 0, "")
		pdf.CellFormat(55, 6, d.SKI, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(45, 6, tr(ucLabels), "1", 0, "L", fill, 0, "")
		pdf.CellFormat(30, 6, tr(result), "1", 1, "C", fill, 0, "")
	}
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "I", 8)
	pdf.MultiCell(0, 4, tr(
		"Detailergebnisse (Soll/Ist je Testschritt) siehe Einzelgeräte-Prüfprotokolle. "+
			"Der vollständige, unveränderte Rohlog dieses Testlaufs liegt als NDJSON-Datei bei."), "", "L", false)

	return pdf.Output(w)
}

func sortedUseCaseIDs(m map[domainmodel.UseCaseID]*orchestrator.UseCaseRunResult) []domainmodel.UseCaseID {
	ids := make([]domainmodel.UseCaseID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedDeviceSKIs(m map[string]*orchestrator.DeviceRunResult) []string {
	skis := make([]string, 0, len(m))
	for ski := range m {
		skis = append(skis, ski)
	}
	sort.Strings(skis)
	return skis
}
