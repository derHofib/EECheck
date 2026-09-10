// Command testwallbox simulates an EEBus-fähige Wallbox (Controllable
// System actor, LPC use case) so the EECheck Steuerbox-Simulator can be
// exercised against a real SHIP/SPINE peer without needing actual
// hardware. It auto-accepts pairing and auto-approves every consumption
// limit the Steuerbox sends, then reports what it applied.
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	eebusapi "github.com/enbility/eebus-go/api"
	ucapi "github.com/enbility/eebus-go/usecases/api"
	cslpc "github.com/enbility/eebus-go/usecases/cs/lpc"
	shipapi "github.com/enbility/ship-go/api"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"

	"github.com/derHofib/EECheck/internal/testdevice"
)

func main() {
	dataDir := flag.String("datadir", testdevice.DefaultDataDir("testwallbox"), "Verzeichnis für die Geräte-Identität (Zertifikat)")
	port := flag.Int("port", 4712, "SHIP-Serverport")
	nominalMaxW := flag.Float64("nominal", 11000, "Nominalleistung in Watt (z.B. 11000 für eine 11kW-Wallbox)")
	flag.Parse()

	dev, err := testdevice.New(testdevice.Options{
		DataDir:        *dataDir,
		Port:           *port,
		Brand:          "EECheck",
		Model:          "Testwallbox",
		SerialNumber:   "EECHECK-WB-0001",
		DeviceCategory: shipapi.DeviceCategoryTypeEMobility,
		DeviceType:     model.DeviceTypeTypeChargingStation,
		EntityType:     model.EntityTypeTypeEVSE,
	})
	if err != nil {
		log.Fatalf("Testwallbox konnte nicht gestartet werden: %v", err)
	}

	var uc *cslpc.LPC
	onEvent := func(ski string, device spineapi.DeviceRemoteInterface, entity spineapi.EntityRemoteInterface, event eebusapi.EventType) {
		switch event {
		case cslpc.LimitWriteApprovalRequired:
			for msgCounter, limit := range uc.PendingConsumptionLimits() {
				uc.ApproveOrDenyConsumptionLimit(msgCounter, true, "")
				fmt.Printf("Limit übernommen: %.0f W (aktiv: %v)\n", limit.Value, limit.IsActive)
			}
		case cslpc.DataUpdateLimit:
			if current, err := uc.ConsumptionLimit(); err == nil {
				fmt.Printf("Aktuelles Verbrauchslimit: %.0f W (aktiv: %v)\n", current.Value, current.IsActive)
			}
		}
	}

	uc = cslpc.NewLPC(dev.LocalEntity, onEvent)
	if err := dev.Service.AddUseCase(uc); err != nil {
		log.Fatalf("LPC-Anwendungsfall konnte nicht installiert werden: %v", err)
	}

	_ = uc.SetConsumptionNominalMax(*nominalMaxW)
	_ = uc.SetConsumptionLimit(ucapi.LoadLimit{Value: 0, IsChangeable: true, IsActive: false})
	_ = uc.SetFailsafeConsumptionActivePowerLimit(*nominalMaxW*0.1, true)
	_ = uc.SetFailsafeDurationMinimum(2*time.Hour, true)

	if err := dev.Start(); err != nil {
		log.Fatalf("Start fehlgeschlagen: %v", err)
	}
	defer dev.Shutdown()

	fmt.Println("=== EECheck Testwallbox ===")
	fmt.Printf("Nominalleistung: %.0f W, Port: %d\n", *nominalMaxW, *port)
	fmt.Println("Bereit zum Pairing durch die Steuerbox-Simulator-App. Strg+C zum Beenden.")

	testdevice.WaitForSignal()
}
