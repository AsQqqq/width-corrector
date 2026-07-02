package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// мелкая задержка между файлами, чтобы не грузить диск на полную
// и чтобы прогресс на сайте шёл плавно
const processDelay = 6 * time.Millisecond

// параметры, которые умеем менять
var paramKeys = []string{"nodeWeight", "beamDeform", "beamSpring", "beamDamp", "beamStrength"}

// "<param>": <число>
// Группы: 1 = префикс ("key": с пробелами), 2 = имя параметра, 3 = число.
// Значения в кавычках (FLT_MAX, $spring_F, "2000") НЕ совпадают - они пропускаются.
var paramRe = regexp.MustCompile(
	`("(nodeWeight|beamDeform|beamSpring|beamDamp|beamStrength)"\s*:\s*)(-?\d+(?:\.\d+)?)`)

func isParam(s string) bool {
	for _, k := range paramKeys {
		if k == s {
			return true
		}
	}
	return false
}

// ---------- разбор строки на код и комментарий ----------

// codePart возвращает часть строки до комментария // (вне строковых литералов)
// и сам комментарий. Это нужно, чтобы НЕ трогать закомментированные параметры.
func codePart(line string) (code, comment string) {
	inStr := false
	esc := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			continue
		}
		if c == '/' && i+1 < len(line) && line[i+1] == '/' {
			return line[:i], line[i:]
		}
	}
	return line, ""
}

// processLine умножает в кодовой части строки значения параметров на их factor.
// factors: имя параметра -> множитель. Возвращает строку и счётчики по параметрам.
func processLine(line string, factors map[string]float64) (string, map[string]int) {
	code, comment := codePart(line)
	counts := map[string]int{}
	newCode := paramRe.ReplaceAllStringFunc(code, func(m string) string {
		sub := paramRe.FindStringSubmatch(m)
		key := sub[2]
		f, ok := factors[key]
		if !ok || f == 1.0 {
			return m // нет множителя или 100% - не трогаем
		}
		val, err := strconv.ParseFloat(sub[3], 64)
		if err != nil {
			return m
		}
		counts[key]++
		return sub[1] + formatNum(val*f)
	})
	return newCode + comment, counts
}

// formatNum форматирует число без мусора с плавающей точкой и лишних нулей
func formatNum(v float64) string {
	s := strconv.FormatFloat(v, 'f', 6, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-0" {
		s = "0"
	}
	return s
}

// ---------- обработка файлов ----------

func collectJbeam(folder string) []string {
	var files []string
	filepath.WalkDir(folder, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".jbeam") {
			files = append(files, p)
		}
		return nil
	})
	return files
}

func processFile(path string, factors map[string]float64) (changed bool, points int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, 0, err
	}
	mode := os.FileMode(0o644)
	if fi, e := os.Stat(path); e == nil {
		mode = fi.Mode()
	}

	lines := strings.Split(string(data), "\n") // \r\n сохраняется: \r остаётся в конце строки
	total := 0
	for i, ln := range lines {
		newLn, counts := processLine(ln, factors)
		if len(counts) > 0 {
			lines[i] = newLn
			for _, n := range counts {
				total += n
			}
		}
	}
	if total == 0 {
		return false, 0, nil
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), mode); err != nil {
		return false, 0, err
	}
	return true, total, nil
}

func relSlash(base, p string) string {
	rel, _ := filepath.Rel(base, p)
	return filepath.ToSlash(rel)
}

// processAll обрабатывает выбранные .jbeam (кроме excluded), вызывая progress по ходу
func processAll(folder string, factors map[string]float64, excluded map[string]bool, progress func(done, total int, file string)) (filesChanged, pointsChanged int, errs []string) {
	var files []string
	for _, p := range collectJbeam(folder) {
		if excluded[relSlash(folder, p)] {
			continue
		}
		files = append(files, p)
	}
	total := len(files)
	for i, p := range files {
		rel := relSlash(folder, p)
		progress(i, total, rel)
		changed, pts, err := processFile(p, factors)
		if err != nil {
			errs = append(errs, rel+": "+err.Error())
		} else if changed {
			filesChanged++
			pointsChanged += pts
		}
		time.Sleep(processDelay)
	}
	progress(total, total, "")
	return
}

// ---------- просмотр текущих значений ----------

type FileWeights struct {
	File   string    `json:"file"`
	Values []float64 `json:"values"`
}

