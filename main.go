package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/systray"
	"github.com/ncruces/zenity"
)

// Интерфейс встроен в сжатом виде (gzip), чтобы его нельзя было
// найти и отредактировать простым поиском по строкам в .exe.
//
//go:embed assets/index.html.gz
var indexGz []byte

// иконка приложения (вкладка браузера)
//
//go:embed assets/favicon.ico
var faviconICO []byte

// Three.js для 3D-модели двигателя (встроена сжатой, отдаётся с gzip)
//
//go:embed assets/three.module.min.js.gz
var threeJsGz []byte

// Monaco editor (движок VS Code) — встроен целиком, отдаётся офлайн на /vs/
//
//go:embed all:assets/vs
var monacoFS embed.FS

// распакованный HTML (заполняется в init)
var indexHTML []byte

func init() {
	zr, err := gzip.NewReader(bytes.NewReader(indexGz))
	if err != nil {
		panic("не удалось прочитать встроенный интерфейс: " + err.Error())
	}
	defer zr.Close()
	indexHTML, err = io.ReadAll(zr)
	if err != nil {
		panic("не удалось распаковать встроенный интерфейс: " + err.Error())
	}
}

// Config — то, что хранится в configs/<hash>.json
type Config struct {
	FolderPath string             `json:"folderPath"`
	FolderName string             `json:"folderName"`
	Settings   map[string]float64 `json:"settings"`
	SavedAt    string             `json:"savedAt"`
}

// last.json — указатель на последнюю выбранную папку
type Last struct {
	FolderPath string `json:"folderPath"`
}

// AppConfig — глобальные настройки приложения (configs/app.json).
// Язык интерфейса: "en" (по умолчанию) или "ru".
type AppConfig struct {
	Lang string `json:"lang"`
}

type server struct {
	configsDir string
}

const appURL = "http://127.0.0.1:8723"

func main() {
	base := exeDir()
	configsDir := filepath.Join(base, "configs")
	if err := os.MkdirAll(configsDir, 0o755); err != nil {
		fmt.Println("Не удалось создать папку configs:", err)
	}

	srv := &server{configsDir: configsDir}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleIndex)
	mux.HandleFunc("/favicon.ico", srv.handleFavicon)
	mux.HandleFunc("/three.module.js", srv.handleThree)
	// Monaco editor (VS Code) — статика из встроенной FS на /vs/
	if monacoSub, err := fs.Sub(monacoFS, "assets/vs"); err == nil {
		mux.Handle("/vs/", monacoContentType(http.StripPrefix("/vs/", http.FileServer(http.FS(monacoSub)))))
	}
	mux.HandleFunc("/api/pick-folder", srv.handlePickFolder)
	mux.HandleFunc("/api/config", srv.handleConfig)
	mux.HandleFunc("/api/app-config", srv.handleAppConfig)
	mux.HandleFunc("/api/backups", srv.handleBackups)
	mux.HandleFunc("/api/backups/open", srv.handleOpenBackup)
	mux.HandleFunc("/api/apply", srv.handleApply)
	mux.HandleFunc("/api/scan", srv.handleScan)
	mux.HandleFunc("/api/files", srv.handleFiles)
	mux.HandleFunc("/api/model", srv.handleModel)
	mux.HandleFunc("/api/engine", srv.handleEngine)
	mux.HandleFunc("/api/engine/apply", srv.handleEngineApply)
	mux.HandleFunc("/api/engine/stage", srv.handleStageApply)
	mux.HandleFunc("/api/engine/preview", srv.handleEnginePreview)
	mux.HandleFunc("/api/engine/create", srv.handleEngineCreate)
	mux.HandleFunc("/api/engine/parse", srv.handleEngineParse)
	mux.HandleFunc("/api/constructor/draft", srv.handleConstructorDraft)
	mux.HandleFunc("/api/code/pick-folder", srv.handleCodePickFolder)
	mux.HandleFunc("/api/code/tree", srv.handleCodeTree)
	mux.HandleFunc("/api/code/file", srv.handleCodeFile)
	mux.HandleFunc("/api/code/raw", srv.handleCodeRaw)
	mux.HandleFunc("/api/code/save", srv.handleCodeSave)
	mux.HandleFunc("/api/code/config", srv.handleCodeConfig)
	mux.HandleFunc("/api/code/search", srv.handleCodeSearch)

	addr := strings.TrimPrefix(appURL, "http://")

	// занимаем порт сразу — если занят, показываем диалог вместо тихого падения
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		zenity.Error(srv.tr(
			"Failed to start the server:\n"+err.Error()+"\n\nThe program may already be running.",
			"Не удалось запустить сервер:\n"+err.Error()+"\n\nВозможно, программа уже запущена."),
			zenity.Title("WidthCorrector"))
		return
	}

	fmt.Println("WidthCorrector запущен на", appURL)
	fmt.Println("Папка конфигов:", configsDir)

	go func() {
		if err := http.Serve(ln, mux); err != nil {
			fmt.Println("Ошибка сервера:", err)
		}
	}()

	// сразу открываем интерфейс в браузере
	go func() {
		time.Sleep(400 * time.Millisecond)
		openBrowser(appURL)
	}()

	// иконка в трее с меню (блокирует main до выхода)
	systray.Run(func() { onTrayReady(srv) }, onTrayExit)
}

