package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// редактируемые скалярные параметры мотора (порядок/подписи — на фронте)
var engineScalarKeys = []string{
	"idleRPM", "maxRPM", "revLimiterRPM", "inertia", "friction", "dynamicFriction",
	"engineBrakeTorque", "maxTorqueRating", "maxOverTorqueDamage",
	"cylinderWallTemperatureDamageThreshold", "headGasketDamageThreshold",
	"pistonRingDamageThreshold", "connectingRodDamageThreshold",
}

type EngineInfo struct {
	Part          string             `json:"part"`
	Name          string             `json:"name"`
	Params        map[string]float64 `json:"params"`
	HasRevLimiter *bool              `json:"hasRevLimiter,omitempty"`
	TorqueCurve   [][2]float64       `json:"torqueCurve"`
	PeakTorque    float64            `json:"peakTorque"`
	PeakTorqueRPM float64            `json:"peakTorqueRpm"`
	PeakPowerHP   float64            `json:"peakPowerHp"`
	PeakPowerRPM  float64            `json:"peakPowerRpm"`
}

func toF(v interface{}) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

// кривая момента: [[rpm,torque],...] (заголовок ["rpm","torque"] отсеивается)
func parseTorqueCurve(v interface{}) [][2]float64 {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out [][2]float64
	for _, row := range arr {
		r, ok := row.([]interface{})
		if !ok || len(r) < 2 {
			continue
		}
		rpm, ok1 := toF(r[0])
		tq, ok2 := toF(r[1])
		if !ok1 || !ok2 {
			continue
		}
		out = append(out, [2]float64{rpm, tq})
	}
	return out
}

// detectEngines: каждая деталь (top-level ключ) с mainEngine, содержащим кривую torque
func detectEngines(text string) []EngineInfo {
	root, err := parseSJSON(text)
	if err != nil {
		return nil
	}
	rm, ok := root.(map[string]interface{})
	if !ok {
		return nil
	}
	var names []string
	for k := range rm {
		names = append(names, k)
	}
	sort.Strings(names)

	var engines []EngineInfo
	for _, part := range names {
		pm, ok := rm[part].(map[string]interface{})
		if !ok {
			continue
		}
		me, ok := pm["mainEngine"].(map[string]interface{})
		if !ok {
			continue
		}
		curve := parseTorqueCurve(me["torque"])
		if len(curve) == 0 {
			continue // без кривой момента — не считаем мотором
		}
		info := EngineInfo{Part: part, Params: map[string]float64{}, TorqueCurve: curve}
		if inf, ok := pm["information"].(map[string]interface{}); ok {
			if n, ok := inf["name"].(string); ok {
				info.Name = n
			}
		}
		if info.Name == "" {
			info.Name = part
		}
		for _, k := range engineScalarKeys {
			if f, ok := toF(me[k]); ok {
				info.Params[k] = f
			}
		}
		if b, ok := me["hasRevLimiter"].(bool); ok {
			info.HasRevLimiter = &b
		}
		// потолок рабочих оборотов: отсечка, иначе maxRPM
		maxOper := info.Params["revLimiterRPM"]
		if maxOper == 0 {
			maxOper = info.Params["maxRPM"]
		}
		for _, p := range curve {
			rpm, tq := p[0], p[1]
			if maxOper > 0 && rpm > maxOper {
				continue // выше рабочего диапазона
			}
			if tq > info.PeakTorque {
				info.PeakTorque = tq
				info.PeakTorqueRPM = rpm
			}
			hp := tq * rpm / 9549.3 * 1.34102
			if hp > info.PeakPowerHP {
				info.PeakPowerHP = hp
				info.PeakPowerRPM = rpm
			}
		}
		engines = append(engines, info)
	}
	return engines
}

// ---------- запись параметров обратно ----------

// objectSpan: индексы [start,end) объекта { ... }, первый '{' которого — на/после pos.
// Корректно пропускает строки и комментарии // и /* */.
func objectSpan(s string, pos int) (int, int, bool) {
	i := pos
	for i < len(s) && s[i] != '{' {
		i++
	}
	if i >= len(s) {
		return 0, 0, false
	}
	depth := 0
	inStr := false
	esc := false
	for j := i; j < len(s); j++ {
		c := s[j]
		if inStr {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			continue
		}
		if c == '/' && j+1 < len(s) {
			if s[j+1] == '/' {
				for j < len(s) && s[j] != '\n' {
					j++
				}
				continue
			}
			if s[j+1] == '*' {
				j += 2
				for j+1 < len(s) && !(s[j] == '*' && s[j+1] == '/') {
					j++
				}
				j++
				continue
			}
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return i, j + 1, true
			}
		}
	}
	return 0, 0, false
}

