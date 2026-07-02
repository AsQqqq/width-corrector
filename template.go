package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ncruces/zenity"
)

// tplParams - параметры генерации шаблона мода (из модалки «Создать шаблон»).
type tplParams struct {
	FolderPath string `json:"folderPath"`
	Lang       string `json:"lang"` // язык комментариев/README: "en" | "ru"
	ModID      string `json:"modId"`
	Name       string `json:"name"`
	Author     string `json:"author"`
}

// мусорные файлы, которые не считаем содержимым при проверке «папка пустая»
var junkNames = map[string]bool{
	".DS_Store": true, "Thumbs.db": true, "desktop.ini": true,
}

// dirEmpty - true, если в папке нет значимых файлов (мусор игнорируется).
func dirEmpty(path string) (bool, int) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, 0
	}
	count := 0
	for _, e := range entries {
		if junkNames[e.Name()] {
			continue
		}
		count++
	}
	return count == 0, count
}

// cm возвращает строку на нужном языке (для комментариев/README).
func cm(lang, en, ru string) string {
	if lang == "ru" {
		return ru
	}
	return en
}

// sanitizeModID приводит имя к безопасному идентификатору (ascii, латиница/цифры/_).
func sanitizeModID(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "my_car"
	}
	// имя детали не должно начинаться с цифры
	if out[0] >= '0' && out[0] <= '9' {
		out = "car_" + out
	}
	return out
}

// POST /api/template/pick-folder - нативный диалог выбора папки под мод.
func (s *server) handleTemplatePickFolder(w http.ResponseWriter, r *http.Request) {
	path, err := zenity.SelectFile(
		zenity.Directory(),
		zenity.Title(s.tr("Select an EMPTY folder for the mod", "Выберите ПУСТУЮ папку под мод")),
	)
	if err != nil {
		if err == zenity.ErrCanceled {
			writeJSON(w, map[string]any{"ok": false, "error": "cancelled"})
			return
		}
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	empty, count := dirEmpty(path)
	writeJSON(w, map[string]any{
		"ok":    true,
		"path":  path,
		"name":  filepath.Base(path),
		"empty": empty,
		"count": count,
	})
}

// POST /api/template/create - сгенерировать базовую структуру мода в выбранной пустой папке.
func (s *server) handleTemplateCreate(w http.ResponseWriter, r *http.Request) {
	var p tplParams
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad json: " + err.Error()})
		return
	}
	if p.Lang != "ru" && p.Lang != "en" {
		p.Lang = "en"
	}
	if p.FolderPath == "" {
		writeJSON(w, map[string]any{"ok": false, "error": s.tr("no folder selected", "папка не выбрана")})
		return
	}
	if !dirExists(p.FolderPath) {
		writeJSON(w, map[string]any{"ok": false, "error": s.tr("folder not found", "папка не найдена")})
		return
	}
	// обязательное требование: папка должна быть пустой
	if empty, _ := dirEmpty(p.FolderPath); !empty {
		writeJSON(w, map[string]any{"ok": false, "error": s.tr(
			"the selected folder is not empty - choose an empty folder",
			"выбранная папка не пустая - выберите пустую папку")})
		return
	}

	p.ModID = sanitizeModID(p.ModID)
	if strings.TrimSpace(p.Name) == "" {
		p.Name = "My Car"
	}
	if strings.TrimSpace(p.Author) == "" {
		p.Author = "You"
	}

	files := generateMod(p)

	// пишем файлы
	written := make([]string, 0, len(files))
	for rel, content := range files {
		full := filepath.Join(p.FolderPath, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		written = append(written, rel)
	}

	writeJSON(w, map[string]any{"ok": true, "files": written, "folder": p.FolderPath})
}

// generateMod возвращает карту «относительный путь -> содержимое» базового мода.
func generateMod(p tplParams) map[string]string {
	veh := "vehicles/" + p.ModID + "/"
	return map[string]string{
		"README.md":                    tplREADME(p),
		veh + p.ModID + ".jbeam":        tplMainJbeam(p),
		veh + p.ModID + "_engine.jbeam": tplEngineJbeam(p),
		veh + "info.json":               tplInfoJSON(p),
		veh + p.ModID + ".pc":           tplConfigPC(p),
	}
}
