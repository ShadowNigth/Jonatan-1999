package ui

import (
	"sort"
	"strings"
	"unicode"
)

// MatchResult describe el resultado de un emparejamiento difuso.
type MatchResult struct {
	Score     int
	Positions []int
}

// Fuzzy comprueba si pattern es una subsecuencia de target (sin distinguir
// mayúsculas) y le asigna una puntuación: más alta = mejor coincidencia.
//
// Bonificaciones: coincidencia al inicio, tras separador y caracteres
// consecutivos. Penalización: huecos entre caracteres.
func Fuzzy(pattern, target string) (MatchResult, bool) {
	if pattern == "" {
		return MatchResult{Score: 0}, true
	}
	p := []rune(strings.ToLower(pattern))
	t := []rune(target)
	lower := make([]rune, len(t))
	for i, r := range t {
		lower[i] = unicode.ToLower(r)
	}

	pos := make([]int, 0, len(p))
	score := 0
	pi := 0
	lastMatch := -2
	for i := 0; i < len(t) && pi < len(p); i++ {
		if lower[i] != p[pi] {
			continue
		}
		pos = append(pos, i)
		score += 10
		if i == 0 {
			score += 20
		} else if isBoundary(t[i-1]) {
			score += 15
		}
		if i == lastMatch+1 {
			score += 12
		} else if lastMatch >= 0 {
			score -= (i - lastMatch - 1) * 2
		}
		if t[i] == rune(pattern[len(pattern)-1]) && pi == len(p)-1 {
			// pequeño premio por terminar en el último carácter
		}
		lastMatch = i
		pi++
	}
	if pi < len(p) {
		return MatchResult{}, false
	}
	// Premio por coincidencia exacta de substring y por cadenas cortas.
	lowTarget := string(lower)
	lowPat := strings.ToLower(pattern)
	if lowTarget == lowPat {
		score += 200
	} else if strings.HasPrefix(lowTarget, lowPat) {
		score += 80
	} else if strings.Contains(lowTarget, lowPat) {
		score += 40
	}
	score -= len(t) / 4
	return MatchResult{Score: score, Positions: pos}, true
}

func isBoundary(r rune) bool {
	switch r {
	case ' ', '-', '_', '/', '.', ':', '|', '@':
		return true
	}
	return false
}

// FilterItems puntúa y ordena una lista de entradas según el filtro.
// Devuelve los índices de los elementos que coinciden, ordenados.
func FilterItems[T any](items []T, filter string, fields func(T) []string) []int {
	type scored struct {
		idx   int
		score int
	}
	var out []scored
	for i, item := range items {
		best := 0
		matched := false
		for _, f := range fields(item) {
			if m, ok := Fuzzy(filter, f); ok {
				matched = true
				if m.Score > best {
					best = m.Score
				}
			}
		}
		if matched {
			out = append(out, scored{i, best})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].score > out[b].score })
	res := make([]int, len(out))
	for i, s := range out {
		res[i] = s.idx
	}
	return res
}
