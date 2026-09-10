// Test tab: device/use-case selection + scenario config, live monitor, and
// results, all swapped within the Test tab's own content area
// (App.setTestContent) so Dashboard/Manuell/Anlage stay untouched.
package gui

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/orchestrator"
	"github.com/derHofib/EECheck/internal/report"
	"github.com/derHofib/EECheck/internal/store"
	"github.com/derHofib/EECheck/internal/usecase"
)

var (
	statusColorPending = color.NRGBA{R: 0x9e, G: 0x9e, B: 0x9e, A: 0xff} // grau
	statusColorRunning = color.NRGBA{R: 0xf9, G: 0xa8, B: 0x25, A: 0xff} // gelb
	statusColorPass    = color.NRGBA{R: 0x2e, G: 0x7d, B: 0x32, A: 0xff} // grün
	statusColorFail    = color.NRGBA{R: 0xc6, G: 0x28, B: 0x28, A: 0xff} // rot
)

func colorForStatus(s orchestrator.RunStatus) color.Color {
	switch s {
	case orchestrator.StatusRunning:
		return statusColorRunning
	case orchestrator.StatusPassed:
		return statusColorPass
	case orchestrator.StatusFailed, orchestrator.StatusError:
		return statusColorFail
	default:
		return statusColorPending
	}
}

func useCaseDisplayName(id domainmodel.UseCaseID) string {
	switch id {
	case domainmodel.UseCaseLPC:
		return "LPC – Verbrauchsbegrenzung"
	case domainmodel.UseCaseLPP:
		return "LPP – Einspeisebegrenzung"
	default:
		return string(id)
	}
}

// ---- Config view ----

