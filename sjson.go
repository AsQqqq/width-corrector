package main

import (
	"errors"
	"strconv"
	"strings"
)

// Минимальный устойчивый парсер SJSON (формат BeamNG):
//   - комментарии // и /* */
//   - запятые между элементами/парами НЕОБЯЗАТЕЛЬНЫ (трактуем как пробел)
//   - незакавыченные ключи и слова (FLT_MAX и т.п.)
// Возвращает дерево из map[string]interface{}, []interface{}, string, float64, bool, nil.

type sjReader struct {
	s string
	i int
}

func parseSJSON(s string) (val interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("ошибка разбора SJSON")
		}
	}()
	r := &sjReader{s: s}
	r.ws()
	return r.value(), nil
}

// пропуск пробелов, запятых и комментариев
func (r *sjReader) ws() {
	for r.i < len(r.s) {
		c := r.s[r.i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' {
			r.i++
			continue
		}
		if c == '/' && r.i+1 < len(r.s) {
			if r.s[r.i+1] == '/' {
				r.i += 2
				for r.i < len(r.s) && r.s[r.i] != '\n' {
					r.i++
				}
				continue
			}
			if r.s[r.i+1] == '*' {
				r.i += 2
				for r.i+1 < len(r.s) && !(r.s[r.i] == '*' && r.s[r.i+1] == '/') {
					r.i++
				}
				r.i += 2
				continue
			}
		}
		break
	}
}

func (r *sjReader) value() interface{} {
	if r.i >= len(r.s) {
		panic("eof")
	}
	switch r.s[r.i] {
	case '{':
		return r.object()
	case '[':
		return r.array()
	case '"':
		return r.str()
	default:
		return r.token()
	}
}

func (r *sjReader) object() map[string]interface{} {
	r.i++ // {
	m := map[string]interface{}{}
	r.ws()
	for r.i < len(r.s) && r.s[r.i] != '}' {
		var key string
		if r.s[r.i] == '"' {
			key = r.str()
		} else {
			key = r.bareword()
		}
		r.ws()
		if r.i < len(r.s) && r.s[r.i] == ':' {
			r.i++
		}
		r.ws()
		m[key] = r.value()
		r.ws()
	}
	r.i++ // }
	return m
}

func (r *sjReader) array() []interface{} {
	r.i++ // [
	arr := []interface{}{}
	r.ws()
	for r.i < len(r.s) && r.s[r.i] != ']' {
		arr = append(arr, r.value())
		r.ws()
	}
	r.i++ // ]
	return arr
}

func (r *sjReader) str() string {
	r.i++ // открывающая "
	var b strings.Builder
	for r.i < len(r.s) {
		c := r.s[r.i]
		if c == '\\' && r.i+1 < len(r.s) {
			n := r.s[r.i+1]
			switch n {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(n)
			}
			r.i += 2
			continue
		}
		if c == '"' {
			r.i++
			break
		}
		b.WriteByte(c)
		r.i++
	}
	return b.String()
}

func isDelim(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', ',', '{', '}', '[', ']', ':', '"', '/':
		return true
	}
	return false
}

func (r *sjReader) bareword() string {
	start := r.i
	for r.i < len(r.s) && !isDelim(r.s[r.i]) {
		r.i++
	}
	if r.i == start { // защита от зацикливания на неожиданном символе
		r.i++
		return ""
	}
	return r.s[start:r.i]
}

func (r *sjReader) token() interface{} {
	w := r.bareword()
	switch w {
	case "true":
		return true
	case "false":
		return false
	case "null", "nil":
		return nil
	}
	if f, err := strconv.ParseFloat(w, 64); err == nil {
		return f
	}
	return w
}
