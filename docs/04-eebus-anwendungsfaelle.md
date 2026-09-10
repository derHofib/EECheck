# EEBus-Anwendungsfälle — Rechercheauftrag für die Coding-KI

Dieses Dokument fasst zusammen, was fachlich vor der Implementierung verifiziert werden muss. **Nichts hiervon aus Trainingsdaten als sicher annehmen** — gegen die offizielle EEBUS-SPINE-Technical-Spezifikation und die Referenzimplementierung `eebus-go` (github.com/enbility) prüfen.

## 1. LPC — Limitation of Power Consumption (Verbraucher)

Zu verifizieren:
- Exakte Rollenverteilung im SPINE-Datenmodell: welche Seite ist Client, welche Server (Steuerbox vs. Wallbox/Wärmepumpe).
- Relevante SPINE-Functions/Features (z.B. `LoadControlLimitListData`, `LoadControlLimitDescriptionListData` o.ä. — Namen gegen Spec/Code prüfen, nicht raten).
- Zulässige/übliche Limit-Stufen und Einheiten (kW vs. %).
- Erwartetes Quittierungs-/Antwortverhalten des gesteuerten Geräts (Pflichtfelder, Timing-Vorgaben falls spezifiziert).
- Bezug zu §14a EnWG / VDE-AR-N 4400 zur Einordnung, welche Grenzwerte/Verhaltensweisen "korrekt" im regulatorischen Sinn sind.

## 2. LPP — Limitation of Power Production (Erzeuger)

Zu verifizieren:
- Ob "LPP" die korrekte offizielle Bezeichnung/Kürzel im aktuellen SPINE-Standard ist (Namensgebung kann abweichen — prüfen).
- Rollenverteilung analog LPC, aber für Erzeugungsrichtung (Steuerbox vs. PV-Wechselrichter/Speicher).
- Relevante SPINE-Functions/Features für Einspeisebegrenzung.
- Übliche Stufung (z.B. 0/30/60/100 % Einspeiseleistung) und deren regulatorische Herkunft (VDE-AR-N 4105 / EEG §9) — als Kontext, nicht als Software-Vorgabe missverstehen, aber Testszenarien sollten diese Praxis-Stufen abbilden können.
- Sonderfall Batteriespeicher: ob Lade- und Entladerichtung getrennt zu behandeln sind oder ob ein Speicher aus SPINE-Sicht wie ein normaler Erzeuger/Verbraucher je nach Fließrichtung behandelt wird.

## 3. Gemeinsame Fragen (beide Anwendungsfälle)

- Können LPC und LPP auf derselben SHIP-Verbindung zu einem einzelnen Gerät parallel aktiv sein (z.B. bidirektionale Wallbox mit V2G), oder ist die Grundannahme "1 Gerät = 1 Use-Case" korrekt?
- Welche Pflicht- vs. optionale Elemente gibt es im Handshake/Pairing, die für einen realistischen Steuerbox-Simulator (nicht nur Minimal-Client) abgebildet sein sollten?
- Gibt es in `eebus-go` bereits Beispielcode für die Server-seitige Steuerbox-/CEM-Rolle (nicht nur die Geräteseite), der als Ausgangspunkt dienen kann?

## 4. Vorgehen bei Unklarheit

Wenn die Spezifikation an einer Stelle mehrdeutig ist oder `eebus-go` von der offiziellen Spec abweicht: beides dokumentieren (z.B. als Kommentar im Code oder kurze Notiz) und mich gezielt fragen, bevor eine Annahme fest im Code landet — insbesondere bei allem, was später Teil eines Prüfnachweises gegenüber dem VNB wird.
