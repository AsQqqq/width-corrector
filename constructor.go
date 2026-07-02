package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GET/POST /api/constructor/draft — авто-черновик конструктора (configs/constructor_draft.json)
func (s *server) handleConstructorDraft(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(s.configsDir, "constructor_draft.json")
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	// GET — отдаём сохранённый конфиг как есть, или null
	data, err := os.ReadFile(path)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		w.Write([]byte("null"))
		return
	}
	w.Write(data)
}

// ---------- конфиг движка из конструктора ----------

type CNode struct {
	Name   string  `json:"name"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
	Weight float64 `json:"weight"`
}

type EngineConfig struct {
	InternalName string             `json:"internalName"`
	Name         string             `json:"name"`
	Authors      string             `json:"authors"`
	Value        float64            `json:"value"`
	SlotType     string             `json:"slotType"`
	UIName       string             `json:"uiName"`
	NodeGroup    string             `json:"nodeGroup"`
	Slots        [][]string         `json:"slots"`   // [type, default, description]
	Torque       [][2]float64       `json:"torque"`  // [[rpm,torque],...]
	Params       map[string]float64 `json:"params"`  // скалярные параметры mainEngine
	HasRevLimiter      bool         `json:"hasRevLimiter"`
	BurnEffMode  string             `json:"burnEffMode"` // "single" | "curve"
	BurnEffSingle float64           `json:"burnEffSingle"`
	BurnEffCurve [][2]float64       `json:"burnEffCurve"`
	EnergyStorage      string       `json:"energyStorage"`
	RequiredEnergyType string       `json:"requiredEnergyType"`
	ThermalsEnabled     bool        `json:"thermalsEnabled"`
	EngineBlockMaterial string      `json:"engineBlockMaterial"`
	Sounds       map[string]string  `json:"sounds"` // soundConfig/Exhaust sample имена и т.п.
	TorqueReactionNodes []string    `json:"torqueReactionNodes"`
	DeformGroups        []string    `json:"deformGroups"`
	BreakTriggerBeam    string      `json:"breakTriggerBeam"`
	Flexbodies   [][]string         `json:"flexbodies"` // [mesh, group]
	Nodes        []CNode            `json:"nodes"`
}

// порядок и формат вывода скалярных параметров mainEngine
var engineParamOrder = []string{
	"idleRPM", "maxRPM", "revLimiterRPM", "maxPhysicalRPM",
	"inertia", "friction", "dynamicFriction", "engineBrakeTorque",
	"idleRPMRoughness", "particulates", "oilVolume",
	"maxTorqueRating", "maxOverTorqueDamage", "maxOverRevDamage",
	"cylinderWallTemperatureDamageThreshold", "headGasketDamageThreshold",
	"pistonRingDamageThreshold", "connectingRodDamageThreshold",
	"starterTorque", "starterThrottleKillTime", "idleRPMStartRate", "idleRPMStartCoef",
}

func qs(s string) string { return `"` + s + `"` }

func joinQuoted(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = qs(s)
	}
	return strings.Join(parts, ",")
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func def(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// генерирует ПОЛНЫЙ jbeam движка (структура нод/балок — проверенный шаблон)
func genEngineJbeam(c EngineConfig) string {
	if c.NodeGroup == "" {
		c.NodeGroup = "2107nsd_engine"
	}
	var b strings.Builder
	w := func(s string) { b.WriteString(s) }

	w("{\n\n")
	w(qs(def(c.InternalName, "custom_engine")) + ": {\n")

	// information
	w("    \"information\":{\n")
	w("        \"authors\":" + qs(def(c.Authors, "NSDmods")) + ",\n")
	w("        \"name\":" + qs(def(c.Name, "Custom Engine")) + ",\n")
	w("        \"value\":" + formatNum(c.Value) + ",\n")
	w("    },\n")
	w("    \"slotType\":" + qs(def(c.SlotType, "2107nsd_engine")) + ",\n")

	// slots
	w("    \"slots\": [\n")
	w("        [\"type\", \"default\", \"description\"],\n")
	for _, s := range c.Slots {
		if len(s) < 3 || strings.TrimSpace(s[0]) == "" {
			continue
		}
		w("        [" + qs(s[0]) + ", " + qs(s[1]) + ", " + qs(s[2]) + "],\n")
	}
	w("    ],\n")

	// powertrain
	w("    \"powertrain\": [\n")
	w("        [\"type\", \"name\", \"inputName\", \"inputIndex\"],\n")
	w("        [\"combustionEngine\", \"mainEngine\", \"dummy\", 0],\n")
	w("    ],\n")

	// mainEngine
	w("    \"mainEngine\": {\n")
	// torque
	w("        \"torque\":[\n")
	w("            [\"rpm\", \"torque\"],\n")
	for _, p := range c.Torque {
		w("            [" + formatNum(p[0]) + ", " + formatNum(p[1]) + "],\n")
	}
	w("        ],\n")
	// scalar params
	for _, k := range engineParamOrder {
		if v, ok := c.Params[k]; ok {
			w("        " + qs(k) + ":" + formatNum(v) + ",\n")
		}
	}
	w("        \"hasRevLimiter\":" + boolStr(c.HasRevLimiter) + ",\n")
	// burn efficiency
	if c.BurnEffMode == "curve" && len(c.BurnEffCurve) > 0 {
		w("        \"burnEfficiency\":[\n")
		for _, p := range c.BurnEffCurve {
			w("            [" + formatNum(p[0]) + ", " + formatNum(p[1]) + "],\n")
		}
		w("        ],\n")
	} else {
		w("        \"burnEfficiency\":" + formatNum(c.BurnEffSingle) + ",\n")
	}
	// fuel
	w("        \"energyStorage\":" + qs(def(c.EnergyStorage, "mainTank")) + ",\n")
	w("        \"requiredEnergyType\":" + qs(def(c.RequiredEnergyType, "gasoline")) + ",\n")
	// thermals
	w("        \"thermalsEnabled\":" + boolStr(c.ThermalsEnabled) + ",\n")
	w("        \"engineBlockMaterial\":" + qs(def(c.EngineBlockMaterial, "iron")) + ",\n")
	// sounds
	if c.Sounds != nil {
		if v := c.Sounds["soundConfig"]; v != "" {
			w("        \"soundConfig\":" + qs(v) + ",\n")
		}
		if v := c.Sounds["soundConfigExhaust"]; v != "" {
			w("        \"soundConfigExhaust\":" + qs(v) + ",\n")
		}
		if v := c.Sounds["starterSample"]; v != "" {
			w("        \"starterSample\":" + qs(v) + ",\n")
		}
	}
	// node-beam interface
	trn := c.TorqueReactionNodes
	if len(trn) == 0 {
		trn = []string{"e1l", "e2l", "e4r"}
	}
	w("        \"torqueReactionNodes:\":[" + joinQuoted(trn) + "],\n")
	w("        \"engineBlock\": {\"[engineGroup]:\":[\"engine_block\"]},\n")
	w("        \"breakTriggerBeam\":" + qs(def(c.BreakTriggerBeam, "engine")) + ",\n")
	w("        \"uiName\":" + qs(def(c.UIName, "Engine")) + ",\n")
	dg := c.DeformGroups
	if len(dg) == 0 {
		dg = []string{"mainEngine", "mainEngine_intake", "mainEngine_accessories"}
	}
	w("        \"deformGroups\":[" + joinQuoted(dg) + "],\n")
	w("    },\n")

	// flexbodies (привязка моделей)
	w("    \"flexbodies\": [\n")
	w("        [\"mesh\", \"[group]:\", \"nonFlexMaterials\"],\n")
	for _, f := range c.Flexbodies {
		if len(f) < 1 || strings.TrimSpace(f[0]) == "" {
			continue
		}
		grp := c.NodeGroup
		if len(f) >= 2 && strings.TrimSpace(f[1]) != "" {
			grp = f[1]
		}
		w("        [" + qs(f[0]) + ", [" + qs(grp) + "]],\n")
	}
	w("    ],\n")

	// nodes
	w("    \"nodes\": [\n")
	w("        [\"id\",   \"posX\",  \"posY\",  \"posZ\"],\n")
	w("        {\"frictionCoef\":0.5},\n")
	w("        {\"nodeMaterial\":\"|NM_METAL\"},\n")
	w("        {\"selfCollision\":false},\n")
	w("        {\"collision\":true},\n")
	w("        {\"engineGroup\":\"engine_block\"},\n")
	w("        {\"group\":" + qs(c.NodeGroup) + "},\n")
	nodes := c.Nodes
	if len(nodes) == 0 {
		nodes = defaultEngineNodes()
	}
	for _, n := range nodes {
		w("        {\"nodeWeight\":" + formatNum(n.Weight) + "},\n")
		w("        [" + qs(n.Name) + ", " + formatNum(n.X) + ", " + formatNum(n.Y) + ", " + formatNum(n.Z) + "],\n")
	}
	w("    ],\n")

	// beams + triangles — проверенный шаблон (структура движка)
	w(engineBeamsTemplate)

	w("},\n\n}\n")
	return b.String()
}

// стандартный набор узлов движка (8 + 2 крепления)
func defaultEngineNodes() []CNode {
	return []CNode{
		{"e1r", -0.17, -0.90, 0.29, 16},
		{"e1l", 0.17, -0.90, 0.29, 16},
		{"e2r", -0.17, -1.58, 0.38, 16},
		{"e2l", 0.17, -1.58, 0.38, 16},
		{"e3r", -0.13, -0.93, 0.88, 9},
		{"e3l", 0.13, -0.93, 0.88, 9},
		{"e4r", -0.13, -1.58, 0.88, 9},
		{"e4l", 0.13, -1.58, 0.88, 9},
		{"em1r", -0.23, -1.22, 0.60, 2},
		{"em1l", 0.23, -1.22, 0.60, 2},
	}
}

// проверенный шаблон балок/треугольников (имена узлов e1..e4 + em1)
const engineBeamsTemplate = `    "beams": [
          ["id1:", "id2:"],
          {"beamType":"|NORMAL", "beamLongBound":1.0, "beamShortBound":1.0},
          {"beamSpring":15001000,"beamDamp":400},
          {"beamDeform":175000,"beamStrength":"FLT_MAX"},
             {"deformGroup":"mainEngine", "deformationTriggerRatio":0.001}
             ["e1r",  "e1l"],
             ["e2r",  "e2l"],
             ["e3r",  "e3l"],
             ["e4r",  "e4l"],
             ["e1r",  "e2r"],
             ["e1l",  "e2l"],
             ["e3r",  "e4r"],
             ["e3l",  "e4l"],
             ["e1r",  "e3r"],
             ["e1l",  "e3l"],
             ["e2r",  "e4r"],
             ["e2l",  "e4l"],
             ["e2r",  "e3r"],
             ["e2l",  "e3l"],
             ["e2r",  "e3l"],
             ["e2l",  "e3r"],
             ["e1r",  "e4r",  {"isExhaust":"mainEngine"}],
             ["e1l",  "e4l"],
             ["e1r",  "e4l"],
             ["e1l",  "e4r"],
             ["e1r",  "e2l"],
             ["e1l",  "e2r"],
             ["e3r",  "e4l"],
             ["e3l",  "e4r"],
             ["e1r",  "e3l"],
             ["e1l",  "e3r"],
             ["e2r",  "e4l"],
             ["e2l",  "e4r"],
             {"beamSpring":3400000,"beamDamp":150},
             {"beamDeform":90000,"beamStrength":"FLT_MAX"},
          ["em1r", "e3l"],
          ["em1r", "e3r"],
          ["em1r", "e4l"],
          ["em1r", "e4r"],
          ["em1r", "e1r"],
          ["em1r", "e1l"],
          ["em1r", "e2l"],
          ["em1r", "e2r"],
          ["em1l", "e3l"],
          ["em1l", "e3r"],
          ["em1l", "e4l"],
          ["em1l", "e4r"],
          ["em1l", "e1r"],
          ["em1l", "e1l"],
          ["em1l", "e2l"],
          ["em1l", "e2r"],
          {"deformGroup":""},
          {"beamPrecompression":1, "beamType":"|NORMAL", "beamLongBound":1.0, "beamShortBound":1.0},
    ],
    "triangles": [
            ["id1:", "id2:", "id3:"],
            {"groundModel":"metal"},
            {"triangleType":"NONCOLLIDABLE"},
            ["e2l",  "e2r",  "e1r"],
            ["e1r",  "e1l",  "e2l"],
            {"triangleType":"NORMALTYPE"},
    ],
`

// ---------- HTTP ----------

// POST /api/engine/preview — body: EngineConfig → текст jbeam (без сохранения)
func (s *server) handleEnginePreview(w http.ResponseWriter, r *http.Request) {
	var cfg EngineConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad json"})
		return
	}
	text := genEngineJbeam(cfg)
	engines := detectEngines(text)
	writeJSON(w, map[string]any{
		"ok":       true,
		"text":     text,
		"valid":    len(engines) > 0,
		"engines":  engines,
	})
}

// ---------- обратный разбор: код jbeam → конфиг (для редактора) ----------

func parseSlotsRows(v interface{}) [][]string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := [][]string{}
	for _, row := range arr {
		r, ok := row.([]interface{})
		if !ok || len(r) < 1 {
			continue
		}
		a, ok := r[0].(string)
		if !ok || a == "type" {
			continue
		}
		b, c := "", ""
		if len(r) >= 2 {
			b, _ = r[1].(string)
		}
		if len(r) >= 3 {
			c, _ = r[2].(string)
		}
		out = append(out, []string{a, b, c})
	}
	return out
}

func parseFlexRows(v interface{}) [][]string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := [][]string{}
	for _, row := range arr {
		r, ok := row.([]interface{})
		if !ok || len(r) < 1 {
			continue
		}
		m, ok := r[0].(string)
		if !ok || m == "mesh" {
			continue
		}
		grp := ""
		if len(r) >= 2 {
			if g, ok := r[1].([]interface{}); ok && len(g) > 0 {
				grp, _ = g[0].(string)
			}
		}
		out = append(out, []string{m, grp})
	}
	return out
}

func parseNodeRows(v interface{}) []CNode {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := []CNode{}
	wt := 0.0
	for _, row := range arr {
		switch r := row.(type) {
		case map[string]interface{}:
			if w, ok := toF(r["nodeWeight"]); ok {
				wt = w
			}
		case []interface{}:
			if len(r) < 4 {
				continue
			}
			name, ok := r[0].(string)
			if !ok {
				continue
			}
			x, ok1 := toF(r[1])
			y, ok2 := toF(r[2])
			z, ok3 := toF(r[3])
			if !ok1 || !ok2 || !ok3 {
				continue
			}
			out = append(out, CNode{Name: name, X: x, Y: y, Z: z, Weight: wt})
		}
	}
	return out
}

func findNodeGroup(v interface{}) string {
	arr, ok := v.([]interface{})
	if !ok {
		return ""
	}
	for _, row := range arr {
		if m, ok := row.(map[string]interface{}); ok {
			if g, ok := m["group"].(string); ok && g != "" {
				return g
			}
		}
	}
	return ""
}

// parseEngineConfig разбирает первый движок из текста jbeam в EngineConfig
func parseEngineConfig(text string) (*EngineConfig, bool) {
	root, err := parseSJSON(text)
	if err != nil {
		return nil, false
	}
	rm, ok := root.(map[string]interface{})
	if !ok {
		return nil, false
	}
	var names []string
	for k := range rm {
		names = append(names, k)
	}
	sort.Strings(names)

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
			continue
		}
		cfg := &EngineConfig{
			InternalName: part,
			Params:       map[string]float64{},
			Sounds:       map[string]string{},
			Torque:       curve,
		}
		if inf, ok := pm["information"].(map[string]interface{}); ok {
			if v, ok := inf["name"].(string); ok {
				cfg.Name = v
			}
			if v, ok := inf["authors"].(string); ok {
				cfg.Authors = v
			}
			if v, ok := toF(inf["value"]); ok {
				cfg.Value = v
			}
		}
		if v, ok := pm["slotType"].(string); ok {
			cfg.SlotType = v
		}
		if v, ok := me["uiName"].(string); ok {
			cfg.UIName = v
		}
		for _, k := range engineParamOrder {
			if v, ok := toF(me[k]); ok {
				cfg.Params[k] = v
			}
		}
		if v, ok := me["hasRevLimiter"].(bool); ok {
			cfg.HasRevLimiter = v
		}
		if f, ok := toF(me["burnEfficiency"]); ok {
			cfg.BurnEffMode = "single"
			cfg.BurnEffSingle = f
		} else if c := parseTorqueCurve(me["burnEfficiency"]); len(c) > 0 {
			cfg.BurnEffMode = "curve"
			cfg.BurnEffCurve = c
		} else {
			cfg.BurnEffMode = "single"
			cfg.BurnEffSingle = 1
		}
		if v, ok := me["energyStorage"].(string); ok {
			cfg.EnergyStorage = v
		}
		if v, ok := me["requiredEnergyType"].(string); ok {
			cfg.RequiredEnergyType = v
		}
		if v, ok := me["thermalsEnabled"].(bool); ok {
			cfg.ThermalsEnabled = v
		}
		if v, ok := me["engineBlockMaterial"].(string); ok {
			cfg.EngineBlockMaterial = v
		}
		cfg.Slots = parseSlotsRows(pm["slots"])
		cfg.Flexbodies = parseFlexRows(pm["flexbodies"])
		cfg.Nodes = parseNodeRows(pm["nodes"])
		cfg.NodeGroup = findNodeGroup(pm["nodes"])
		return cfg, true
	}
	return nil, false
}

// POST /api/engine/parse — body {text} → конфиг (код редактора → форма)
func (s *server) handleEngineParse(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad json"})
		return
	}
	cfg, valid := parseEngineConfig(body.Text)
	writeJSON(w, map[string]any{"ok": true, "valid": valid, "config": cfg})
}

// POST /api/engine/create — body: {folderPath, fileName, config}
func (s *server) handleEngineCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FolderPath string       `json:"folderPath"`
		FileName   string       `json:"fileName"`
		Config     EngineConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad json"})
		return
	}
	if body.FolderPath == "" || !dirExists(body.FolderPath) {
		writeJSON(w, map[string]any{"ok": false, "error": "папка не выбрана"})
		return
	}
	name := strings.TrimSpace(body.FileName)
	if name == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "укажите имя файла"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(name), ".jbeam") {
		name += ".jbeam"
	}
	// только имя файла, без путей вверх
	name = filepath.Base(name)

	text := genEngineJbeam(body.Config)
	// предохранитель: сгенерированное должно распознаваться как мотор
	if len(detectEngines(text)) == 0 {
		writeJSON(w, map[string]any{"ok": false, "error": "сгенерированный файл не распознаётся как мотор — проверьте кривую момента и параметры"})
		return
	}

	dest := filepath.Join(filepath.Clean(body.FolderPath), name)
	if _, err := os.Stat(dest); err == nil {
		// файл уже есть — бекапим папку перед перезаписью
		if _, err := s.createBackup(body.FolderPath); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "бекап перед перезаписью не удался: " + err.Error()})
			return
		}
	}
	if err := os.WriteFile(dest, []byte(text), 0o644); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "file": name, "backups": s.listBackups()})
}
