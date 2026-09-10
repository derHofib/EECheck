// Package orchestrator implements the "Session-/Orchestrator" module from
// docs/02-architektur.md: it holds the state of an Anlagen-Testlauf (which
// devices, which use cases, which scenario), drives the LPC/LPP handlers
// through a scenario against one or more devices - sequentially per device,
// in parallel across devices, matching docs/01-anforderungen.md 1.4 - and
// aggregates the results for the report engine.
//
// Scope note (see docs/05-recherche-antworten.md): LPC/LPP only expose the
// commanded limit and the device's read-back of that limit
// (LoadControlLimitListData), not a live power *measurement*. This
// orchestrator's "Ist-Wert" is therefore the device's acknowledged/read-back
// limit, i.e. "did the device accept and apply the commanded limit" - not
// an independently measured actual power draw. True live power measurement
// would require also implementing the MPC/MGCP monitoring use cases, which
// is a natural, additive extension point (see docs/02-architektur.md,
// Erweiterbarkeit) but out of scope for this MVP.
package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	spineapi "github.com/enbility/spine-go/api"

	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/usecase"
)

// DeviceUnderTest names one paired device entity and which of its supported
// use cases should be exercised in a test run.
type DeviceUnderTest struct {
	Entity   spineapi.EntityRemoteInterface
	SKI      string
	Name     string
	UseCases []domainmodel.UseCaseID
}

// RunStatus is the coarse state of a run, a device-in-run, or a use-case-in-run.
type RunStatus string

const (
	StatusPending RunStatus = "pending"
	StatusRunning RunStatus = "running"
	StatusPassed  RunStatus = "passed"
	StatusFailed  RunStatus = "failed"
	StatusError   RunStatus = "error"
)

// UseCaseRunResult accumulates the step-by-step results for one use case on
// one device within a run.
type UseCaseRunResult struct {
	UseCase     domainmodel.UseCaseID
	NominalMaxW float64
	Status      RunStatus
	Steps       []usecase.StepResult
	Error       string
}

// DeviceRunResult accumulates all use-case results for one device.
type DeviceRunResult struct {
	SKI      string
	Name     string
	Status   RunStatus
	UseCases map[domainmodel.UseCaseID]*UseCaseRunResult
}

// RunEvent is published to subscribers as a test run progresses, so the
// Live-Monitor GUI can render status without polling.
type RunEvent struct {
	RunID    string
	SKI      string
	UseCase  domainmodel.UseCaseID
	Status   RunStatus
	Step     *usecase.StepResult // set once a step finishes
	StepName string               // set while a step is in flight (before result available)
}

// TestRun is one Anlagen-Testlauf: a fixed scenario executed against a set
// of devices, each against its selected use cases.
type TestRun struct {
	ID        string
	StartedAt time.Time
	EndedAt   time.Time
	Steps     []usecase.ScenarioStep
	Tolerance usecase.Tolerance
	Status    RunStatus

	mu      sync.Mutex
	Devices map[string]*DeviceRunResult // by SKI
}

// Orchestrator wires the protocol-agnostic use-case handlers together and
// runs test runs against real, paired devices.
type Orchestrator struct {
	handlers map[domainmodel.UseCaseID]usecase.Handler

	mu   sync.Mutex
	subs map[int]chan RunEvent
	next int
}

// New creates an Orchestrator over the given installed use-case handlers
// (typically lpc.New() and lpp.New(), already passed to eebus.Core.AddUseCase).
func New(handlers ...usecase.Handler) *Orchestrator {
	o := &Orchestrator{
		handlers: make(map[domainmodel.UseCaseID]usecase.Handler),
		subs:     make(map[int]chan RunEvent),
	}
	for _, h := range handlers {
		o.handlers[h.ID()] = h
	}
	return o
}

// Handler returns the installed handler for a use case, if any.
func (o *Orchestrator) Handler(id domainmodel.UseCaseID) (usecase.Handler, bool) {
	h, ok := o.handlers[id]
	return h, ok
}