// ---------- системный трей ----------

func onTrayReady(s *server) {
	systray.SetIcon(faviconICO)
	systray.SetTitle("")
	systray.SetTooltip("WidthCorrector")

	mOpen := systray.AddMenuItem(
		s.tr("Open program", "Открыть программу"),
		s.tr("Open the interface in the browser", "Открыть интерфейс в браузере"))
	systray.AddSeparator()
	mQuit := systray.AddMenuItem(
		s.tr("Quit program", "Закрыть программу"),
		s.tr("Exit completely", "Полностью завершить работу"))

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openBrowser(appURL)
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onTrayExit() {
	os.Exit(0)
}

// ---------- HTTP-обработчики ----------

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// "/" и "/editor" отдают одно приложение (фронт сам открывает редактор по пути)
	if r.URL.Path != "/" && r.URL.Path != "/editor" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

func (s *server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(faviconICO)
}

// monacoContentType форсит правильный MIME (на Windows реестр отдаёт .js как text/plain,
// .ttf как octet-stream → модули и иконки Monaco ломаются) + кэш (имена файлов стабильны).
func monacoContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".js"):
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case strings.HasSuffix(r.URL.Path, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case strings.HasSuffix(r.URL.Path, ".ttf"):
			w.Header().Set("Content-Type", "font/ttf")
		case strings.HasSuffix(r.URL.Path, ".json") || strings.HasSuffix(r.URL.Path, ".map"):
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}

func (s *server) handleThree(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(threeJsGz)
}

// POST /api/pick-folder — нативный диалог выбора папки
func (s *server) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	path, err := zenity.SelectFile(
		zenity.Directory(),
		zenity.Title(s.tr("Select a folder with .jbeam files", "Выберите папку с файлами .jbeam")),
	)
	if err != nil {
		if err == zenity.ErrCanceled {
			writeJSON(w, map[string]any{"ok": false, "error": "cancelled"})
			return
		}
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	folder := s.folderInfo(path)

	// сохраняем как последнюю выбранную
	s.saveLast(path)

	// подгружаем настройки этой папки (или дефолтные)
	cfg, ok := s.loadConfig(path)
	var settings map[string]float64
	if ok {
		settings = cfg.Settings
	} else {
		settings = defaultSettings()
	}

	writeJSON(w, map[string]any{
		"ok":       true,
		"folder":   folder,
		"settings": settings,
	})
}

