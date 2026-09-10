// Package lpc implements the "Limitation of Power Consumption" use-case
// handler for the Steuerbox-Simulator's Energy Guard role, wrapping
// eebus-go's usecases/eg/lpc package. See docs/05-recherche-antworten.md
// section 1 for the verified role split (we are Energy Guard/client, the
// device under test is Controllable System/server).
package lpc

import (
	"time"

	eebusapi "github.com/enbility/eebus-go/api"
	ucapi "github.com/enbility/eebus-go/usecases/api"
	eglpc "github.com/enbility/eebus-go/usecases/eg/lpc"
	spineapi "github.com/enbility/spine-go/api"
	spinemodel "github.com/enbility/spine-go/model"

	"github.com/derHofib/EECheck/internal/eebus"
	domainmodel "github.com/derHofib/EECheck/internal/model"
	"github.com/derHofib/EECheck/internal/usecase"
)

// Handler adapts eebus-go's eg/lpc.LPC to the generic usecase.Handler
// interface driven by the orchestrator.
type Handler struct {
	uc *eglpc.LPC
}

// New creates an uninstalled LPC handler. Call eebus.Core.AddUseCase(h) to
// install it on the local Steuerbox entity.
func New() *Handler { return &Handler{} }

var _ usecase.Handler = (*Handler)(nil)
var _ eebus.UseCaseAdapter = (*Handler)(nil)

func (h *Handler) ID() domainmodel.UseCaseID { return domainmodel.UseCaseLPC }

func (h *Handler) Direction() usecase.Direction { return usecase.DirectionConsume }

func (h *Handler) Install(localEntity spineapi.EntityLocalInterface, eventCB eebusapi.EntityEventCallback) error {
	h.uc = eglpc.NewLPC(localEntity, eventCB)
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
	return h.uc.ConsumptionNominalMax(entity)
}

func (h *Handler) CurrentLimitW(entity spineapi.EntityRemoteInterface) (float64, bool, error) {
	limit, err := h.uc.ConsumptionLimit(entity)
	if err != nil {
		return 0, false, err
	}
	return limit.Value, limit.IsActive, nil
}

func (h *Handler) WriteLimitW(entity spineapi.EntityRemoteInterface, valueW float64, active bool, duration time.Duration, resultCB usecase.WriteResultFunc) error {
	limit := ucapi.LoadLimit{
		Value:        valueW, // LPC: positive Watts = consumption limit
		IsActive:     active,
		IsChangeable: true,
		Duration:     duration,
	}
	_, err := h.uc.WriteConsumptionLimit(entity, limit, func(result spinemodel.ResultDataType, _ spinemodel.MsgCounterType) {
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
