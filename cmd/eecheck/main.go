// Command eecheck is the EEBus-Steuerbox-Simulator described in
// docs/00-prompt-fuer-coding-ki.md: a standalone desktop tool that plays
// the Steuerbox/CEM role to test EEBus-controllable devices (Wallboxen,
// Wärmepumpen, PV-Wechselrichter, Speicher) for correct LPC/LPP behaviour.
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/derHofib/EECheck/internal/eebus"
	"github.com/derHofib/EECheck/internal/gui"
	"github.com/derHofib/EECheck/internal/messagelog"
	"github.com/derHofib/EECheck/internal/orchestrator"
	"github.com/derHofib/EECheck/internal/store"
	"github.com/derHofib/EECheck/internal/usecase/lpc"
	"github.com/derHofib/EECheck/internal/usecase/lpp"
)

func main() {
	dataDir, err := defaultDataDir()
	if err != nil {
		log.Fatalf("could not determine data directory: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("could not create data directory %s: %v", dataDir, err)
	}

	core, err := eebus.NewCore(eebus.Config{DataDir: dataDir})
	if err != nil {
		log.Fatalf("could not initialise EEBus core: %v", err)
	}

	lpcHandler := lpc.New()
	lppHandler := lpp.New()
	if err := core.AddUseCase(lpcHandler); err != nil {
		log.Fatalf("could not install LPC use case: %v", err)
	}
	if err := core.AddUseCase(lppHandler); err != nil {
		log.Fatalf("could not install LPP use case: %v", err)
	}

	msgLog := messagelog.New()
	if err := core.SubscribeMessages(msgLog); err != nil {
		log.Fatalf("could not attach message log: %v", err)
	}

	deviceStore, err := store.Open(dataDir)
	if err != nil {
		log.Fatalf("could not open device store: %v", err)
	}
	runStore, err := store.OpenRunStore(dataDir)
	if err != nil {
		log.Fatalf("could not open run store: %v", err)
	}

	// Re-trust every previously paired device so a re-test never requires
	// pairing again (docs/05-recherche-antworten.md section 5).
	for _, kd := range deviceStore.All() {
		core.Pair(kd.SKI, kd.ShipID)
	}

	if err := core.Start(); err != nil {
		log.Fatalf("could not start EEBus service: %v", err)
	}
	defer core.Shutdown()

	orch := orchestrator.New(lpcHandler, lppHandler)

	app := gui.NewApp(core, orch, deviceStore, runStore, msgLog, dataDir)
	app.Run()
}

func defaultDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "EECheck"), nil
}
