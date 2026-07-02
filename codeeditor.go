package main

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ncruces/zenity"
)

// расширения, которые показываем как картинку (превью в браузере)
var codeImageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".bmp": true, ".webp": true, ".ico": true,
}

const codeMaxTextSize = 6 << 20 // 6 МБ — выше считаем «слишком большой для редактора»

// safeReal = safeJoin + разрешение симлинков и повторное удержание внутри папки.
// Для ещё не существующего файла (новое сохранение) проверяет родительскую папку.
func (s *server) safeReal(folder, rel string) (string, bool) {
	clean, ok := s.safeJoin(folder, rel)
	if !ok {
		return "", false
	}
	baseReal, err := filepath.EvalSymlinks(filepath.Clean(folder))
	if err != nil {
		return "", false
	}
	real, err := filepath.EvalSymlinks(clean)
	if err != nil {
		pd, e := filepath.EvalSymlinks(filepath.Dir(clean))
		if e != nil {
			return "", false
		}
		real = filepath.Join(pd, filepath.Base(clean))
	}
	if real != baseReal && !strings.HasPrefix(real, baseReal+string(os.PathSeparator)) {
		return "", false
	}
	return real, true
}

func hasNullByte(data []byte) bool {
	n := len(data)
	if n > 8000 {
		n = 8000
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// POST /api/code/pick-folder — нативный диалог выбора папки мода (независимо от рабочей папки)
func (s *server) handleCodePickFolder(w http.ResponseWriter, r *http.Request) {
	path, err := zenity.SelectFile(
		zenity.Directory(),
		zenity.Title("Выберите папку с модом для редактора кода"),
	)
	if err != nil {
		if err == zenity.ErrCanceled {
			writeJSON(w, map[string]any{"ok": false, "error": "cancelled"})
			return
		}
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "path": path, "name": filepath.Base(path)})
}

type codeEntry struct {
	Name string `json:"name"`
	Path string `json:"path"` // относительный путь (ToSlash)
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

// GET /api/code/tree?folderPath=...&sub=...  — ленивый список содержимого подпапки
func (s *server) handleCodeTree(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folderPath")
	sub := r.URL.Query().Get("sub")
	base, ok := s.safeReal(folder, sub)
	if !ok {
		writeJSON(w, map[string]any{"ok": false, "error": "недопустимый путь"})
		return
	}
	dirents, err := os.ReadDir(base)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	entries := []codeEntry{}
	for _, d := range dirents {
		rel := filepath.ToSlash(filepath.Join(sub, d.Name()))
		var size int64
		if info, e := d.Info(); e == nil {
			size = info.Size()
		}
		entries = append(entries, codeEntry{Name: d.Name(), Path: rel, Dir: d.IsDir(), Size: size})
	}
	// папки сверху, затем по имени (регистронезависимо)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	writeJSON(w, map[string]any{"ok": true, "entries": entries})
}

// GET /api/code/file?folderPath=...&path=...  — содержимое (text) / тип (image/binary)
func (s *server) handleCodeFile(w http.ResponseWriter, r *http.Request) {
	clean, ok := s.safeReal(r.URL.Query().Get("folderPath"), r.URL.Query().Get("path"))
	if !ok {
		writeJSON(w, map[string]any{"ok": false, "error": "недопустимый путь"})
		return
	}
	fi, err := os.Stat(clean)
	if err != nil || fi.IsDir() {
		writeJSON(w, map[string]any{"ok": false, "error": "файл не найден"})
		return
	}
	ext := strings.ToLower(filepath.Ext(clean))
	if codeImageExts[ext] {
		writeJSON(w, map[string]any{"ok": true, "kind": "image", "size": fi.Size()})
		return
	}
	if fi.Size() > codeMaxTextSize {
		writeJSON(w, map[string]any{"ok": true, "kind": "binary", "size": fi.Size(), "reason": "слишком большой"})
		return
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if hasNullByte(data) {
		writeJSON(w, map[string]any{"ok": true, "kind": "binary", "size": fi.Size()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "kind": "text", "text": string(data), "size": fi.Size()})
}

// GET /api/code/raw?folderPath=...&path=...  — сырые байты (для <img>)
func (s *server) handleCodeRaw(w http.ResponseWriter, r *http.Request) {
	clean, ok := s.safeReal(r.URL.Query().Get("folderPath"), r.URL.Query().Get("path"))
	if !ok {
		http.Error(w, "недопустимый путь", http.StatusBadRequest)
		return
	}
	if fi, err := os.Stat(clean); err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, clean) // content-type по расширению, поддержка range
}

// GET/POST /api/code/config — настройки редактора (папка + автосохранение), configs/editor.json
func (s *server) handleCodeConfig(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(s.configsDir, "editor.json")
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		os.WriteFile(path, body, 0o644)
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	data, err := os.ReadFile(path)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		w.Write([]byte("null"))
		return
	}
	w.Write(data)
}

// расширения, которые точно бинарные — пропускаем при поиске
var codeBinaryExts = map[string]bool{
	".dds": true, ".cdae": true, ".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".bmp": true, ".webp": true, ".ico": true, ".ogg": true,
	".wav": true, ".mp3": true, ".zip": true, ".dae": false, // dae — текст (xml)
}

type searchHit struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// GET /api/code/search?folderPath=...&q=...  — глобальный поиск по тексту файлов
func (s *server) handleCodeSearch(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folderPath")
	q := r.URL.Query().Get("q")
	base, ok := s.safeReal(folder, "")
	if !ok || strings.TrimSpace(q) == "" {
		writeJSON(w, map[string]any{"ok": true, "results": []searchHit{}, "capped": false})
		return
	}
	ql := strings.ToLower(q)
	hits := []searchHit{}
	capped := false
	const maxHits = 600
	const maxFileSize = 2 << 20 // 2 МБ

	filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if codeImageExts[ext] || codeBinaryExts[ext] {
			return nil
		}
		info, e := d.Info()
		if e != nil || info.Size() > maxFileSize {
			return nil
		}
		data, e := os.ReadFile(p)
		if e != nil || hasNullByte(data) {
			return nil
		}
		rel, _ := filepath.Rel(base, p)
		rel = filepath.ToSlash(rel)
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(strings.ToLower(line), ql) {
				t := strings.TrimSpace(line)
				if len(t) > 200 {
					t = t[:200]
				}
				hits = append(hits, searchHit{File: rel, Line: i + 1, Text: t})
				if len(hits) >= maxHits {
					capped = true
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	writeJSON(w, map[string]any{"ok": true, "results": hits, "capped": capped})
}

// POST /api/code/save  — body {folderPath, path, text}
func (s *server) handleCodeSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FolderPath string `json:"folderPath"`
		Path       string `json:"path"`
		Text       string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad json"})
		return
	}
	clean, ok := s.safeReal(body.FolderPath, body.Path)
	if !ok {
		writeJSON(w, map[string]any{"ok": false, "error": "недопустимый путь"})
		return
	}
	mode := os.FileMode(0o644)
	if fi, e := os.Stat(clean); e == nil {
		if fi.IsDir() {
			writeJSON(w, map[string]any{"ok": false, "error": "это папка"})
			return
		}
		mode = fi.Mode()
	}
	if err := os.WriteFile(clean, []byte(body.Text), mode); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
