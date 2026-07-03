<div align="center">

<img src="https://raw.githubusercontent.com/AsQqqq/width-corrector/refs/heads/master/repo/banner.png" alt="WidthCorrector banner" width="100%">

<br>

**A toolkit for BeamNG vehicle modding - batch `.jbeam` editing, engine tuning, a built-in code editor and the full offline documentation, all in a single portable `.exe`.**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey)]()
[![Offline](https://img.shields.io/badge/100%25-offline-36d399)]()
[![Lang](https://img.shields.io/badge/UI-RU%20%7C%20EN-ff7a45)]()
[![License](https://img.shields.io/badge/license-MIT-green)]()

### [English](#-english) · [Русский](#-русский)

</div>

---

<a name="-english"></a>
## 🇬🇧 English

**WidthCorrector** is a local desktop toolkit for BeamNG.drive vehicle modding. A Go backend spins up a local server and serves an interface that is fully embedded into the binary - one `.exe`, no installer, no dependencies, works completely offline. The app lives in the system tray and opens in your default browser.

### 🧰 What's inside

The interface is split into pages, plus a couple of full-screen tools:

- **Variables** - batch correction of 5 `.jbeam` parameters across a whole folder: `nodeWeight`, `beamDeform`, `beamSpring`, `beamDamp`, `beamStrength`. Each is a 0-200% dial (100% = no change); the new value is `current x percent/100`. Recursive over every `.jbeam`, skips commented and non-numeric values, makes a backup before writing.
- **Engine** - engine tuning: live power/torque graph, editable torque curve, all `mainEngine` parameters (idle/max/rev-limiter RPM, inertia, friction, damage thresholds), tuning stages (intake/exhaust/ECU curves) and an interactive "pit-stop" dyno.
- **Constructor** - generates a complete, valid engine `.jbeam` from scratch (nodes/beams, torque curve, all parameters) with a live two-way code editor.
- **Code editor** - a full Monaco (VS Code) editor embedded offline: file tree, tabs, search, syntax highlighting for `.jbeam`/`.lua`/`.json`, image preview. Opens on its own URL `/editor`.
- **3D viewer** - a wireframe view of the nodes/beams of any `.jbeam`, with per-node/beam parameter labels.

### 📚 Offline documentation (new)

A **complete offline copy of the official BeamNG modding documentation** is built right into the app - so you can read it without internet, right next to your files.

- **All 232 pages** from [documentation.beamng.com/modding](https://documentation.beamng.com/modding/), with the same structure: a navigation tree on the left, the article in the center.
- **Two languages.** English is the original; Russian is a full machine translation of every page (each page carries a note that the translation is automatic and links to the original).
- **Everything is embedded** - text, tables, code blocks with syntax highlighting, and all 578 screenshots / diagrams / GIFs - so it all works offline.
- **Bookmarkable URLs.** Every page has its own address, e.g. `/docs/vehicle/intro_jbeam/jbeamsyntax`, so a specific page can be pinned.
- **Mod template generator.** The button at the bottom of the tree creates a ready starter skeleton of a car mod (folder structure, base `.jbeam`, engine, `info.json`, `.pc`) with comments in the chosen language.

### 🌍 Bilingual & offline

The whole UI switches between **Russian and English** with one click (the choice is remembered). Everything - including the 12 MB Monaco editor and the 55 MB documentation - is embedded via `go:embed`, so the program is a single self-contained file that never needs the network.

### 📦 Requirements

Go 1.21+ installed. On macOS:

```bash
brew install go
```

### 🚀 Running in development (macOS)

```bash
./build.sh          # go run . - starts the server and opens the browser
```

### 🛠 Building

```bash
./build.sh win      # WidthCorrector.exe (Windows, cross-compiled from Mac)
./build.sh mac      # WidthCorrector (macOS)
```

`zenity` on Windows works without cgo, so the `.exe` builds straight from a Mac. The `-H windowsgui` flag removes the black console window (remove it in `build.sh` if you want the server log).

### 🌐 Address

After startup the server listens on `http://127.0.0.1:8723` and opens the browser automatically. Main routes: `/` (app), `/editor` (code editor), `/docs` (documentation).

### 💬 Contact

Message here - [@danilka_pikaso](https://t.me/danilka_pikaso)

---

<a name="-русский"></a>
## 🇷🇺 Русский

**WidthCorrector** - локальный десктоп-набор инструментов для моддинга машин в BeamNG.drive. Go-бэкенд поднимает локальный сервер и отдаёт интерфейс, полностью встроенный в бинарник - один `.exe`, без установщика и зависимостей, работает целиком офлайн. Программа висит в системном трее и открывается в браузере по умолчанию.

### 🧰 Что внутри

Интерфейс разбит на страницы плюс пара полноэкранных инструментов:

- **Переменные** - пакетная коррекция 5 параметров `.jbeam` по всей папке: `nodeWeight`, `beamDeform`, `beamSpring`, `beamDamp`, `beamStrength`. Каждый - крутилка 0-200% (100% = без изменений); новое значение = `текущее x процент/100`. Рекурсивно по всем `.jbeam`, пропускает закомментированные и нечисловые значения, перед записью делает бэкап.
- **Двигатель** - настройка мотора: живой график мощности/момента, редактируемая кривая момента, все параметры `mainEngine` (холостые/макс/отсечка, инерция, трение, пороги повреждений), стейджи тюнинга (кривые интейка/выхлопа/ECU) и интерактивный «пит-стоп»-дино.
- **Конструктор** - генерирует полный валидный `.jbeam` двигателя с нуля (ноды/балки, кривая момента, все параметры) с живым двусторонним редактором кода.
- **Редактор кода** - полноценный Monaco (VS Code), встроенный офлайн: дерево файлов, вкладки, поиск, подсветка `.jbeam`/`.lua`/`.json`, превью картинок. Открывается по своему URL `/editor`.
- **3D-просмотр** - каркас нод/балок любого `.jbeam` с подписями параметров по узлам и балкам.

### 📚 Офлайн-документация (новое)

В программу встроена **полная офлайн-копия официальной документации BeamNG по моддингу** - можно читать без интернета, прямо рядом со своими файлами.

- **Все 232 страницы** с [documentation.beamng.com/modding](https://documentation.beamng.com/modding/), с той же структурой: дерево навигации слева, статья по центру.
- **Два языка.** Английский - оригинал; русский - полный машинный перевод каждой страницы (на каждой странице есть пометка, что перевод автоматический, и ссылка на оригинал).
- **Всё встроено** - текст, таблицы, блоки кода с подсветкой и все 578 скриншотов / схем / GIF - поэтому работает офлайн.
- **URL для закладок.** У каждой страницы свой адрес, например `/docs/vehicle/intro_jbeam/jbeamsyntax`, так что конкретную страницу можно закрепить.
- **Генератор шаблона мода.** Кнопка внизу дерева создаёт готовый стартовый скелет мода машины (структура папок, базовый `.jbeam`, движок, `info.json`, `.pc`) с комментариями на выбранном языке.

### 🌍 Два языка и офлайн

Весь интерфейс переключается между **русским и английским** одной кнопкой (выбор запоминается). Всё - включая 12 МБ редактора Monaco и 55 МБ документации - встроено через `go:embed`, поэтому программа это один самодостаточный файл, которому никогда не нужна сеть.

### 📦 Требования

Установленный Go 1.21+. На macOS:

```bash
brew install go
```

### 🚀 Запуск при разработке (macOS)

```bash
./build.sh          # go run . - поднимет сервер и откроет браузер
```

### 🛠 Сборка

```bash
./build.sh win      # WidthCorrector.exe (Windows, кросс-компиляция с Mac)
./build.sh mac      # WidthCorrector (macOS)
```

`zenity` на Windows работает без cgo, поэтому `.exe` собирается прямо с Mac. Флаг `-H windowsgui` убирает чёрное окно консоли (убери его в `build.sh`, если нужен лог сервера).

### 🌐 Адрес

После запуска сервер слушает `http://127.0.0.1:8723` и сам открывает браузер. Основные маршруты: `/` (приложение), `/editor` (редактор кода), `/docs` (документация).

### 💬 Связь

Пишите сюда - [@danilka_pikaso](https://t.me/danilka_pikaso)
