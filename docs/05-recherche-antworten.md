# Recherche-Antworten — verifiziert gegen eebus-go / ship-go / spine-go

Dieses Dokument beantwortet die in `04-eebus-anwendungsfaelle.md` gestellten Fragen.
Alle Aussagen unten sind gegen den Quellcode von `github.com/enbility/eebus-go`,
`ship-go` und `spine-go` (Tag `v0.7.0` / `v0.6.0` / `v0.7.0`, geprüft am 2026-09-10)
verifiziert — nicht aus Trainingsdaten geraten. Fundstellen sind angegeben.

## 1. LPC — Limitation of Power Consumption

**Rollenverteilung (verifiziert):** Die Steuerbox ist der **"Energy Guard" (EG)**
Akteur (`model.UseCaseActorTypeEnergyGuard`), das gesteuerte Gerät (Wallbox,
Wärmepumpe) ist der **"Controllable System" (CS)** Akteur
(`model.UseCaseActorTypeControllableSystem`).

- Steuerbox-Seite: Paket `usecases/eg/lpc`, lokale Entity-Type `GridGuard` (oder `CEM`).
  Sie ist **Client** der Features `LoadControl`, `DeviceConfiguration`,
  `DeviceDiagnosis`, `ElectricalConnection`.
- Geräte-Seite: Paket `usecases/cs/lpc`. Sie ist **Server** dieser Features.

Da wir die Steuerbox simulieren, implementieren wir die **EG-Seite** und nutzen
`eebus-go` als Bibliothek dafür — nicht die CS-Seite.

**SPINE-Functions (verifiziert, `usecases/eg/lpc/usecase.go` + `public.go`):**
- Feature `LoadControl`, Function `LoadControlLimitListData` /
  `LoadControlLimitDescriptionListData`.
- Limit-Filter: `LimitType = SignDependentAbsValueLimit`,
  `LimitCategory = Obligation`, `LimitDirection = Consume`,
  `ScopeType = ActivePowerLimit`.
- Limit-Wert ist ein **ScaledNumber in Watt** (absolut, kein Prozentwert auf
  Protokollebene). Prozentstufen (0/30/60/100 %) sind reine UI-Konvention und
  müssen gegen `ConsumptionNominalMax` (Feature `ElectricalConnection`,
  Characteristic `PowerConsumptionNominalMax`/`ContractualConsumptionNominalMax`)
  in Watt umgerechnet werden.
- Failsafe-Werte: `DeviceConfiguration` Keys
  `FailsafeConsumptionActivePowerLimit` (W) und `FailsafeDurationMinimum`
  (Dauer, laut Code-Kommentar zwischen 2h und 24h).
- Heartbeat: Feature `DeviceDiagnosis`, Function `DeviceDiagnosisHeartbeatData`;
  `IsHeartbeatWithinDuration(2 * time.Minute)` ist der im Code verwendete
  Referenzwert für "Heartbeat noch gültig".

**Bezug zu §14a EnWG / VDE-AR-N 4400:** liegt außerhalb dessen, was sich aus dem
Quellcode verifizieren lässt (das ist eine regulatorische Zuordnungsfrage, keine
Protokollfrage). Empfehlung: die regulatorischen Grenzwerte (z. B. 4,2 kW
Mindest-Wirkleistung nach §14a EnWG) als **konfigurierbare Szenario-Vorgabe**
im Tool hinterlegen, nicht hart kodieren — das Tool prüft nur, ob das Gerät ein
gesetztes Limit korrekt umsetzt, nicht ob ein bestimmter Zahlenwert regulatorisch
richtig ist.

## 2. LPP — Limitation of Power Production

**Name bestätigt:** "Limitation of Power Production" ist die offizielle
Bezeichnung — `model.UseCaseNameTypeLimitationOfPowerProduction` existiert in
`spine-go/model`, mit eigenständigen Paketen `usecases/eg/lpp` (Steuerbox) und
`usecases/cs/lpp` (Erzeuger/Speicher). Kürzel "LPP" ist korrekt.

**Rollenverteilung:** identisch zu LPC — Steuerbox = Energy Guard (Client),
PV-Wechselrichter/Speicher = Controllable System (Server). Gültige Entity-Typen
für das gesteuerte Gerät (`eg/lpp/usecase.go`): `CEM`, `EVSE`, `Inverter`,
`SmartEnergyAppliance`, `SubMeterElectricity`.

**Kritischer Wire-Detail (verifiziert, `examples/controlbox/main.go`):**
Der Limitwert für Produktion muss als **negativer** Watt-Wert gesendet werden
(`LimitDirection = Produce`, aber `Value` selbst negativ). Zitat aus dem
offiziellen Beispielcode:

> "Per the LPP spec, APPL (Active Power Limit) values for production must be
> <= 0. The eebus-go stack does not transform positive values to negative
> values, so the caller must provide the correct sign."

→ Unsere LPP-Szenario-Engine muss Prozentstufen (0/30/60/100 % Einspeiseleistung)
in **negative** Watt-Werte umrechnen, bevor sie gesendet werden. Das ist eine
leicht zu übersehende Fehlerquelle und wird im Code explizit kommentiert.

**Sonderfall Speicher (verifiziert):** Es gibt **keinen** eigenen Anwendungsfall
für Lade-/Entladerichtung. Ein Speicher/bidirektionale Wallbox wird als normale
Entity behandelt, die **gleichzeitig** LPC (Laderichtung = Verbrauch) und LPP
(Entladerichtung = Erzeugung) unterstützen kann — es sind zwei unabhängige
SPINE-Limits (unterschiedliches `LimitDirection`) auf ggf. derselben Entity.

## 3. Gemeinsame Fragen

