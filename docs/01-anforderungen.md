# Anforderungen — EEBus-Steuerbox-Simulator

## 1. Funktionale Anforderungen

### 1.1 Discovery & Pairing (geräteunabhängig)
- mDNS/Zeroconf-Discovery aller EEBus-fähigen Geräte im lokalen Netz.
- Anzeige mit Gerätetyp (soweit aus SPINE-Device-Info ableitbar: Verbraucher/Erzeuger/Speicher/unbekannt).
- SHIP-Pairing-Flow inkl. Trust-Bestätigung (PIN/Vertrauensanzeige) über die GUI.
- Mehrere Geräte parallel gepairt halten (für Anlagen-Testläufe).
- Pairing-/Zertifikatsdaten persistent lokal speichern, erneutes Pairing bei Re-Test nicht nötig.

### 1.2 Verbraucher-Testpfad (LPC)
- Leistungslimit setzen (frei wählbarer kW-Wert), Limit aufheben, Sperrsignal senden.
- Erfassen der Geräteantwort: Quittierung, tatsächlich umgesetzte Soll-Leistung, Zeit bis zur Umsetzung.
- Vordefinierte Testszenarien (z.B. Stufen 0 % / 30 % / 60 % / 100 %, oder frei konfigurierbar) — je nach Vorgabe des LPC-Anwendungsfalls.

### 1.3 Erzeuger-Testpfad (LPP bzw. korrekter Anwendungsfall gemäß Spec)
- Einspeisebegrenzung/Wirkleistungsreduzierung setzen (analog zu LPC, aber Erzeugungsrichtung).
- Typische regulatorisch relevante Stufen abbilden (0 % / 30 % / 60 % / 100 % Einspeiseleistung, exakte Stufen gegen Spec/VDE-AR-N 4105 prüfen).
- Erfassen der Geräteantwort analog 1.2.
- Falls das Zielgerät ein Speicher ist: ggf. Unterscheidung Lade-/Entladerichtung berücksichtigen (klären, ob eigener Anwendungsfall oder Teil von LPP).

### 1.4 Kombinierter Anlagen-Testlauf
- Mehrere gepairte Geräte (z.B. PV-WR + Speicher + Wallbox) einem gemeinsamen Testlauf zuordnen.
- Testlauf fährt definierte Szenarien sequenziell oder parallel gegen alle zugeordneten Geräte.
- Ergebnis pro Gerät UND als Gesamt-Anlagenzusammenfassung (ein Netzanschlusspunkt, ein Prüfnachweis).

### 1.5 Live-Nachrichtenverkehr
- Vollständiger SHIP/SPINE-Nachrichtenaustausch in Echtzeit, roh + decodiert, mit Zeitstempel, pro Gerät unterscheidbar (bei Mehrgeräte-Lauf).
- Filterbar nach Gerät / Nachrichtentyp.

### 1.6 Prüfprotokoll-Export
- Rohlog: strukturiert, maschinenlesbar (JSON/NDJSON), vollständig, unverändert archivierbar.
- PDF-Bericht pro Gerät: Gerätedaten, Testzeitpunkt, Szenarien mit Soll/Ist-Vergleich, Bestanden/Nicht bestanden je Testfall.
- PDF-Gesamtbericht pro Anlage/Netzanschlusspunkt: Zusammenfassung aller geprüften Geräte, Gesamtergebnis, zur Weitergabe an Kunde/VNB geeignet.

## 2. Nicht-funktionale Anforderungen

- Native Builds macOS (Apple Silicon/arm64) + Windows (x64), keine WebView2-Abhängigkeit.
- Bedienbar unter Zeitdruck auf der Baustelle (klare Statusanzeigen, wenige Klicks bis zum Testlauf).
- Erweiterbar um weitere EEBus-Anwendungsfälle, ohne Kernarchitektur umzubauen.
- Protokoll-Layer unabhängig von GUI testbar (Unit-/Integrationstests ohne reale Hardware, soweit über Mocks möglich).

## 3. Akzeptanzkriterien

- Für jeden Testfall eindeutiges Bestanden/Nicht-bestanden-Kriterium (z.B. Soll-Ist-Abweichung innerhalb Toleranz X, Reaktionszeit innerhalb Y Sekunden — konkrete Werte vor Umsetzung mit mir klären bzw. aus Spec ableiten).
- Erzeugtes PDF-Protokoll muss ohne Zusatzsoftware lesbar und für Dritte (VNB/Kunde) nachvollziehbar sein.
- Rohlog muss im Streitfall als vollständiger, unveränderter Nachweis dienen können (keine verlustbehaftete Aggregation).
