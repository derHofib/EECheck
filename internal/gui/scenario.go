package gui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/orchestrator"
	"github.com/derHofib/EECheck/internal/usecase"
)

// ShowScenarioConfig renders screen 3 from docs/03-ui-design.md: per
// selected device, which use cases to test, using the standard 0/30/60/100%
// scenario by default with an optional custom hold time, and a summary
// before starting.
func (a *App) ShowScenarioConfig(devices []orchestrator.LiveEntity) {
	// which use cases are enabled per device, defaulting to "all supported"
	enabled := make(map[string]map[domainmodel.UseCaseID]bool)
	for _, d := range devices {
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
	for _, d := range devices {
		d := d
		name := a.knownDeviceName(d.SKI)
		header := widget.NewLabelWithStyle(fmt.Sprintf("%s  (%s)", name, d.SKI), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		checks := container.NewHBox()
		for _, uc := range d.UseCases {
			uc := uc
			c := widget.NewCheck(useCaseDisplayName(uc), func(v bool) {
				enabled[d.SKI][uc] = v
			})
			c.SetChecked(true)
			checks.Add(c)
		}
		deviceBoxes.Add(container.NewVBox(header, checks, widget.NewSeparator()))
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
		for _, d := range devices {
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
			return
		}

		runID := fmt.Sprintf("run-%s", time.Now().Format("20060102-150405"))
		run := a.orch.NewTestRun(runID, dut, steps, tolerance)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		a.ShowLiveMonitor(run, dut, names, cancel, done)

		go func() {
			a.orch.Run(ctx, run, dut)
			close(done)
		}()
	})
	startBtn.Importance = widget.HighImportance

	backBtn := widget.NewButton("Zurück", func() { a.ShowDashboard() })

	form := widget.NewForm(widget.NewFormItem("Haltezeit je Stufe (Sekunden)", holdEntry))

	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Szenario-Konfiguration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			summary,
			form,
		),
		container.NewHBox(backBtn, startBtn),
		nil, nil,
		container.NewVScroll(deviceBoxes),
	)
	a.setContent(content)
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