// Subscribe returns a channel of run progress events and an unsubscribe func.
func (o *Orchestrator) Subscribe() (<-chan RunEvent, func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	id := o.next
	o.next++
	ch := make(chan RunEvent, 256)
	o.subs[id] = ch
	return ch, func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		delete(o.subs, id)
		close(ch)
	}
}

func (o *Orchestrator) publish(e RunEvent) {
	o.mu.Lock()
	subs := make([]chan RunEvent, 0, len(o.subs))
	for _, ch := range o.subs {
		subs = append(subs, ch)
	}
	o.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// NewTestRun creates a run definition. Call Run to execute it.
func (o *Orchestrator) NewTestRun(id string, devices []DeviceUnderTest, steps []usecase.ScenarioStep, tolerance usecase.Tolerance) *TestRun {
	run := &TestRun{
		ID:        id,
		Steps:     steps,
		Tolerance: tolerance,
		Status:    StatusPending,
		Devices:   make(map[string]*DeviceRunResult, len(devices)),
	}
	for _, d := range devices {
		ucResults := make(map[domainmodel.UseCaseID]*UseCaseRunResult, len(d.UseCases))
		for _, ucID := range d.UseCases {
			ucResults[ucID] = &UseCaseRunResult{UseCase: ucID, Status: StatusPending}
		}
		run.Devices[d.SKI] = &DeviceRunResult{SKI: d.SKI, Name: d.Name, Status: StatusPending, UseCases: ucResults}
	}
	return run
}

// Run executes the scenario against every device in parallel; within a
// device, use cases run sequentially (so LPC and LPP on the same
// bidirectional entity don't race each other's writes), and within a use
// case, scenario steps run sequentially.
func (o *Orchestrator) Run(ctx context.Context, run *TestRun, devices []DeviceUnderTest) {
	run.StartedAt = time.Now()
	run.Status = StatusRunning

	var wg sync.WaitGroup
	for _, d := range devices {
		wg.Add(1)
		go func(d DeviceUnderTest) {
			defer wg.Done()
			o.runDevice(ctx, run, d)
		}(d)
	}
	wg.Wait()

	run.EndedAt = time.Now()
	run.mu.Lock()
	run.Status = StatusPassed
	for _, dr := range run.Devices {
		if dr.Status == StatusFailed || dr.Status == StatusError {
			run.Status = dr.Status
			break
		}
	}
	run.mu.Unlock()
}

func (o *Orchestrator) runDevice(ctx context.Context, run *TestRun, d DeviceUnderTest) {
	run.mu.Lock()
	dr := run.Devices[d.SKI]
	run.mu.Unlock()

	overall := StatusPassed
	for _, ucID := range d.UseCases {
		handler, ok := o.handlers[ucID]
		if !ok {
			run.mu.Lock()
			res := dr.UseCases[ucID]
			res.Status = StatusError
			res.Error = fmt.Sprintf("kein Handler für Use Case %s installiert", ucID)
			run.mu.Unlock()
			overall = StatusError
			continue
		}

		status := o.runUseCase(ctx, run, d, handler)
		if status == StatusFailed && overall != StatusError {
			overall = StatusFailed
		} else if status == StatusError {
			overall = StatusError
		}
	}

	run.mu.Lock()
	dr.Status = overall
	run.mu.Unlock()
}

func (o *Orchestrator) runUseCase(ctx context.Context, run *TestRun, d DeviceUnderTest, handler usecase.Handler) RunStatus {
	run.mu.Lock()
	res := run.Devices[d.SKI].UseCases[handler.ID()]
	res.Status = StatusRunning
	run.mu.Unlock()

	o.publish(RunEvent{RunID: run.ID, SKI: d.SKI, UseCase: handler.ID(), Status: StatusRunning})

	nominalMaxW, err := handler.NominalMaxW(d.Entity)
	if err != nil || nominalMaxW <= 0 {
		run.mu.Lock()
		res.Status = StatusError
		res.Error = fmt.Sprintf("Nominalleistung nicht ermittelbar: %v", err)
		run.mu.Unlock()
		o.publish(RunEvent{RunID: run.ID, SKI: d.SKI, UseCase: handler.ID(), Status: StatusError})
		return StatusError
	}
	run.mu.Lock()
	res.NominalMaxW = nominalMaxW
	run.mu.Unlock()

	overall := StatusPassed
	for _, step := range run.Steps {
		select {
		case <-ctx.Done():
			return StatusError
		default:
		}

		o.publish(RunEvent{RunID: run.ID, SKI: d.SKI, UseCase: handler.ID(), Status: StatusRunning, StepName: step.Name})

		result := o.runStep(d.Entity, handler, nominalMaxW, step, run.Tolerance)

		run.mu.Lock()
		res.Steps = append(res.Steps, result)
		run.mu.Unlock()

		o.publish(RunEvent{RunID: run.ID, SKI: d.SKI, UseCase: handler.ID(), Status: StatusRunning, Step: &result})

		if !result.Pass {
			overall = StatusFailed
		}

		if step.HoldFor > 0 {
			select {
			case <-time.After(step.HoldFor):
			case <-ctx.Done():
				return StatusError
			}
		}
	}

	run.mu.Lock()
	res.Status = overall
	run.mu.Unlock()
	o.publish(RunEvent{RunID: run.ID, SKI: d.SKI, UseCase: handler.ID(), Status: overall})

	return overall
}

func (o *Orchestrator) runStep(entity spineapi.EntityRemoteInterface, handler usecase.Handler, nominalMaxW float64, step usecase.ScenarioStep, tol usecase.Tolerance) usecase.StepResult {
	setpointW := nominalMaxW * step.PercentOfNominal / 100

	result := usecase.StepResult{
		Step:          step,
		SetpointW:     setpointW,
		SentAt:        time.Now(),
		ToleranceUsed: tol,
	}

	ackCh := make(chan struct {
		ok  bool
		msg string
	}, 1)

	err := handler.WriteLimitW(entity, setpointW, true, 0, func(accepted bool, errMsg string) {
		ackCh <- struct {
			ok  bool
			msg string
		}{accepted, errMsg}
	})
	if err != nil {
		result.AckError = err.Error()
		result.Pass = false
		result.FailReason = "Schreibaufruf fehlgeschlagen: " + err.Error()
		return result
	}

	select {
	case ack := <-ackCh:
		result.AckReceived = true
		result.AckAt = time.Now()
		if !ack.ok {
			result.AckError = ack.msg
			result.Pass = false
			result.FailReason = "Gerät hat das Limit abgelehnt: " + ack.msg
			return result
		}
	case <-time.After(tol.AckTimeout):
		result.Pass = false
		result.FailReason = fmt.Sprintf("keine Quittierung innerhalb von %s", tol.AckTimeout)
		return result
	}

	// Poll the device's read-back limit until it matches the setpoint
	// within tolerance, or ActualTimeout elapses.
	deadline := time.Now().Add(tol.ActualTimeout)
	allowed := tol.AllowedDeviation(setpointW)
	for {
		valueW, isActive, err := handler.CurrentLimitW(entity)
		if err == nil && isActive {
			result.ActualKnown = true
			result.ActualW = valueW
			result.ActualAt = time.Now()
			deviation := valueW - setpointW
			if deviation < 0 {
				deviation = -deviation
			}
			if deviation <= allowed {
				result.Pass = true
				return result
			}
		}
		if time.Now().After(deadline) {
			result.Pass = false
			if result.ActualKnown {
				result.FailReason = fmt.Sprintf("Ist-Wert %.0f W weicht zu stark vom Soll-Wert %.0f W ab (Toleranz ±%.0f W)", result.ActualW, setpointW, allowed)
			} else {
				result.FailReason = fmt.Sprintf("kein gültiger Ist-Wert innerhalb von %s", tol.ActualTimeout)
			}
			return result
		}
		time.Sleep(2 * time.Second)
	}
}
