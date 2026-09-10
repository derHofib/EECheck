// Package usecase defines the shared contract implemented by the concrete
// LPC (internal/usecase/lpc) and LPP (internal/usecase/lpp) handlers, so the
// orchestrator (internal/orchestrator) can drive either without knowing
// SPINE/eebus-go details - matching the "Use-Case-Handler" module boundary
// from docs/02-architektur.md.
package usecase

import (
	"time"

	spineapi "github.com/enbility/spine-go/api"

	"github.com/derHofib/EECheck/internal/eebus"
	domainmodel "github.com/derHofib/EECheck/internal/model"
)

// Direction is the energy flow direction a use case limits.
type Direction string

const (
	DirectionConsume Direction = "consume" // LPC
	DirectionProduce Direction = "produce" // LPP - wire value is negative Watts, see docs/05-recherche-antworten.md
)

// ScenarioStep is one setpoint of a test scenario, expressed as a percentage
// of the device's nominal max power. docs/01-anforderungen.md asks for the
// classic 0/30/60/100% steps as the default, with free values as an option.
type ScenarioStep struct {
	Name             string
	PercentOfNominal float64       // 0..100
	HoldFor          time.Duration // how long to keep this step active before moving to the next one
}

// DefaultScenarioSteps returns the standard 0/30/60/100% test sequence.
func DefaultScenarioSteps(holdFor time.Duration) []ScenarioStep {
	pcts := []struct {
		name string
		pct  float64
	}{
		{"0 %", 0},
		{"30 %", 30},
		{"60 %", 60},
		{"100 %", 100},
	}
	steps := make([]ScenarioStep, 0, len(pcts))
	for _, p := range pcts {
		steps = append(steps, ScenarioStep{Name: p.name, PercentOfNominal: p.pct, HoldFor: holdFor})
	}
	return steps
}

// Tolerance defines when an observed value counts as matching a setpoint,
// and how long to wait for acknowledgement / actual-value convergence.
// These are operator-configurable software defaults, NOT verified spec
// values - see docs/05-recherche-antworten.md section 7.
type Tolerance struct {
	RelativePercent float64       // e.g. 5 -> ±5% of setpoint
	AbsoluteWatts   float64       // e.g. 100 -> ±100 W, whichever tolerance band is larger applies
	AckTimeout      time.Duration // time to wait for the write to be acknowledged/rejected
	ActualTimeout   time.Duration // time to wait for the reported actual power to converge
}

// DefaultTolerance is the tool's built-in default; always shown to the
// operator in the report so the Prüfprotokoll stays honest about what was
// actually checked.
func DefaultTolerance() Tolerance {
	return Tolerance{
		RelativePercent: 5,
		AbsoluteWatts:   100,
		AckTimeout:      30 * time.Second,
		ActualTimeout:   90 * time.Second,
	}
}

// AllowedDeviation returns the wider of the relative/absolute tolerance
// bands for a given setpoint, in Watts.
func (t Tolerance) AllowedDeviation(setpointW float64) float64 {
	rel := setpointW * t.RelativePercent / 100
	if rel < 0 {
		rel = -rel
	}
	if rel > t.AbsoluteWatts {
		return rel
	}
	return t.AbsoluteWatts
}

// StepResult captures the observed Soll/Ist behaviour of one scenario step
// against one device/use case, for pass/fail evaluation and the report.
type StepResult struct {
	Step ScenarioStep

	SetpointW float64
	SentAt    time.Time

	AckReceived bool
	AckAt       time.Time
	AckError    string

	ActualKnown bool
	ActualW     float64
	ActualAt    time.Time

	ToleranceUsed Tolerance
	Pass          bool
	FailReason    string
}

// WriteResultFunc is invoked once eebus-go reports the write result
// (accepted/rejected) for a WriteLimitW call.
type WriteResultFunc func(accepted bool, errMsg string)

// Handler is the use-case-specific surface the orchestrator drives. It
// extends eebus.UseCaseAdapter (ID/Install/SupportedEntities, used by
// internal/eebus.Core.AddUseCase) with the scenario-execution operations.
type Handler interface {
	eebus.UseCaseAdapter

	Direction() Direction

	// NominalMaxW returns the device's nominal max power in Watts (always
	// positive), used to convert scenario percentages to absolute setpoints.
	NominalMaxW(entity spineapi.EntityRemoteInterface) (float64, error)

	// CurrentLimitW returns the currently reported limit in Watts (signed
	// per Direction for LPP) and whether it is active.
	CurrentLimitW(entity spineapi.EntityRemoteInterface) (valueW float64, isActive bool, err error)

	// WriteLimitW sends a new limit in Watts (already sign-adjusted by the
	// caller is NOT required - implementations apply the correct sign for
	// their direction themselves, see internal/usecase/lpp).
	WriteLimitW(entity spineapi.EntityRemoteInterface, valueW float64, active bool, duration time.Duration, resultCB WriteResultFunc) error
}

// UseCaseIDFor is a small helper so orchestrator code can log/report
// consistently without importing lpc/lpp directly.
func UseCaseIDFor(h Handler) domainmodel.UseCaseID { return h.ID() }
