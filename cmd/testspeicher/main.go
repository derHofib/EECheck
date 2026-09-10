// Command testspeicher simulates an EEBus-fähiger Batteriespeicher
// (Controllable System actor) that supports both LPC (Ladeleistung /
// Verbrauchsrichtung) and LPP (Einspeiseleistung / Erzeugungsrichtung)
// simultaneously on one entity - exactly the "bidirektionaler Speicher"
// case confirmed possible in docs/05-recherche-antworten.md section 3, so
// the Steuerbox-Simulator's Szenario-Konfiguration screen can be exercised
// with a device offering both use cases at once.
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	eebusapi "github.com/enbility/eebus-go/api"
	ucapi "github.com/enbility/eebus-go/usecases/api"
	cslpc "github.com/enbility/eebus-go/usecases/cs/lpc"
	cslpp "github.com/enbility/eebus-go/usecases/cs/lpp"
	shipapi "github.com/enbility/ship-go/api"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"

	"github.com/derHofib/EECheck/internal/testdevice"
)

func main() {
	dataDir := flag.String("datadir", testdevice.DefaultDataDir("testspeicher"), "Verzeichnis für die Geräte-Identität (Zertifikat)")
	port := flag.Int("port", 4713, "SHIP-Serverport")
	nominalMaxW := flag.Float64("nominal", 8000, "Nominal-Lade-/Entladeleistung in Watt (z.B. 8000 für einen 8kW-Speicher)")
	flag.Parse()

	dev, err := testdevice.New(testdevice.Options{
		DataDir:        *dataDir,
		Port:           *port,
		Brand:          "EECheck",
		Model:          "Testspeicher",
		SerialNumber:   "EECHECK-SP-0001",
		DeviceCategory: shipapi.DeviceCategoryTypeInverter,
		DeviceType:     model.DeviceTypeTypeInverter,
		EntityType:     model.EntityTypeTypeInverter,
	})
	if err != nil {
		log.Fatalf("Testspeicher konnte nicht gestartet werden: %v", err)
	}

	var lpc *cslpc.LPC
	onLPCEvent := func(ski string, device spineapi.DeviceRemoteInterface, entity spineapi.EntityRemoteInterface, event eebusapi.EventType) {
		switch event {
		case cslpc.LimitWriteApprovalRequired:
			for msgCounter, limit := range lpc.PendingConsumptionLimits() {
				lpc.ApproveOrDenyConsumptionLimit(msgCounter, true, "")
				fmt.Printf("Ladeleistungslimit (LPC) übernommen: %.0f W (aktiv: %v)\n", limit.Value, limit.IsActive)
			}
		case cslpc.DataUpdateLimit:
			if current, err := lpc.ConsumptionLimit(); err == nil {
				fmt.Printf("Aktuelles Ladeleistungslimit (LPC): %.0f W (aktiv: %v)\n", current.Value, current.IsActive)
			}
		}
	}

	var lpp *cslpp.LPP
	onLPPEvent := func(ski string, device spineapi.DeviceRemoteInterface, entity spineapi.EntityRemoteInterface, event eebusapi.EventType) {
		switch event {
		case cslpp.LimitWriteApprovalRequired:
			for msgCounter, limit := range lpp.PendingProductionLimits() {
				lpp.ApproveOrDenyProductionLimit(msgCounter, true, "")
				fmt.Printf("Einspeiselimit (LPP) übernommen: %.0f W (aktiv: %v)\n", limit.Value, limit.IsActive)
			}
		case cslpp.DataUpdateLimit:
			if current, err := lpp.ProductionLimit(); err == nil {
				fmt.Printf("Aktuelles Einspeiselimit (LPP): %.0f W (aktiv: %v)\n", current.Value, current.IsActive)
			}
		}
	}

	lpc = cslpc.NewLPC(dev.LocalEntity, onLPCEvent)
	if err := dev.Service.AddUseCase(lpc); err != nil {
		log.Fatalf("LPC-Anwendungsfall konnte nicht installiert werden: %v", err)
	}
	lpp = cslpp.NewLPP(dev.LocalEntity, onLPPEvent)
	if err := dev.Service.AddUseCase(lpp); err != nil {
		log.Fatalf("LPP-Anwendungsfall konnte nicht installiert werden: %v", err)
	}

	_ = lpc.SetConsumptionNominalMax(*nominalMaxW)
	_ = lpc.SetConsumptionLimit(ucapi.LoadLimit{Value: 0, IsChangeable: true, IsActive: false})
	_ = lpc.SetFailsafeConsumptionActivePowerLimit(*nominalMaxW*0.1, true)
	_ = lpc.SetFailsafeDurationMinimum(2*time.Hour, true)

	_ = lpp.SetProductionNominalMax(*nominalMaxW)
	// LPP limit values are negative Watts while active (see
	// docs/05-recherche-antworten.md section 2); 0 is sign-agnostic so this
	// initial "no limit active" state doesn't need a sign.
	_ = lpp.SetProductionLimit(ucapi.LoadLimit{Value: 0, IsChangeable: true, IsActive: false})
	_ = lpp.SetFailsafeProductionActivePowerLimit(*nominalMaxW*0.1, true)
	_ = lpp.SetFailsafeDurationMinimum(2*time.Hour, true)

	if err := dev.Start(); err != nil {
		log.Fatalf("Start fehlgeschlagen: %v", err)
	}
	defer dev.Shutdown()

	fmt.Println("=== EECheck Testspeicher (LPC + LPP) ===")
	fmt.Printf("Nominal-Lade-/Entladeleistung: %.0f W, Port: %d\n", *nominalMaxW, *port)
	fmt.Println("Bereit zum Pairing durch die Steuerbox-Simulator-App. Strg+C zum Beenden.")

	testdevice.WaitForSignal()
}