// setEngineParams заменяет скалярные параметры в блоке mainEngine выбранной детали
func setEngineParams(text, part string, params map[string]float64) (string, []string, error) {
	reKey := regexp.MustCompile(`"` + regexp.QuoteMeta(part) + `"\s*:`)
	loc := reKey.FindStringIndex(text)
	if loc == nil {
		return text, nil, fmt.Errorf("деталь не найдена в файле")
	}
	ps, pe, ok := objectSpan(text, loc[1])
	if !ok {
		return text, nil, fmt.Errorf("не удалось разобрать блок детали")
	}
	partText := text[ps:pe]

	reME := regexp.MustCompile(`"mainEngine"\s*:`)
	mloc := reME.FindStringIndex(partText)
	if mloc == nil {
		return text, nil, fmt.Errorf("mainEngine не найден")
	}
	ms, meEnd, ok := objectSpan(partText, mloc[1])
	if !ok {
		return text, nil, fmt.Errorf("не удалось разобрать mainEngine")
	}
	meText := partText[ms:meEnd]

	applied := []string{}
	for key, val := range params {
		re := regexp.MustCompile(`("` + regexp.QuoteMeta(key) + `"\s*:\s*)(-?\d+(?:\.\d+)?)`)
		if re.MatchString(meText) {
			meText = re.ReplaceAllString(meText, "${1}"+formatNum(val))
			applied = append(applied, key)
		}
	}
	newPart := partText[:ms] + meText + partText[meEnd:]
	return text[:ps] + newPart + text[pe:], applied, nil
}

// ---------- HTTP ----------

func (s *server) safeJoin(folder, rel string) (string, bool) {
	if folder == "" || !dirExists(folder) {
		return "", false
	}
	base := filepath.Clean(folder)
	clean := filepath.Clean(filepath.Join(base, filepath.FromSlash(rel)))
	if clean != base && !strings.HasPrefix(clean, base+string(os.PathSeparator)) {
		return "", false
	}
	return clean, true
}

