#!/usr/bin/env bash
# Кросс-сборка Помогатора под Windows x64 (Linux/macOS).
# Требуется Go 1.22+.
set -euo pipefail
cd "$(dirname "$0")/source"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w -H windowsgui" -o ../Pomogator.exe .
echo "Готово: $(cd .. && pwd)/Pomogator.exe"