func (a *App) buildTestConfigView() fyne.CanvasObject {
	live := a.orch.LiveEntities()
	enabled := make(map[string]map[domainmodel.UseCaseID]bool)
	for _, d := range live {
		m := make(map[domainmodel.UseCaseID]bool)
		for _, uc := range d.UseCases {
			m[uc] = true
		}
		enabled[d.SKI] = m
	}

	holdEntry := widget.NewEntry()
	holdEntry.SetText("10")
	holdEntry.Validator = func(s string) error {
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("bitte eine Zahl (Sekunden) eingeben")
		}
		return nil
	}

	deviceBoxes := container.NewVBox()
	if len(live) == 0 {
		deviceBoxes.Add(widget.NewLabel("Keine gepairten Geräte. Bitte zuerst im Dashboard \"Neues Gerät suchen\"."))
	}
	for _, d := range live {
		d := d
		name := a.knownDeviceName(d.SKI)
		selectCheck := widget.NewCheck(fmt.Sprintf("%s  (%s)", name, d.SKI), func(v bool) { a.selected[d.SKI] = v })
		selectCheck.SetChecked(a.selected[d.SKI])
		ucChecks := container.NewHBox()
		for _, uc := range d.UseCases {
			uc := uc
			c := widget.NewCheck(useCaseDisplayName(uc), func(v bool) { enabled[d.SKI][uc] = v })
			c.SetChecked(true)
			ucChecks.Add(c)
		}
		deviceBoxes.Add(container.NewVBox(selectCheck, ucChecks, widget.NewSeparator()))
	}

	summary := widget.NewLabel("Standard-Szenario: 0 % / 30 % / 60 % / 100 % der Nominalleistung je Anwendungsfall.")

	startBtn := widget.NewButton("Testlauf starten", func() {
		holdSeconds, _ := strconv.Atoi(holdEntry.Text)
		if holdSeconds < 0 {
			holdSeconds = 0
		}
		steps := usecase.DefaultScenarioSteps(time.Duration(holdSeconds) * time.Second)
		tolerance := usecase.DefaultTolerance()

		var dut []orchestrator.DeviceUnderTest
		names := make(map[string]string)
		for _, d := range live {
			if !a.selected[d.SKI] {
				continue
			}
			var chosen []domainmodel.UseCaseID
			for _, uc := range d.UseCases {
				if enabled[d.SKI][uc] {
					chosen = append(chosen, uc)
				}
			}
			if len(chosen) == 0 {
				continue
			}
			name := a.knownDeviceName(d.SKI)
			names[d.SKI] = name
			dut = append(dut, orchestrator.DeviceUnderTest{
				Entity:   d.Entity,
				SKI:      d.SKI,
				Name:     name,
				UseCases: chosen,
			})
		}
		if len(dut) == 0 {
			dialog.ShowInformation("Keine Geräte ausgewählt", "Bitte mindestens ein Gerät mit mindestens einem Anwendungsfall auswählen.", a.win)
			return
		}

		runID := fmt.Sprintf("run-%s", time.Now().Format("20060102-150405"))
		run := a.orch.NewTestRun(runID, dut, steps, tolerance)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		a.setTestContent(a.buildLiveMonitorView(run, dut, names, cancel, done))

		go func() {
			a.orch.Run(ctx, run, dut)
			close(done)
		}()
	})
	startBtn.Importance = widget.HighImportance

	form := widget.NewForm(widget.NewFormItem("Haltezeit je Stufe (Sekunden)", holdEntry))

	history := a.buildRunHistoryList()

	configColumn := container.NewVBox(
		widget.NewLabelWithStyle("Geräte & Anwendungsfälle für diesen Testlauf", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		deviceBoxes,
		widget.NewSeparator(),
		summary,
		form,
		startBtn,
	)

	split := container.NewHSplit(container.NewVScroll(configColumn), history)
	split.Offset = 0.65

	return container.NewBorder(
		widget.NewLabelWithStyle("Test", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		split,
	)
}

func (a *App) buildRunHistoryList() fyne.CanvasObject {
	runs := a.runs.All()
	list := widget.NewList(
		func() int { return len(runs) },
		func() fyne.CanvasObject { return widget.NewLabel("Testlauf") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			r := runs[id]
			obj.(*widget.Label).SetText(fmt.Sprintf("%s\n%s\n%s",
				r.StartedAt.Format("02.01.2006 15:04"), deviceNamesLabel(r.DeviceNames), string(r.Status)))
		},
	)
	return container.NewBorder(
		widget.NewLabelWithStyle("Vergangene Testläufe", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil, list)
}

func deviceNamesLabel(names []string) string {
	if len(names) == 0 {
		return "-"
	}
	out := names[0]
	for _, n := range names[1:] {
		out += ", " + n
	}
	return out
}

// ---- Live-Monitor view ----

type deviceBlock struct {
	dot       *canvas.Circle
	stepLabel *widget.Label
	setLabel  *widget.Label
	actLabel  *widget.Label
}

func (a *App) buildLiveMonitorView(run *orchestrator.TestRun, devices []orchestrator.DeviceUnderTest, names map[string]string, cancel context.CancelFunc, done <-chan struct{}) fyne.CanvasObject {
	blocks := make(map[string]map[domainmodel.UseCaseID]*deviceBlock)
	statusRows := container.NewVBox()

	for _, d := range devices {
		blocks[d.SKI] = make(map[domainmodel.UseCaseID]*deviceBlock)
		header := widget.NewLabelWithStyle(fmt.Sprintf("%s  (%s)", names[d.SKI], d.SKI), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		statusRows.Add(header)
		for _, uc := range d.UseCases {
			dot := canvas.NewCircle(statusColorPending)
			dotBox := container.NewGridWrap(fyne.NewSize(16, 16), dot)
			stepLabel := widget.NewLabel("wartet …")
			setLabel := widget.NewLabel("Soll: -")
			actLabel := widget.NewLabel("Ist: -")
			row := container.NewHBox(dotBox, widget.NewLabel(useCaseDisplayName(uc)+":"), stepLabel, setLabel, actLabel)
			statusRows.Add(row)
			blocks[d.SKI][uc] = &deviceBlock{dot: dot, stepLabel: stepLabel, setLabel: setLabel, actLabel: actLabel}
		}
	}

	logData := widget.NewMultiLineEntry()
	logData.Disable()
	logData.Wrapping = fyne.TextWrapWord

	var cancelled atomic.Bool

	unsubRun := a.subscribeRunEvents(run, blocks)
	unsubLog := a.subscribeMessageLogInto(logData)

	go func() {
		<-done
		unsubRun()
		unsubLog()
		if !cancelled.Load() {
			fyne.Do(func() { a.setTestContent(a.buildResultsView(run, names)) })
		}
	}()

	cancelBtn := widget.NewButton("Abbrechen", func() {
		cancelled.Store(true)
		cancel()
		unsubRun()
		unsubLog()
		a.setTestContent(a.buildTestConfigView())
	})
	cancelBtn.Importance = widget.DangerImportance

	return container.NewBorder(
		widget.NewLabelWithStyle("Live-Monitor – Testlauf "+run.ID, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		cancelBtn,
		nil, nil,
		container.NewVSplit(container.NewVScroll(statusRows), container.NewBorder(widget.NewLabel("Live-Nachrichtenverkehr"), nil, nil, nil, logData)),
	)
}

func (a *App) subscribeRunEvents(run *orchestrator.TestRun, blocks map[string]map[domainmodel.UseCaseID]*deviceBlock) func() {
	events, unsubscribe := a.orch.Subscribe()
	go func() {
		for ev := range events {
			if ev.RunID != run.ID {
				continue
			}
			ev := ev
			fyne.Do(func() {
				b, ok := blocks[ev.SKI][ev.UseCase]
				if !ok {
					return
				}
				b.dot.FillColor = colorForStatus(ev.Status)
				b.dot.Refresh()
				if ev.StepName != "" {
					b.stepLabel.SetText(ev.StepName)
				}
				if ev.Step != nil {
					b.setLabel.SetText(fmt.Sprintf("Soll: %.0f W", ev.Step.SetpointW))
					if ev.Step.ActualKnown {
						b.actLabel.SetText(fmt.Sprintf("Ist: %.0f W", ev.Step.ActualW))
					} else {
						b.actLabel.SetText("Ist: unbekannt")
					}
					b.stepLabel.SetText(ev.Step.Step.Name)
				}
			})
		}
	}()
	return unsubscribe
}

func (a *App) subscribeMessageLogInto(out *widget.Entry) func() {
	entries, unsubscribe := a.msgLog.Subscribe()
	go func() {
		for e := range entries {
			e := e
			fyne.Do(func() {
				line := fmt.Sprintf("[%s] %s  %s/%s  %s\n", e.Timestamp.Format("15:04:05.000"), e.SKI, e.EventType, e.Function, string(e.Data))
				out.SetText(out.Text + line)
			})
		}
	}()
	return unsubscribe
}

// ---- Results view ----

func (a *App) buildResultsView(run *orchestrator.TestRun, names map[string]string) fyne.CanvasObject {
	reportDir := filepath.Join(a.dataDir, "reports", run.ID)
	_ = os.MkdirAll(reportDir, 0o755)

	deviceNames := make([]string, 0, len(names))
	deviceResults := make(map[string]orchestrator.RunStatus, len(run.Devices))
	for ski, dr := range run.Devices {
		deviceNames = append(deviceNames, names[ski])
		deviceResults[ski] = dr.Status
	}

	a.exportAllReports(run, reportDir)

	_ = a.runs.Save(store.RunRecord{
		ID:            run.ID,
		StartedAt:     run.StartedAt,
		EndedAt:       run.EndedAt,
		SiteAddress:   a.meta.SiteAddress,
		Status:        run.Status,
		DeviceNames:   deviceNames,
		DeviceResults: deviceResults,
		ReportDir:     reportDir,
	})

	verdict := widget.NewLabelWithStyle(verdictText(run.Status), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	details := container.NewVBox()
	for _, ski := range sortedSKIs(run.Devices) {
		d := run.Devices[ski]
		card := container.NewVBox()
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
		accordion := widget.NewAccordion(widget.NewAccordionItem(fmt.Sprintf("%s (%s) — %s", names[ski], ski, d.Status), card))
		details.Add(accordion)
	}

	openFolderBtn := widget.NewButton("Berichtsordner anzeigen", func() {
		dialog.ShowInformation("Bereits gespeichert", "PDF-Berichte und Rohlog liegen unter\n\n"+reportDir, a.win)
	})
	retestBtn := widget.NewButton("Erneut testen", func() { a.setTestContent(a.buildTestConfigView()) })
	retestBtn.Importance = widget.HighImportance

	return container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Ergebnis", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			verdict,
		),
		container.NewHBox(openFolderBtn, retestBtn),
		nil, nil,
		container.NewVScroll(details),
	)
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
