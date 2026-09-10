package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/orchestrator"
	"github.com/derHofib/EECheck/internal/report"
	"github.com/derHofib/EECheck/internal/store"
)

// ShowResults renders screen 5 from docs/03-ui-design.md: overall verdict,
// per-device detail, and export actions. Reports are also written
// automatically into the local report directory for this run so the
// history entry on the Dashboard always has something to open, even if the
// operator never clicks an export button.
func (a *App) ShowResults(run *orchestrator.TestRun, names map[string]string) {
	reportDir := filepath.Join(a.dataDir, "reports", run.ID)
	_ = os.MkdirAll(reportDir, 0o755)

	deviceNames := make([]string, 0, len(names))
	for _, n := range names {
		deviceNames = append(deviceNames, n)
	}

	a.exportAllReports(run, reportDir)

	_ = a.runs.Save(store.RunRecord{
		ID:          run.ID,
		StartedAt:   run.StartedAt,
		EndedAt:     run.EndedAt,
		SiteAddress: a.meta.SiteAddress,
		Status:      run.Status,
		DeviceNames: deviceNames,
		ReportDir:   reportDir,
	})

	verdict := widget.NewLabelWithStyle(verdictText(run.Status), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	details := container.NewVBox()
	for _, ski := range sortedSKIs(run.Devices) {
		d := run.Devices[ski]
		header := widget.NewLabelWithStyle(fmt.Sprintf("%s (%s) — %s", names[ski], ski, d.Status), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		card := container.NewVBox(header)
		for _, uc := range sortedUCs(d.UseCases) {
			res := d.UseCases[uc]
			card.Add(widget.NewLabel(fmt.Sprintf("  %s: %s (Nominal %.0f W)", uc, res.Status, res.NominalMaxW)))
			for _, step := range res.Steps {
				actual := "-"
				if step.ActualKnown {
					actual = fmt.Sprintf("%.0f W", step.ActualW)
				}
				line := fmt.Sprintf("    %s: Soll %.0f W, Ist %s, %s", step.Step.Name, step.SetpointW, actual, passLabel(step.Pass))
				if !step.Pass && step.FailReason != "" {
					line += " – " + step.FailReason
				}
				card.Add(widget.NewLabel(line))
			}
		}
		accordion := widget.NewAccordion(widget.NewAccordionItem(fmt.Sprintf("%s (%s)", names[ski], ski), card))
		details.Add(accordion)
	}

	exportPDFBtn := widget.NewButton("PDF exportieren (bereits gespeichert unter "+reportDir+")", func() {
		dialog.ShowInformation("Bereits gespeichert", "Die PDF-Berichte und der Rohlog wurden automatisch unter\n\n"+reportDir+"\n\nabgelegt.", a.win)
	})
	rawLogBtn := widget.NewButton("Rohlog exportieren", func() {
		dialog.ShowInformation("Bereits gespeichert", "Der vollständige Rohlog liegt als raw-log.ndjson unter\n\n"+reportDir, a.win)
	})
	retestBtn := widget.NewButton("Erneut testen", func() { a.ShowDashboard() })

	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Ergebnis / Report", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			verdict,
		),
		container.NewHBox(exportPDFBtn, rawLogBtn, retestBtn),
		nil, nil,
		container.NewVScroll(details),
	)
	a.setContent(content)
}

func (a *App) exportAllReports(run *orchestrator.TestRun, dir string) {
	for ski, d := range run.Devices {
		path := filepath.Join(dir, sanitizeFilename(ski)+".pdf")
		f, err := os.Create(path)
		if err != nil {
			continue
		}
		_ = report.GenerateDevicePDF(f, run, d, a.meta)
		_ = f.Close()
	}

	if f, err := os.Create(filepath.Join(dir, "gesamtbericht.pdf")); err == nil {
		_ = report.GenerateOverallPDF(f, run, a.meta)
		_ = f.Close()
	}

	if f, err := os.Create(filepath.Join(dir, "raw-log.ndjson")); err == nil {
		_ = a.msgLog.WriteNDJSONBetween(f, run.StartedAt, run.EndedAt)
		_ = f.Close()
	}
}

func sanitizeFilename(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == ':' || r == '/' || r == '\\' || r == ' ' {
			out = append(out, '_')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func verdictText(s orchestrator.RunStatus) string {
	switch s {
	case orchestrator.StatusPassed:
		return "Gesamtergebnis: BESTANDEN"
	case orchestrator.StatusFailed:
		return "Gesamtergebnis: NICHT BESTANDEN"
	default:
		return "Gesamtergebnis: FEHLER"
	}
}

func passLabel(pass bool) string {
	if pass {
		return "bestanden"
	}
	return "fehlgeschlagen"
}

func sortedSKIs(m map[string]*orchestrator.DeviceRunResult) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedUCs(m map[domainmodel.UseCaseID]*orchestrator.UseCaseRunResult) []domainmodel.UseCaseID {
	out := make([]domainmodel.UseCaseID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
