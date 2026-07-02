<div align="center">

### [🇬🇧 English](#-english) · [🇷🇺 Русский](#-русский)

</div>

---

<a name="-english"></a>
# WidthCorrector
Local application for batch correction of `.jbeam` parameters (BeamNG).
The Go backend spins up a local server, serves an embedded HTML interface, and stores
settings in the `configs/` folder next to the executable.
## How it works
- `main.go` - Go server. HTML is embedded directly into the binary via `go:embed`,
  so the output is **a single file** `.exe` - nothing extra to carry around.
- On the first save, a `configs/` folder is created next to the `.exe`.
- For each working folder, its own file `configs/<name>_<hash>.json` is created.
- `configs/last.json` remembers the last selected folder — on startup the session
  is restored automatically.
- The folder is selected via the **native OS dialog**, so the real path is preserved.
## Requirements
Go 1.21+ installed. On macOS:
\`\`\`bash
brew install go
\`\`\`
## Running in development (macOS)
\`\`\`bash
./build.sh          # go run . - starts the server and opens the browser
\`\`\`
## Building .exe for Windows (cross-compilation from Mac)
zenity on Windows works without cgo, so you can build the `.exe` right from a Mac:
\`\`\`bash
./build.sh win      # creates WidthCorrector.exe
\`\`\`
The `-H windowsgui` flag removes the black console window. If you want to see the server
log in the console - remove it from `build.sh`.
## Address
After startup the server listens on `http://127.0.0.1:8723` and opens the browser itself.
## API (for reference)
- `GET  /`                                       - HTML interface
- `POST /api/pick-folder` - open the native dialog, return the path + number of `.jbeam`
- `GET  /api/config`              - restore the last session
- `POST /api/config`              - save the current folder's settings
## Contact
Message here - https://t.me/danilka_pikaso

---

<a name="-русский"></a>
# WidthCorrector

Локальное приложение для пакетной коррекции параметров `.jbeam` (BeamNG).
Go-бэкенд поднимает локальный сервер, отдаёт встроенный HTML-интерфейс и хранит
настройки в папке `configs/` рядом с исполняемым файлом.

## Как это устроено

- `main.go` - Go-сервер. HTML встроен прямо в бинарник через `go:embed`,
  поэтому на выходе **один файл** `.exe` - ничего рядом таскать не нужно.
- При первом сохранении рядом с `.exe` создаётся папка `configs/`.
- Для каждой рабочей папки создаётся свой файл `configs/<имя>_<хэш>.json`.
- `configs/last.json` помнит последнюю выбранную папку — при запуске сессия
  восстанавливается автоматически.
- Папка выбирается **нативным диалогом ОС**, поэтому сохраняется реальный путь.

## Требования

Установленный Go 1.21+. На macOS:

\`\`\`bash
brew install go
\`\`\`

## Запуск при разработке (macOS)

\`\`\`bash
./build.sh          # go run . - поднимет сервер и откроет браузер
\`\`\`

## Сборка .exe для Windows (кросс-компиляция с Mac)

zenity на Windows работает без cgo, поэтому собрать `.exe` можно прямо с Mac:

\`\`\`bash
./build.sh win      # создаст WidthCorrector.exe
\`\`\`

Флаг `-H windowsgui` убирает чёрное окно консоли. Если хочешь видеть лог
сервера в консоли - убери его из `build.sh`.

## Адрес

После запуска сервер слушает `http://127.0.0.1:8723` и сам открывает браузер.

## API (для справки)

- `GET  /`                                       - HTML-интерфейс
- `POST /api/pick-folder` - открыть нативный диалог, вернуть путь + число `.jbeam`
- `GET  /api/config`              - восстановить последнюю сессию
- `POST /api/config`              - сохранить настройки текущей папки

## Связь
Пишите сюда - https://t.me/danilka_pikaso