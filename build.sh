#!/usr/bin/env bash
# Сборка WidthCorrector
set -e
cd "$(dirname "$0")"

# пересжать интерфейс (встраивается в бинарник в сжатом виде)
mkdir -p assets
gzip -9 -c WidthCorrector.html > assets/index.html.gz

# подтянуть зависимости
go mod tidy

case "${1:-local}" in
  win|windows)
    echo "Сборка под Windows (.exe)…"
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o WidthCorrector.exe .
    echo "Готово: WidthCorrector.exe"
    ;;
  mac|darwin)
    echo "Сборка под macOS…"
    go build -ldflags "-s -w" -o WidthCorrector .
    echo "Готово: WidthCorrector"
    ;;
  *)
    echo "Локальный запуск…"
    go run .
    ;;
esac
