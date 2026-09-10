# Architektur — EEBus-Steuerbox-Simulator

Ausgangsvorschlag zur Prüfung/Verfeinerung durch die Coding-KI, nicht bindend im Detail.

## Leitprinzip

Protokoll-Layer strikt von GUI-Layer trennen. Anwendungsfälle (LPC, LPP, künftige) als austauschbare/ergänzbare Module, nicht hart verdrahtet — Gerätetyp-Erkennung entscheidet, welche Use-Case-Handler aktiviert werden.

## Modulübersicht

```
┌─────────────────────────────────────────────┐
│                   GUI-Layer                  │
│  (Fyne — Discovery, Szenario-Config, Live-   │
│   Monitor, Report-Ansicht)                   │
└───────────────────┬───────────────────────────┘
                    │ Events / State-Updates
┌───────────────────▼───────────────────────────┐
│              Session-/Orchestrator            │
│  - verwaltet gepairte Geräte                  │
│  - ordnet Geräte einem Anlagen-Testlauf zu    │
│  - startet/koordiniert Szenarien pro Gerät    │
└───────────────────┬───────────────────────────┘
                    │
     ┌───────────────┼───────────────┐
     ▼               ▼               ▼
┌──────────┐   ┌──────────┐   ┌──────────────┐
│ LPC-      │   │ LPP-      │   │ weitere Use- │
│ Handler   │   │ Handler   │   │ Case-Handler │
│(Verbraucher)│ │(Erzeuger) │   │  (Plugin)    │
└─────┬────┘   └─────┬────┘   └──────┬───────┘
     │               │               │
     └───────────────┼───────────────┘
                    ▼
        ┌─────────────────────────┐
        │   Protokoll-Core        │
        │  SHIP (Pairing/Trust,   │
        │  Transport) + SPINE     │
        │  (Datenmodell)          │
        │  — Basis: eebus-go      │
        └───────────┬─────────────┘
                    ▼
        ┌─────────────────────────┐
        │  Discovery (mDNS)        │
        └─────────────────────────┘

        ┌─────────────────────────┐
        │  Report-Engine            │
        │  - sammelt Nachrichten-   │
        │    verlauf pro Session   │
        │  - Rohlog-Export (JSON)  │
        │  - PDF-Generierung       │
        │    (pro Gerät + gesamt)  │
        └─────────────────────────┘
```

## Modulverantwortlichkeiten

- **Discovery**: findet EEBus-Geräte im Netz, liefert Basisinformationen (SHIP-ID, ggf. Geräteklasse) an den Orchestrator.
- **Protokoll-Core**: kapselt SHIP-Verbindungsaufbau/Pairing/Trust und SPINE-Nachrichtenverarbeitung. Bietet eine generische Schnittstelle "sende Function X an Gerät Y, empfange Antwort", unabhängig vom konkreten Anwendungsfall.
- **Use-Case-Handler (LPC/LPP/…)**: kennen die konkreten SPINE-Functions/Datenstrukturen ihres Anwendungsfalls, übersetzen Testszenarien ("setze Limit auf 3 kW") in SPINE-Nachrichten und werten Antworten aus. Neue Handler = neues Modul, kein Eingriff in Core nötig.
- **Session-/Orchestrator**: hält den Zustand eines Anlagen-Testlaufs (welche Geräte, welche Szenarien, welcher Fortschritt), koordiniert parallele/sequenzielle Ausführung, aggregiert Ergebnisse für den Gesamtbericht.
- **Report-Engine**: konsumiert den vollständigen Nachrichtenverlauf + Testergebnisse, erzeugt Rohlog und PDF (pro Gerät und Anlagen-Gesamtbericht).
- **GUI-Layer**: reine Darstellung/Interaktion, keine Protokolllogik. Abonniert Zustandsänderungen vom Orchestrator.

## Erweiterbarkeit

Neuer Anwendungsfall (z.B. später ein weiterer SPINE-Use-Case) = neuer Handler nach demselben Interface wie LPC/LPP-Handler, Registrierung im Orchestrator. Kein Umbau von Discovery, Protokoll-Core, Report-Engine oder GUI erforderlich.

## Zu klärende Architekturfragen (an mich oder per Recherche)

- Können LPC und LPP gleichzeitig auf derselben SHIP-Verbindung zu einem Gerät laufen (falls ein Gerät beide Rollen hätte, z.B. bidirektionale Wallbox), oder ist strikt 1 Gerät = 1 Use-Case anzunehmen?
- Toleranzwerte/Zeitfenster für Bestanden/Nicht-bestanden je Use-Case (siehe `01-anforderungen.md`, Akzeptanzkriterien).