// GET /api/engine?folderPath=...&file=rel  — определить мотор(ы) и параметры
func (s *server) handleEngine(w http.ResponseWriter, r *http.Request) {
	clean, ok := s.safeJoin(r.URL.Query().Get("folderPath"), r.URL.Query().Get("file"))
	if !ok {
		writeJSON(w, map[string]any{"ok": false, "error": "недопустимый путь"})
		return
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	engines := detectEngines(string(data))
	stages := detectStages(string(data))
	writeJSON(w, map[string]any{"ok": true, "isEngine": len(engines) > 0, "engines": engines, "stages": stages})
}

// ---------- стейджи (тюнинг через torqueMod*) ----------

var stageModKeys = map[string]string{
	"torqueModIntake":  "intake",
	"torqueModExhaust": "exhaust",
	"torqueModMult":    "mult",
}

type StageInfo struct {
	Part    string       `json:"part"`
	Name    string       `json:"name"`
	ModType string       `json:"modType"`
	ModKey  string       `json:"modKey"`
	Curve   [][2]float64 `json:"curve"`
}

func detectStages(text string) []StageInfo {
	root, err := parseSJSON(text)
	if err != nil {
		return nil
	}
	rm, ok := root.(map[string]interface{})
	if !ok {
		return nil
	}
	var names []string
	for k := range rm {
		names = append(names, k)
	}
	sort.Strings(names)

	out := []StageInfo{}
	for _, part := range names {
		pm, ok := rm[part].(map[string]interface{})
		if !ok {
			continue
		}
		me, ok := pm["mainEngine"].(map[string]interface{})
		if !ok {
			continue
		}
		name := part
		if inf, ok := pm["information"].(map[string]interface{}); ok {
			if n, ok := inf["name"].(string); ok && n != "" {
				name = n
			}
		}
		// стабильный порядок типов
		for _, key := range []string{"torqueModIntake", "torqueModExhaust", "torqueModMult"} {
			curve := parseTorqueCurve(me[key])
			if len(curve) == 0 {
				continue
			}
			out = append(out, StageInfo{Part: part, Name: name, ModType: stageModKeys[key], ModKey: key, Curve: curve})
		}
	}
	return out
}

// arraySpan: индексы [start,end) массива [ ... ], первый '[' которого на/после pos
func arraySpan(s string, pos int) (int, int, bool) {
	i := pos
	for i < len(s) && s[i] != '[' {
		i++
	}
	if i >= len(s) {
		return 0, 0, false
	}
	depth := 0
	inStr := false
	esc := false
	for j := i; j < len(s); j++ {
		c := s[j]
		if inStr {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			continue
		}
		if c == '/' && j+1 < len(s) {
			if s[j+1] == '/' {
				for j < len(s) && s[j] != '\n' {
					j++
				}
				continue
			}
			if s[j+1] == '*' {
				j += 2
				for j+1 < len(s) && !(s[j] == '*' && s[j+1] == '/') {
					j++
				}
				j++
				continue
			}
		}
		if c == '[' {
			depth++
		} else if c == ']' {
			depth--
			if depth == 0 {
				return i, j + 1, true
			}
		}
	}
	return 0, 0, false
}

func genCurveArray(curve [][2]float64) string {
	var b strings.Builder
	b.WriteString("[\n")
	b.WriteString("            [\"rpm\", \"torque\"],\n")
	for _, p := range curve {
		b.WriteString("            [" + formatNum(p[0]) + ", " + formatNum(p[1]) + "],\n")
	}
	b.WriteString("        ]")
	return b.String()
}

// setStageCurve заменяет массив torqueMod* в mainEngine выбранной детали
func setStageCurve(text, part, modKey string, curve [][2]float64) (string, error) {
	reKey := regexp.MustCompile(`"` + regexp.QuoteMeta(part) + `"\s*:`)
	loc := reKey.FindStringIndex(text)
	if loc == nil {
		return text, fmt.Errorf("деталь не найдена в файле")
	}
	ps, pe, ok := objectSpan(text, loc[1])
	if !ok {
		return text, fmt.Errorf("не удалось разобрать блок детали")
	}
	partText := text[ps:pe]

	reME := regexp.MustCompile(`"mainEngine"\s*:`)
	mloc := reME.FindStringIndex(partText)
	if mloc == nil {
		return text, fmt.Errorf("mainEngine не найден")
	}
	ms, meEnd, ok := objectSpan(partText, mloc[1])
	if !ok {
		return text, fmt.Errorf("не удалось разобрать mainEngine")
	}
	meText := partText[ms:meEnd]

	reMod := regexp.MustCompile(`"` + regexp.QuoteMeta(modKey) + `"\s*:`)
	modloc := reMod.FindStringIndex(meText)
	if modloc == nil {
		return text, fmt.Errorf(modKey + " не найден")
	}
	as, ae, ok := arraySpan(meText, modloc[1])
	if !ok {
		return text, fmt.Errorf("не удалось разобрать кривую")
	}
	newME := meText[:as] + genCurveArray(curve) + meText[ae:]
	newPart := partText[:ms] + newME + partText[meEnd:]
	return text[:ps] + newPart + text[pe:], nil
}

// POST /api/engine/stage — body {folderPath, file, part, modKey, curve}  (бекап → запись кривой)
func (s *server) handleStageApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FolderPath string       `json:"folderPath"`
		File       string       `json:"file"`
		Part       string       `json:"part"`
		ModKey     string       `json:"modKey"`
		Curve      [][2]float64 `json:"curve"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad json"})
		return
	}
	if _, ok := stageModKeys[body.ModKey]; !ok {
		writeJSON(w, map[string]any{"ok": false, "error": "неизвестный тип тюнинга"})
		return
	}
	clean, ok := s.safeJoin(body.FolderPath, body.File)
	if !ok {
		writeJSON(w, map[string]any{"ok": false, "error": "недопустимый путь"})
		return
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if _, err := s.createBackup(body.FolderPath); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "не удалось сделать бекап: " + err.Error()})
		return
	}
	newText, err := setStageCurve(string(data), body.Part, body.ModKey, body.Curve)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	mode := os.FileMode(0o644)
	if fi, e := os.Stat(clean); e == nil {
		mode = fi.Mode()
	}
	if err := os.WriteFile(clean, []byte(newText), mode); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "backups": s.listBackups()})
}

// POST /api/engine/apply  — body {folderPath, file, part, params}  (бекап → запись)
func (s *server) handleEngineApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FolderPath string             `json:"folderPath"`
		File       string             `json:"file"`
		Part       string             `json:"part"`
		Params     map[string]float64 `json:"params"`
		Torque     [][2]float64       `json:"torque"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad json"})
		return
	}
	clean, ok := s.safeJoin(body.FolderPath, body.File)
	if !ok {
		writeJSON(w, map[string]any{"ok": false, "error": "недопустимый путь"})
		return
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	// сначала бекап всей папки
	if _, err := s.createBackup(body.FolderPath); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "не удалось сделать бекап: " + err.Error()})
		return
	}
	newText, applied, err := setEngineParams(string(data), body.Part, body.Params)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	// базовая кривая момента (если прислана) — пишем тем же механизмом
	if len(body.Torque) > 0 {
		newText, err = setStageCurve(newText, body.Part, "torque", body.Torque)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "кривая момента: " + err.Error()})
			return
		}
		applied = append(applied, "torque")
	}
	mode := os.FileMode(0o644)
	if fi, e := os.Stat(clean); e == nil {
		mode = fi.Mode()
	}
	if err := os.WriteFile(clean, []byte(newText), mode); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "applied": applied, "backups": s.listBackups()})
}
