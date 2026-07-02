package main

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ModelNode struct {
	Name       string      `json:"name"`
	X          float64     `json:"x"`
	Y          float64     `json:"y"`
	Z          float64     `json:"z"`
	NodeWeight interface{} `json:"nodeWeight,omitempty"`
	Part       string      `json:"part"`
}

type ModelBeam struct {
	A            int         `json:"a"`
	B            int         `json:"b"`
	BeamDeform   interface{} `json:"beamDeform,omitempty"`
	BeamSpring   interface{} `json:"beamSpring,omitempty"`
	BeamDamp     interface{} `json:"beamDamp,omitempty"`
	BeamStrength interface{} `json:"beamStrength,omitempty"`
	Part         string      `json:"part"`
}

type rawBeam struct {
	a, b                           string
	deform, spring, damp, strength interface{}
}

func asFloat(v interface{}) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

// inline-опции строки переопределяют контекст-модификатор
func pickVal(ctx, opts map[string]interface{}, key string) interface{} {
	if opts != nil {
		if v, ok := opts[key]; ok {
			return v
		}
	}
	return ctx[key]
}

func collectNodesInto(arr []interface{}, part string, nodes *[]ModelNode, idx map[string]int) {
	ctx := map[string]interface{}{}
	for _, row := range arr {
		switch r := row.(type) {
		case map[string]interface{}:
			for k, v := range r {
				ctx[k] = v
			}
		case []interface{}:
			if len(r) < 4 {
				continue
			}
			name, ok := r[0].(string)
			if !ok {
				continue
			}
			x, ok1 := asFloat(r[1])
			y, ok2 := asFloat(r[2])
			z, ok3 := asFloat(r[3])
			if !ok1 || !ok2 || !ok3 {
				continue // заголовок/нечисловые координаты
			}
			if _, exists := idx[name]; exists {
				continue
			}
			var opts map[string]interface{}
			if len(r) >= 5 {
				opts, _ = r[4].(map[string]interface{})
			}
			idx[name] = len(*nodes)
			*nodes = append(*nodes, ModelNode{
				Name: name, X: x, Y: y, Z: z,
				NodeWeight: pickVal(ctx, opts, "nodeWeight"),
				Part:       part,
			})
		}
	}
}

func collectBeamsInto(arr []interface{}, raws *[]rawBeam) {
	ctx := map[string]interface{}{}
	for _, row := range arr {
		switch r := row.(type) {
		case map[string]interface{}:
			for k, v := range r {
				ctx[k] = v
			}
		case []interface{}:
			if len(r) < 2 {
				continue
			}
			a, ok1 := r[0].(string)
			b, ok2 := r[1].(string)
			if !ok1 || !ok2 {
				continue
			}
			var opts map[string]interface{}
			if len(r) >= 3 {
				opts, _ = r[2].(map[string]interface{})
			}
			*raws = append(*raws, rawBeam{
				a: a, b: b,
				deform:   pickVal(ctx, opts, "beamDeform"),
				spring:   pickVal(ctx, opts, "beamSpring"),
				damp:     pickVal(ctx, opts, "beamDamp"),
				strength: pickVal(ctx, opts, "beamStrength"),
			})
		}
	}
}

// buildModel разбирает файл по ДЕТАЛЯМ (top-level ключи jbeam).
// Каждый узел/балка помечается именем своей детали; балки разрешаются в
// пределах своей детали (имена узлов между деталями могут совпадать).
func buildModel(text string) ([]ModelNode, []ModelBeam, []string, error) {
	root, e := parseSJSON(text)
	if e != nil {
		return nil, nil, nil, e
	}
	var nodes []ModelNode
	var beams []ModelBeam

	rm, ok := root.(map[string]interface{})
	if !ok {
		return nodes, beams, []string{}, nil
	}

	var names []string
	for k := range rm {
		names = append(names, k)
	}
	sort.Strings(names) // стабильный порядок деталей

	withGeom := map[string]bool{}
	for _, part := range names {
		idx := map[string]int{} // имя узла -> глобальный индекс, локально для детали
		var raws []rawBeam
		startN := len(nodes)

		var walk func(v interface{})
		walk = func(v interface{}) {
			switch t := v.(type) {
			case map[string]interface{}:
				for k, vv := range t {
					switch k {
					case "nodes":
						if arr, ok := vv.([]interface{}); ok {
							collectNodesInto(arr, part, &nodes, idx)
						}
					case "beams":
						if arr, ok := vv.([]interface{}); ok {
							collectBeamsInto(arr, &raws)
						}
					default:
						walk(vv)
					}
				}
			case []interface{}:
				for _, el := range t {
					walk(el)
				}
			}
		}
		walk(rm[part])

		for _, b := range raws {
			i, ok1 := idx[b.a]
			j, ok2 := idx[b.b]
			if !ok1 || !ok2 {
				continue
			}
			beams = append(beams, ModelBeam{
				A: i, B: j,
				BeamDeform: b.deform, BeamSpring: b.spring,
				BeamDamp: b.damp, BeamStrength: b.strength,
				Part: part,
			})
		}
		if len(nodes) > startN {
			withGeom[part] = true
		}
	}

	parts := []string{}
	for _, n := range names {
		if withGeom[n] {
			parts = append(parts, n)
		}
	}
	return nodes, beams, parts, nil
}

// GET /api/model?folderPath=...&file=<относит.путь>
func (s *server) handleModel(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folderPath")
	rel := r.URL.Query().Get("file")
	if folder == "" || !dirExists(folder) {
		writeJSON(w, map[string]any{"ok": false, "error": "папка не найдена"})
		return
	}
	base := filepath.Clean(folder)
	clean := filepath.Clean(filepath.Join(base, filepath.FromSlash(rel)))
	if clean != base && !strings.HasPrefix(clean, base+string(os.PathSeparator)) {
		writeJSON(w, map[string]any{"ok": false, "error": "недопустимый путь"})
		return
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	nodes, beams, parts, err := buildModel(string(data))
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "разбор файла: " + err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "nodes": nodes, "beams": beams, "parts": parts})
}
