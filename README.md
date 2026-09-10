# EECheck — EEBus-Steuerbox-Simulator

Eigenständiges Desktop-Tool zur vollständigen Prüfung von EEBus-steuerbaren
Anlagen (Wallboxen, Wärmepumpen, PV-Wechselrichter, Speicher) nach §14a EnWG
bzw. den Regelungen für Erzeugungsanlagen. Das Tool simuliert die
"Steuerbox"-Gegenstelle (Energy-Guard-Rolle) vollständig und erzeugt daraus
ein belastbares Prüfprotokoll für Errichter.

Siehe `docs/00-prompt-fuer-coding-ki.md` für den vollständigen Projektauftrag
und `docs/05-recherche-antworten.md` für alle gegen den eebus-go-Quellcode
verifizierten fachlichen Entscheidungen (SPINE-Functions, Rollenverteilung,
LPP-Vorzeichenkonvention, Pairing/Trust-Verhalten).

## Architektur

```
cmd/eecheck            Programmeinstieg, verdrahtet alle Module
internal/eebus          Protokoll-Core: Discovery, Pairing, Steuerbox-Identität
internal/model           Protokoll-agnostisches Domänenmodell
internal/usecase/lpc     Limitation of Power Consumption (Verbraucher)
internal/usecase/lpp     Limitation of Power Production (Erzeuger)
internal/orchestrator    Anlagen-Testlauf über mehrere Geräte
internal/messagelog      Live-Nachrichtenverkehr + Rohlog (NDJSON)
internal/report          PDF-Prüfprotokoll (Gerät + Gesamt-Anlage)
internal/store           Persistenz: bekannte Geräte, Testlauf-Historie
internal/gui             Fyne-GUI (Dashboard, Discovery, Szenario, Live-Monitor, Report)
internal/testdevice      Gemeinsamer SHIP/SPINE-Unterbau für die Test-Geräte-Fixtures
cmd/testwallbox          Simulierte Wallbox (Controllable System, LPC) zum Testen ohne Hardware
cmd/testspeicher         Simulierter Batteriespeicher (Controllable System, LPC+LPP)
```

## Bauen

Benötigt Go 1.24+. Fyne nutzt CGO/OpenGL; auf Linux werden zum Bauen
X11/OpenGL-Entwicklungspakete benötigt (z. B. `libgl1-mesa-dev xorg-dev
pkg-config` auf Debian/Ubuntu). Für native macOS-(arm64)- und
Windows-(x64)-Builds ohne WebView-Abhängigkeit wird
[`fyne-cross`](https://github.com/fyne-io/fyne-cross) empfohlen:

```
go install fyne.io/fyne/v2/cmd/fyne-cross@latest
fyne-cross darwin -arch=arm64 -app-id de.eecheck.steuerboxsimulator ./cmd/eecheck
fyne-cross windows -arch=amd64 -app-id de.eecheck.steuerboxsimulator ./cmd/eecheck
```

Lokal (Linux, zur Entwicklung):

```
go build ./...
go run ./cmd/eecheck
```

## Ohne echte Hardware testen: Test-Wallbox und Test-Speicher

`cmd/testwallbox` und `cmd/testspeicher` simulieren die Geräteseite
(Controllable System) und lassen sich von `eecheck` ganz normal per
Discovery finden und pairen - nützlich, um die Kommunikation zu prüfen,
bevor eine echte Anlage angeschlossen wird.

```
go run ./cmd/testwallbox                 # 11 kW Wallbox, LPC, Port 4712
go run ./cmd/testspeicher                # 8 kW Speicher, LPC+LPP, Port 4713
go run ./cmd/eecheck                     # Steuerbox-Simulator: "Neues Gerät suchen"
```

Beide Fixtures akzeptieren Pairing-Anfragen automatisch (keine manuelle
Bestätigung auf der Geräteseite nötig - siehe Kommentar in
`internal/testdevice/device.go`) und geben auf der Konsole aus, welches
Limit sie empfangen und übernommen haben. Ihre Geräte-Identität
(Zertifikat) wird unter `~/.config/EECheck/testwallbox` bzw.
`.../testspeicher` persistiert, bleibt also über Neustarts stabil.

**Voraussetzung:** Discovery basiert auf mDNS/Zeroconf (UDP-Multicast).
Das funktioniert zuverlässig im normalen LAN eines Mac/PC, aber nicht in
jeder Sandbox/Cloud-Umgebung mit eingeschränktem Netzwerk (dort wird
kein Gerät gefunden, obwohl beide Programme fehlerfrei laufen). Am
besten auf dem echten Zielrechner testen, auf dem später auch `eecheck`
läuft.

## Stand / offene Punkte

Diese Implementierung deckt den vollständigen MVP-Pfad aus
`docs/00-prompt-fuer-coding-ki.md` ab: generisches Discovery/Pairing, LPC-
und LPP-Testpfad, Mehrgeräte-Anlagentestlauf, Report-Export (PDF +
NDJSON-Rohlog), GUI nach `docs/03-ui-design.md`. Nicht Teil dieses Standes,
bewusst als Erweiterung markiert (siehe Kommentare in
`internal/orchestrator/orchestrator.go`):

- Reale Leistungsmessung (MPC/MGCP-Anwendungsfälle) statt Limit-Rückmeldung
  als "Ist-Wert"-Proxy.
- Verifikation gegen echte Hardware (Wallbox etc.) — bislang nur gegen den
  eebus-go-Quellcode und dessen eigenes `examples/controlbox`-Referenzbeispiel
  verifiziert, nicht gegen reale Geräte getestet.
- Toleranz-/Timeout-Defaults sind Software-Voreinstellungen, keine
  verifizierten Spec-Werte — vor Nutzung als offizieller VNB-Nachweis prüfen
  (siehe `docs/05-recherche-antworten.md`, Abschnitt 7).
