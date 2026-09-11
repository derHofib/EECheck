#!/usr/bin/env bash
# Baut EECheck als doppelklickbare macOS-App (EECheck.app), damit die
# Anwendung nicht mehr über die Kommandozeile gestartet werden muss.
#
# Voraussetzung: Xcode Command Line Tools (`xcode-select --install`) und Go.
#
# Nutzung:
#   ./scripts/build-macos-app.sh
# Ergebnis:
#   EECheck.app im Projektverzeichnis - per Doppelklick starten, oder in
#   den Programme-Ordner ziehen.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v go >/dev/null 2>&1; then
  echo "Go ist nicht installiert. Siehe https://go.dev/dl" >&2
  exit 1
fi

FYNE_BIN="$(go env GOPATH)/bin/fyne"
if [ ! -x "$FYNE_BIN" ]; then
  echo "Installiere das fyne-Packaging-Tool..."
  go install fyne.io/fyne/v2/cmd/fyne@v2.8.1
fi

echo "Baue EECheck.app..."
# fyne package legt EECheck.app im aktuellen Verzeichnis ab (nicht in -sourceDir).
"$FYNE_BIN" package \
  -os darwin \
  -icon "$(pwd)/Icon.png" \
  -name EECheck \
  -appID de.eecheck.steuerboxsimulator \
  -sourceDir ./cmd/eecheck

echo ""
echo "Fertig: EECheck.app liegt im Projektverzeichnis."
echo "Per Doppelklick starten oder in den Programme-Ordner ziehen."