// GET  /api/config  — восстановить последнюю сессию
// POST /api/config  — сохранить настройки текущей папки
func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		last, ok := s.loadLast()
		if !ok {
			writeJSON(w, map[string]any{"ok": true, "folder": nil})
			return
		}
		// папка могла быть удалена/перемещена
		if !dirExists(last.FolderPath) {
			writeJSON(w, map[string]any{"ok": true, "folder": nil})
			return
		}
		cfg, found := s.loadConfig(last.FolderPath)
		settings := defaultSettings()
		if found {
			settings = cfg.Settings
		}
		writeJSON(w, map[string]any{
			"ok":       true,
			"folder":   s.folderInfo(last.FolderPath),
			"settings": settings,
		})

	case http.MethodPost:
		var body struct {
			FolderPath string             `json:"folderPath"`
			Settings   map[string]float64 `json:"settings"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "bad json: " + err.Error()})
			return
		}
		if body.FolderPath == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "no folderPath"})
			return
		}
		file, err := s.saveConfig(body.FolderPath, body.Settings)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		s.saveLast(body.FolderPath)
		writeJSON(w, map[string]any{"ok": true, "file": "configs/" + file})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ---------- язык интерфейса (app.json) ----------

const defaultLang = "en"

func (s *server) appConfigPath() string {
	return filepath.Join(s.configsDir, "app.json")
}

func (s *server) loadAppConfig() AppConfig {
	cfg := AppConfig{Lang: defaultLang}
	data, err := os.ReadFile(s.appConfigPath())
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return AppConfig{Lang: defaultLang}
	}
	if cfg.Lang != "ru" && cfg.Lang != "en" {
		cfg.Lang = defaultLang
	}
	return cfg
}

func (s *server) saveAppConfig(cfg AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.appConfigPath(), data, 0o644)
}

// tr возвращает строку на языке интерфейса (для нативных элементов: трей, диалоги).
func (s *server) tr(en, ru string) string {
	if s.loadAppConfig().Lang == "ru" {
		return ru
	}
	return en
}

// GET  /api/app-config — текущий язык интерфейса
// POST /api/app-config — сохранить выбранный язык (body: {lang})
func (s *server) handleAppConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"ok": true, "lang": s.loadAppConfig().Lang})

	case http.MethodPost:
		var body struct {
			Lang string `json:"lang"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "bad json: " + err.Error()})
			return
		}
		if body.Lang != "ru" && body.Lang != "en" {
			body.Lang = defaultLang
		}
		if err := s.saveAppConfig(AppConfig{Lang: body.Lang}); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "lang": body.Lang})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ---------- бекапы ----------

const maxBackups = 5

type BackupInfo struct {
	ID       string `json:"id"`       // = имя папки (дата_время)
	DateTime string `json:"datetime"` // человекочитаемая дата/время
	Count    int    `json:"count"`    // сколько .jbeam в бекапе
	Path     string `json:"path"`     // полный путь
}

type backupMeta struct {
	Source    string `json:"source"`
	CreatedAt string `json:"createdAt"`
}

func (s *server) backupsDir() string {
	return filepath.Join(s.configsDir, "backups")
}

