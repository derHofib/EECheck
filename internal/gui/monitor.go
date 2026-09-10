package gui

import (
	"context"
	"fmt"
	"image/color"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/orchestrator"
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

// deviceBlock is the per-device status widget from docs/03-ui-design.md
// screen 4: current step, setpoint, last reported actual value, elapsed
// time, color-coded per use case.
type deviceBlock struct {
	container *fyne.Container
	dot       *canvas.Circle
	stepLabel *widget.Label
	setLabel  *widget.Label
	actLabel  *widget.Label
}

// ShowLiveMonitor renders screen 4: one status block per device/use case,
// a scrollable filterable live log, and Abbrechen. When the run finishes
// (successfully or not) it automatically advances to the Ergebnis screen.
func (a *App) ShowLiveMonitor(run *orchestrator.TestRun, devices []orchestrator.DeviceUnderTest, names map[string]string, cancel context.CancelFunc, done <-chan struct{}) {
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
			blocks[d.SKI][uc] = &deviceBlock{container: row, dot: dot, stepLabel: stepLabel, setLabel: setLabel, actLabel: actLabel}
		}
	}

	logData := widget.NewMultiLineEntry()
	logData.Disable()
	logData.Wrapping = fyne.TextWrapWord

	var cancelled atomic.Bool

	unsubRun := a.subscribeRunEvents(run, blocks)
	unsubLog := a.subscribeMessageLog(run, logData)

	go func() {
		<-done
		unsubRun()
		unsubLog()
		if !cancelled.Load() {
			fyne.Do(func() { a.ShowResults(run, names) })
		}
	}()

	cancelBtn := widget.NewButton("Abbrechen", func() {
		cancelled.Store(true)
		cancel()
		unsubRun()
		unsubLog()
		a.ShowDashboard()
	})
	cancelBtn.Importance = widget.DangerImportance

	content := container.NewBorder(
		widget.NewLabelWithStyle("Live-Monitor – Testlauf "+run.ID, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		cancelBtn,
		nil, nil,
		container.NewVSplit(container.NewVScroll(statusRows), container.NewBorder(widget.NewLabel("Live-Nachrichtenverkehr"), nil, nil, nil, logData)),
	)
	a.setContent(content)
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

func (a *App) subscribeMessageLog(run *orchestrator.TestRun, out *widget.Entry) func() {
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
