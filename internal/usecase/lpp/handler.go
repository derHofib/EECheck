// Package lpp implements the "Limitation of Power Production" use-case
// handler, wrapping eebus-go's usecases/eg/lpp package.
//
// Critical wire detail (verified in eebus-go's own examples/controlbox,
// see docs/05-recherche-antworten.md section 2): production limit values
// MUST be sent as negative Watts. eebus-go does not transform the sign for
// the caller. This handler is the single place that conversion happens, so
// callers (the orchestrator, the GUI) always work in "positive percent of
// nominal production" terms and never have to remember the sign rule.
package lpp

import (
	"time"

	eebusapi "github.com/enbility/eebus-go/api"
	ucapi "github.com/enbility/eebus-go/usecases/api"
	eglpp "github.com/enbility/eebus-go/usecases/eg/lpp"
	spineapi "github.com/enbility/spine-go/api"
	spinemodel "github.com/enbility/spine-go/model"

	"github.com/derHofib/EECheck/internal/eebus"
	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/usecase"
)

// Handler adapts eebus-go's eg/lpp.LPP to the generic usecase.Handler
// interface driven by the orchestrator.
type Handler struct {
	uc *eglpp.LPP
}

// New creates an uninstalled LPP handler. Call eebus.Core.AddUseCase(h) to
// install it on the local Steuerbox entity.
func New() *Handler { return &Handler{} }

var _ usecase.Handler = (*Handler)(nil)
var _ eebus.UseCaseAdapter = (*Handler)(nil)

func (h *Handler) ID() domainmodel.UseCaseID { return domainmodel.UseCaseLPP }

func (h *Handler) Direction() usecase.Direction { return usecase.DirectionProduce }

func (h *Handler) Install(localEntity spineapi.EntityLocalInterface, eventCB eebusapi.EntityEventCallback) error {
	h.uc = eglpp.NewLPP(localEntity, eventCB)
	return nil
}

func (h *Handler) SupportedEntities() []spineapi.EntityRemoteInterface {
	scenarios := h.uc.RemoteEntitiesScenarios()
	entities := make([]spineapi.EntityRemoteInterface, 0, len(scenarios))
	for _, s := range scenarios {
		if len(s.Scenarios) > 0 {
			entities = append(entities, s.Entity)
		}
	}
	return entities
}

func (h *Handler) NominalMaxW(entity spineapi.EntityRemoteInterface) (float64, error) {
	return h.uc.ProductionNominalMax(entity)
}

// CurrentLimitW returns the currently reported production limit. Per the
// LPP wire format the value is reported as a negative Watt value while
// active; callers of usecase.Handler should treat a negative value here as
// "limiting production to |value| W", consistent with WriteLimitW's input.
func (h *Handler) CurrentLimitW(entity spineapi.EntityRemoteInterface) (float64, bool, error) {
	limit, err := h.uc.ProductionLimit(entity)
	if err != nil {
		return 0, false, err
	}
	return limit.Value, limit.IsActive, nil
}

// WriteLimitW takes valueW as a positive "allowed production power" in
// Watts (e.g. 30% of nominal) and sends it on the wire with the sign LPP
// requires (negative). Callers never need to apply the sign themselves.
func (h *Handler) WriteLimitW(entity spineapi.EntityRemoteInterface, valueW float64, active bool, duration time.Duration, resultCB usecase.WriteResultFunc) error {
	wireValue := valueW
	if wireValue > 0 {
		wireValue = -wireValue
	}
	limit := ucapi.LoadLimit{
		Value:        wireValue,
		IsActive:     active,
		IsChangeable: true,
		Duration:     duration,
	}
	_, err := h.uc.WriteProductionLimit(entity, limit, func(result spinemodel.ResultDataType, _ spinemodel.MsgCounterType) {
		if resultCB == nil {
			return
		}
		if result.ErrorNumber == nil || *result.ErrorNumber == 0 {
			resultCB(true, "")
		} else {
			desc := ""
			if result.Description != nil {
				desc = string(*result.Description)
			}
			resultCB(false, desc)
		}
	})
	return err
}
