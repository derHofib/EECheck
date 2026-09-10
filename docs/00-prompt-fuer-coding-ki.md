# Projektauftrag: EEBus-Steuerbox-Simulator — vollständige Anlagenprüfung nach §14a EnWG

> Dies ist der Einstiegsprompt. Die Dateien `01-anforderungen.md`, `02-architektur.md`, `03-ui-design.md` und `04-eebus-anwendungsfaelle.md` im selben Ordner sind Teil desselben Auftrags — bitte alle vor Beginn der Architekturentscheidung lesen. Sie ersetzen eine frühere, engere Version dieses Prompts (die nur Verbraucher-Tests abdeckte).

## Kontext

Ich bin Elektrotechnik-Handwerksmeister und errichte Ladeinfrastruktur, PV-Anlagen und Speicher. Nach §14a EnWG (Verbrauch) bzw. den zugehörigen Regelungen für Erzeugungsanlagen (Einspeisebegrenzung, EEG §9 / VDE-AR-N 4105) müssen steuerbare Geräte auf Anforderung des VNB per EEBus (SHIP/SPINE) begrenzbar sein — sowohl im Verbrauch (Wallbox, Wärmepumpe) als auch in der Erzeugung (PV-Wechselrichter, Batteriespeicher). Die Gegenstelle dazu ist die "Steuerbox" beim Netzanschlusspunkt. Ich möchte diese Steuerbox-Rolle vollständig simulieren können, um **jede von mir errichtete Anlage** — Verbraucher wie Erzeuger, einzeln oder kombiniert an einem Netzanschlusspunkt — auf korrekte EEBus-Regelungsreaktion zu prüfen und daraus einen belastbaren Nachweis als Errichter zu erzeugen.

## Ziel des Tools

Eine eigenständige Desktop-Anwendung, die die komplette Steuerbox-Gegenstelle für eine Anlage simuliert:

1. **Verbraucher-Prüfung** (Anwendungsfall LPC – Limitation of Power Consumption): Wallboxen, Wärmepumpen etc. auf korrekte Leistungsbegrenzung testen.
2. **Erzeuger-Prüfung** (Anwendungsfall LPP – Limitation of Power Production, exakte Bezeichnung gegen Spec verifizieren): PV-Wechselrichter, Batteriespeicher etc. auf korrekte Einspeisebegrenzung/Wirkleistungsreduzierung testen.
3. **Kombinierte Anlagenprüfung**: mehrere Geräte an einem Netzanschlusspunkt gleichzeitig pairen und in einem gemeinsamen Testlauf prüfen (realistisches Szenario: PV + Speicher + Wallbox hinter einem Zähler).
4. Aus jedem Testlauf einen Nachweis erzeugen: Rohlog (vollständig, maschinenlesbar) + druckbares PDF-Protokoll (pro Gerät und als Gesamt-Anlagenprotokoll).

Standalone-Projekt, unabhängig von fieldvibe.de.

## Kernanforderung an dich als Coding-KI

Das Tool ist **kein reiner Wallbox-Tester**, sondern ein generisches Prüfwerkzeug für alle EEBus-steuerbaren Einheiten einer Anlage. Architektur entsprechend erweiterbar auslegen (siehe `02-architektur.md`): neue Anwendungsfälle/Gerätetypen müssen sich später ergänzen lassen, ohne den Kern umzubauen.

## Plattform (unverändert)

- Native Builds für macOS (Apple Silicon/arm64) und Windows (x64).
- Keine WebView-Abhängigkeit (kein Electron/Tauri+WebView2/CEF) — echtes natives GUI-Rendering, unter Windows ohne WebView2-Runtime lauffähig.
- Empfehlung zur Prüfung: Go (`eebus-go`/`ship-go`/`spine-go` von Enbility als Protokollbasis) + Fyne als natives GUI-Toolkit. Alternativen nur vorschlagen, wenn sie das WebView-Verbot strikt einhalten und protokollseitig überlegen sind — eigene Bewertung vornehmen.

## Vorgehen

1. Alle vier Begleitdateien lesen, insbesondere `04-eebus-anwendungsfaelle.md` — dort steht, was du fachlich noch verifizieren musst, bevor du SPINE-Function-Namen im Code verwendest.
2. Architektur- und Stack-Entscheidung mit Begründung vorschlagen (siehe `02-architektur.md` als Ausgangspunkt).
3. MVP inkrementell: (a) Discovery/Pairing generisch für beliebige EEBus-Geräte, (b) LPC-Testpfad (Verbraucher) mit echter Wallbox verifizieren, (c) LPP-Testpfad (Erzeuger) ergänzen, (d) Mehrgeräte-/Anlagen-Testlauf, (e) Report-Export, (f) GUI-Politur nach `03-ui-design.md`.
4. Offene fachliche Punkte aktiv recherchieren oder mich gezielt fragen — nicht raten.