// GET  /api/backups — список бекапов
// POST /api/backups — создать бекап (body: {folderPath})
func (s *server) handleBackups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"ok": true, "backups": s.listBackups(), "dir": s.backupsDir()})

	case http.MethodPost:
		var body struct {
			FolderPath string `json:"folderPath"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.FolderPath == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "no folderPath"})
			return
		}
		if !dirExists(body.FolderPath) {
			writeJSON(w, map[string]any{"ok": false, "error": "папка не найдена"})
			return
		}
		info, err := s.createBackup(body.FolderPath)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "created": info, "backups": s.listBackups()})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// POST /api/backups/open — открыть папку бекапа (body: {id}) или корень бекапов
func (s *server) handleOpenBackup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	target := s.backupsDir()
	if body.ID != "" {
		// защита от выхода за пределы папки бекапов
		target = filepath.Join(s.backupsDir(), filepath.Base(body.ID))
	}
	if !dirExists(target) {
		if body.ID == "" {
			os.MkdirAll(target, 0o755) // корень бекапов создаём, если его ещё нет
		} else {
			writeJSON(w, map[string]any{"ok": false, "error": "папка бекапа не найдена"})
			return
		}
	}
	if err := openPath(target); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *server) createBackup(srcFolder string) (BackupInfo, error) {
	dir := s.backupsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return BackupInfo{}, err
	}

	now := time.Now()
	name := now.Format("2006-01-02_15-04-05")
	dest := filepath.Join(dir, name)
	// гарантируем уникальность имени, если за ту же секунду создаётся повторно
	for i := 1; dirExists(dest); i++ {
		name = now.Format("2006-01-02_15-04-05") + fmt.Sprintf("_%d", i)
		dest = filepath.Join(dir, name)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return BackupInfo{}, err
	}

	count := 0
	filepath.WalkDir(srcFolder, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".jbeam") {
			return nil
		}
		rel, e := filepath.Rel(srcFolder, p)
		if e != nil {
			return nil
		}
		out := filepath.Join(dest, rel)
		if e := os.MkdirAll(filepath.Dir(out), 0o755); e != nil {
			return nil
		}
		if e := copyFile(p, out); e == nil {
			count++
		}
		return nil
	})

	// служебная мета (откуда сделан бекап)
	meta, _ := json.MarshalIndent(backupMeta{
		Source:    srcFolder,
		CreatedAt: now.Format(time.RFC3339),
	}, "", "  ")
	os.WriteFile(filepath.Join(dest, "_backup.json"), meta, 0o644)

	s.pruneBackups() // держим максимум maxBackups

	return BackupInfo{
		ID:       name,
		DateTime: formatBackupTime(name),
		Count:    count,
		Path:     dest,
	}, nil
}

// оставляем максимум maxBackups, удаляя самые старые
func (s *server) pruneBackups() {
	entries, err := os.ReadDir(s.backupsDir())
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // имена-таймстампы сортируются хронологически
	for len(names) > maxBackups {
		os.RemoveAll(filepath.Join(s.backupsDir(), names[0]))
		names = names[1:]
	}
}

func (s *server) listBackups() []BackupInfo {
	entries, err := os.ReadDir(s.backupsDir())
	if err != nil {
		return []BackupInfo{}
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names))) // новые сверху
	out := make([]BackupInfo, 0, len(names))
	for _, name := range names {
		full := filepath.Join(s.backupsDir(), name)
		out = append(out, BackupInfo{
			ID:       name,
			DateTime: formatBackupTime(name),
			Count:    countJbeam(full),
			Path:     full,
		})
	}
	return out
}

// "2026-06-30_15-30-05" -> "30.06.2026 15:30:05"
func formatBackupTime(name string) string {
	base := name
	parts := strings.Split(name, "_")
	if len(parts) >= 2 {
		base = parts[0] + "_" + parts[1]
	}
	if t, err := time.Parse("2006-01-02_15-04-05", base); err == nil {
		return t.Format("02.01.2006 15:04:05")
	}
	return name
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func openPath(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// ---------- работа с конфигами ----------

func (s *server) configFileName(folderPath string) string {
	h := sha1.Sum([]byte(filepath.Clean(folderPath)))
	short := hex.EncodeToString(h[:])[:8]
	name := sanitize(filepath.Base(folderPath))
	if name == "" {
		name = "folder"
	}
	return name + "_" + short + ".json"
}

func (s *server) configPath(folderPath string) string {
	return filepath.Join(s.configsDir, s.configFileName(folderPath))
}

func (s *server) loadConfig(folderPath string) (Config, bool) {
	data, err := os.ReadFile(s.configPath(folderPath))
	if err != nil {
		return Config{}, false
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false
	}
	if cfg.Settings == nil {
		cfg.Settings = defaultSettings()
	}
	return cfg, true
}

func (s *server) saveConfig(folderPath string, settings map[string]float64) (string, error) {
	if settings == nil {
		settings = defaultSettings()
	}
	cfg := Config{
		FolderPath: folderPath,
		FolderName: filepath.Base(folderPath),
		Settings:   settings,
		SavedAt:    time.Now().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	file := s.configFileName(folderPath)
	if err := os.WriteFile(filepath.Join(s.configsDir, file), data, 0o644); err != nil {
		return "", err
	}
	return file, nil
}

func (s *server) saveLast(folderPath string) {
	data, _ := json.MarshalIndent(Last{FolderPath: folderPath}, "", "  ")
	os.WriteFile(filepath.Join(s.configsDir, "last.json"), data, 0o644)
}

func (s *server) loadLast() (Last, bool) {
	data, err := os.ReadFile(filepath.Join(s.configsDir, "last.json"))
	if err != nil {
		return Last{}, false
	}
	var l Last
	if err := json.Unmarshal(data, &l); err != nil || l.FolderPath == "" {
		return Last{}, false
	}
	return l, true
}

// ---------- вспомогательное ----------

func (s *server) folderInfo(path string) map[string]any {
	return map[string]any{
		"path":       path,
		"name":       filepath.Base(path),
		"jbeamCount": countJbeam(path),
	}
}

// рекурсивно считаем .jbeam в папке
func countJbeam(root string) int {
	count := 0
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".jbeam") {
			count++
		}
		return nil
	})
	return count
}

func defaultSettings() map[string]float64 {
	return map[string]float64{
		"nodeWeight":   100,
		"beamDeform":   100,
		"beamSpring":   100,
		"beamDamp":     100,
		"beamStrength": 100,
	}
}

func sanitize(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

// директория, где лежит исполняемый файл (рядом с ним будет configs/)
func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		if wd, e := os.Getwd(); e == nil {
			return wd
		}
		return "."
	}
	return filepath.Dir(exe)
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		fmt.Println("Откройте в браузере вручную:", url)
	}
}
