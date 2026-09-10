# UI-Design — EEBus-Steuerbox-Simulator

Zielgruppe: ich selbst als Techniker vor Ort beim Kunden, oft unter Zeitdruck. Oberfläche muss auf einen Blick verständlich sein, wenig Klicks bis zum Testergebnis, keine überladenen Ansichten.

## Screens

### 1. Start / Anlagen-Dashboard
- Übersicht: aktuell gepairte Geräte (Karten/Liste), je mit Statuspunkt (verbunden/getrennt) und erkanntem Typ (Verbraucher/Erzeuger/unbekannt).
- Primäraktion: "Neues Gerät suchen" (→ Discovery-Screen).
- Sekundäraktion: bestehende Geräte zu einem neuen Anlagen-Testlauf zusammenstellen (Mehrfachauswahl → "Testlauf starten").
- Liste vergangener Testläufe mit Datum, Anlagenkontext, Gesamtergebnis (bestanden/nicht bestanden), Klick öffnet gespeicherten Report.

### 2. Discovery & Pairing
- Live-Liste gefundener EEBus-Geräte (SHIP-ID, IP, erkannter Typ falls möglich).
- Pro Gerät: "Pairing starten" → Trust-/PIN-Bestätigungsdialog → Erfolg/Fehler-Rückmeldung.
- Gepairte Geräte wandern sichtbar in die Dashboard-Liste.

### 3. Szenario-Konfiguration
- Vor Testlauf-Start: pro ausgewähltem Gerät passende Szenario-Optionen (Verbraucher → LPC-Stufen/Freitext-kW; Erzeuger → LPP-Stufen/Freitext-%).
- Voreingestellte Standardszenarien wählbar (Schnellstart), individuelle Werte optional.
- Zusammenfassung "Was wird getestet" vor dem eigentlichen Start, um Fehlkonfiguration auf der Baustelle zu vermeiden.

### 4. Live-Monitor (während Testlauf)
- Pro Gerät ein Statusblock: aktueller Schritt, Soll-Wert, zuletzt gemeldeter Ist-Wert, verstrichene Zeit seit Befehl.
- Darunter/daneben: scrollbarer Live-Log (roh + decodiert), filterbar nach Gerät und Nachrichtentyp.
- Deutliche Farbcodierung: Grün = Schritt bestanden, Rot = fehlgeschlagen/Timeout, Gelb = läuft gerade.
- Abbrechen-Option jederzeit sichtbar.

### 5. Ergebnis / Report
- Gesamtergebnis der Anlage oben (bestanden/nicht bestanden, Zeitstempel).
- Klappbare Detailblöcke pro Gerät mit Soll/Ist-Tabelle je Testfall.
- Buttons: "PDF exportieren" (Einzelgerät + Gesamt), "Rohlog exportieren", "Erneut testen".

## Navigationsfluss

```
Dashboard ──▶ Discovery & Pairing ──▶ (zurück zu) Dashboard
    │
    └──▶ Geräteauswahl für Testlauf ──▶ Szenario-Konfiguration
              ──▶ Live-Monitor ──▶ Ergebnis/Report ──▶ (zurück zu) Dashboard
```

## Layout-Prinzipien

- Große, eindeutig beschriftete Klickflächen (Bedienung ggf. mit Handschuhen/unter Zeitdruck).
- Statusfarben konsistent über alle Screens (Grün/Rot/Gelb wie oben).
- Kein verschachteltes Options-Chaos — Standardpfad (Discovery → Auswahl → Standardszenario → Start) in möglichst wenigen Schritten erreichbar, erweiterte Optionen optional einblendbar.
- Light/Dark-Modus (Baustellenumgebung, wechselnde Lichtverhältnisse) — sofern mit gewähltem GUI-Toolkit ohne großen Mehraufwand umsetzbar.
- Live-Log immer verfügbar, aber nicht im Weg des Standardflusses (z.B. einklappbar).

## Offene Design-Fragen (an mich klären)

- ~~Soll es eine reine Einzelfenster-App sein, oder sind mehrere Fenster/Tabs gewünscht?~~
  **Entschieden:** Ein Fenster mit persistenter Tab-Leiste: **Dashboard**
  (gepairte Geräte, Live-Verbindungsstatus, kompakter Live-Nachrichtenverkehr,
  aktuelle Anlage/Kunde-Kopfzeile), **Test** (Geräte-/Anwendungsfall-Auswahl,
  Szenario-Konfiguration, Live-Monitor und Ergebnis innerhalb desselben Tabs
  nacheinander, plus Testlauf-Historie), **Manuelle Steuerung** (freien
  kW-Wert direkt an ein Gerät senden, ohne Testlauf/Bericht — Fernbedienung
  zum schnellen Prüfen vor Ort) und **Anlage & Kunde** (Stammdaten, die auf
  jedem Prüfprotokoll erscheinen). Discovery/Pairing ist ein Dialog über dem
  Dashboard, kein eigener Tab, da es ein einmaliger Vorgang pro Gerät ist.
- Reicht Text+Tabelle für den PDF-Report, oder sollen Soll/Ist-Verläufe auch als Diagramm dargestellt werden?