**LPC + LPP gleichzeitig auf einer Verbindung zu einem Gerät?** Ja, bestätigt.
`examples/controlbox` registriert `eg/lpc` UND `eg/lpp` gleichzeitig auf
derselben lokalen Entity und damit auf derselben SHIP-Verbindung. Für ein
bidirektionales Gerät (V2G-Wallbox, Hybrid-Wechselrichter mit Speicher) sind
das zwei unabhängig aktive Use-Case-Handler. → Architekturentscheidung:
**1 gepairtes Gerät kann 0..n aktive Use-Case-Handler haben**, nicht strikt 1:1.
Das bestätigt auch die in `02-architektur.md` skizzierte Handler-Registrierung.

**Pflicht- vs. optionale Handshake-Elemente:** Aus `eg/lpc/usecase.go`:
Szenario 1 (Limit) und Szenario 2 (Failsafe-Konfiguration) und Szenario 3
(Heartbeat) sind laut Code als `Mandatory: true` markiert, Szenario 4
(ElectricalConnection/Nominalleistung) ist optional. Für einen realistischen
Steuerbox-Simulator sollten alle vier Szenarien unterstützt werden (auch das
optionale), da ein Errichter-Prüfwerkzeug mehr abdecken soll als ein
Minimal-Client.

**Referenzimplementierung Steuerbox/CEM-Rolle:** Ja, vorhanden:
`eebus-go/examples/controlbox` (Stand: HEAD, nicht im Tag v0.7.0 enthalten,
aber mit v0.7.0-kompatiblen APIs). Registriert Energy-Guard-Entity vom Typ
`GridGuard`, `DeviceCategoryType = GridConnectionHub`, nutzt
`eg/lpc.NewLPC(...)` und `eg/lpp.NewLPP(...)`. Dient als strukturelle Vorlage
für `internal/core` in diesem Projekt.

## 4. Pairing/Trust — Korrektur zu 01-anforderungen.md / 03-ui-design.md

Die Dokumente sprechen von "PIN/Vertrauensanzeige". Verifiziert in
`ship-go/api/connectionstate.go`:

```go
ConnectionStatePin // PIN processing, not supported right now!
```

**PIN-basierte Bestätigung ist im aktuellen SHIP-Stack nicht implementiert.**
Der tatsächliche Trust-Mechanismus ist zertifikatsbasiert (mTLS, SKI als
Fingerprint): die Anwendung entscheidet über `AllowWaitingForTrust(identity)
bool`, ob eine unbekannte Gegenstelle überhaupt auf Bestätigung warten darf,
und der Fortschritt wird über `ServicePairingDetailUpdate` mit Zuständen
`None → Queued → Initiated/ReceivedPairingRequest → InProgress → Trusted →
Completed` (bzw. `RemoteDeniedTrust`/`Error`) gemeldet.

→ GUI-Konsequenz: Der Pairing-Dialog zeigt SKI/Fingerprint des Geräts und
einen Bestätigen/Ablehnen-Dialog ("Diesem Gerät vertrauen?"), **keine
PIN-Eingabe**. `03-ui-design.md` wird entsprechend umgesetzt (Trust-Bestätigung
statt PIN-Dialog).

## 5. Persistenz von Pairing-Daten

Es gibt keine fertige Persistenzschicht in `ship-go` für "gepairte Geräte" auf
Anwendungsebene — das Zertifikats-/Trust-Handling passiert intern im SHIP-Hub
pro Prozess-Laufzeit. Der in `examples/controlbox` gezeigte Weg (und der Weg,
den wir übernehmen): bekannte SKIs selbst persistieren (JSON-Store,
`internal/devicestore`) und beim Start erneut über
`Service.RegisterRemoteService(ServiceIdentity)` registrieren; unser
`AllowWaitingForTrust` gibt für bereits bekannte SKIs automatisch `true`
zurück. So ist beim Re-Test kein erneutes manuelles Pairing nötig.

## 6. Gewählte Bibliotheksversionen

Letzter getaggter Release-Stand von `eebus-go` (`v0.7.0`) enthält bereits
`usecases/eg/lpc` und `usecases/eg/lpp` vollständig — wir binden **getaggte
Versionen**, keine Pseudo-Versionen vom HEAD:

- `github.com/enbility/eebus-go v0.7.0`
- `github.com/enbility/ship-go v0.6.0`
- `github.com/enbility/spine-go v0.7.0`
- `fyne.io/fyne/v2 v2.8.1`

## 7. Offene Punkte — bewusst nicht geraten, sondern konfigurierbar gemacht

Folgende Werte sind **nicht** aus dem Protokoll-Code ableitbar (das sind
regulatorische/vertragliche Werte, keine SPINE-Protokolldetails) und wurden
daher **nicht** hart kodiert, sondern als konfigurierbare Voreinstellungen mit
dokumentiertem Default umgesetzt. Vor Verwendung als offizieller
VNB-Prüfnachweis bitte gegen die aktuelle EEBUS-SPINE-Spec bzw. mit dem
zuständigen VNB abgleichen:

- **Toleranz Soll/Ist-Leistung** je Testschritt (Default im Code: ±5 % vom
  Sollwert oder ±100 W, je nachdem was größer ist — konfigurierbar pro
  Szenario).
- **Reaktionszeit-Timeout** bis zur Quittierung/Umsetzung (Default: 30 s für
  die Schreibbestätigung, weitere 60 s Kulanz bis zur beobachteten
  Ist-Leistungsänderung — konfigurierbar).

Diese Defaults sind reine Software-Voreinstellungen des Errichters, keine
Spec-Werte, und werden im Bericht auch so ausgewiesen (Feld "verwendete
Toleranz/Timeout" pro Testfall), damit der Prüfnachweis nachvollziehbar bleibt.