// scanParam собирает текущие значения одного параметра по всем файлам
func scanParam(folder, param string) (files []FileWeights, totalPoints int) {
	for _, p := range collectJbeam(folder) {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var vals []float64
		for _, ln := range strings.Split(string(data), "\n") {
			code, _ := codePart(ln)
			if !strings.Contains(code, param) {
				continue
			}
			for _, m := range paramRe.FindAllStringSubmatch(code, -1) {
				if m[2] != param {
					continue
				}
				if v, e := strconv.ParseFloat(m[3], 64); e == nil {
					vals = append(vals, v)
				}
			}
		}
		if len(vals) > 0 {
			rel, _ := filepath.Rel(folder, p)
			files = append(files, FileWeights{File: rel, Values: vals})
			totalPoints += len(vals)
		}
	}
	return
}

// ---------- HTTP-обработчики ----------

// POST /api/apply  - body {folderPath, settings:{param:pct}, excluded:[relpaths]}
// Ответ - поток NDJSON (по одному JSON-событию в строке): phase/backup/progress/done/error
func (s *server) handleApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FolderPath string             `json:"folderPath"`
		Settings   map[string]float64 `json:"settings"`
		Excluded   []string           `json:"excluded"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	send := func(event string, payload map[string]any) {
		if payload == nil {
			payload = map[string]any{}
		}
		payload["event"] = event
		b, _ := json.Marshal(payload)
		w.Write(b)
		w.Write([]byte("\n"))
		flusher.Flush()
	}

	if body.FolderPath == "" || !dirExists(body.FolderPath) {
		send("error", map[string]any{"error": "папка не найдена"})
		return
	}

	// множители по всем параметрам
	factors := map[string]float64{}
	allHundred := true
	for _, key := range paramKeys {
		pct, has := body.Settings[key]
		if !has {
			continue
		}
		factors[key] = pct / 100
		if pct != 100 {
			allHundred = false
		}
	}
	if allHundred {
		send("done", map[string]any{
			"filesChanged": 0, "pointsChanged": 0,
			"errors": []string{}, "backups": s.listBackups(), "skipped": true,
		})
		return
	}

	excluded := map[string]bool{}
	for _, e := range body.Excluded {
		excluded[filepath.ToSlash(e)] = true
	}

	// 1) бекап (полный снимок папки, до изменений)
	send("phase", map[string]any{"phase": "backup"})
	info, err := s.createBackup(body.FolderPath)
	if err != nil {
		send("error", map[string]any{"error": "не удалось сделать бекап: " + err.Error()})
		return
	}
	send("backup", map[string]any{"id": info.ID, "datetime": info.DateTime})

	// 2) обработка только выбранных файлов
	send("phase", map[string]any{"phase": "process"})
	fc, pc, errs := processAll(body.FolderPath, factors, excluded, func(done, total int, file string) {
		send("progress", map[string]any{"done": done, "total": total, "file": file})
	})

	send("done", map[string]any{
		"filesChanged": fc, "pointsChanged": pc,
		"errors": errs, "backups": s.listBackups(),
	})
}

// GET /api/files?folderPath=...  - список .jbeam с размерами (для панели выбора)
type FileEntry struct {
	File string `json:"file"`
	Size int64  `json:"size"`
}

func (s *server) handleFiles(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folderPath")
	if folder == "" || !dirExists(folder) {
		writeJSON(w, map[string]any{"ok": false, "error": "папка не найдена"})
		return
	}
	files := []FileEntry{}
	for _, p := range collectJbeam(folder) {
		var size int64
		if fi, err := os.Stat(p); err == nil {
			size = fi.Size()
		}
		files = append(files, FileEntry{File: relSlash(folder, p), Size: size})
	}
	writeJSON(w, map[string]any{"ok": true, "files": files, "total": len(files)})
}

// GET /api/scan?folderPath=...&param=beamSpring  - текущие значения параметра
func (s *server) handleScan(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folderPath")
	param := r.URL.Query().Get("param")
	if param == "" || !isParam(param) {
		param = "nodeWeight"
	}
	if folder == "" || !dirExists(folder) {
		writeJSON(w, map[string]any{"ok": false, "error": "папка не найдена"})
		return
	}
	files, total := scanParam(folder, param)
	writeJSON(w, map[string]any{
		"ok":          true,
		"param":       param,
		"files":       files,
		"totalPoints": total,
		"totalFiles":  len(files),
	})
}
