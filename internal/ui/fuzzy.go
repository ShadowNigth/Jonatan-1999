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

type fuzzyQuery struct {
	lower string
	runes []rune
}

func compileFuzzy(pattern string) fuzzyQuery {
	lower := strings.ToLower(pattern)
	return fuzzyQuery{lower: lower, runes: []rune(lower)}
}

// Fuzzy comprueba si pattern es una subsecuencia de target (sin distinguir
// mayúsculas) y le asigna una puntuación: más alta = mejor coincidencia.
//
// Bonificaciones: coincidencia al inicio, tras separador y caracteres
// consecutivos. Penalización: huecos entre caracteres.
func Fuzzy(pattern, target string) (MatchResult, bool) {
	return compileFuzzy(pattern).match(target, true)
}

// match comparte el patrón ya normalizado entre muchos candidatos. FilterItems
// no necesita posiciones, por lo que evita crear una lista por cada campo.
func (q fuzzyQuery) match(target string, keepPositions bool) (MatchResult, bool) {
	if len(q.runes) == 0 {
		return MatchResult{Score: 0}, true
	}
	t := []rune(target)
	var pos []int
	if keepPositions {
		pos = make([]int, 0, len(q.runes))
	}
	score := 0
	pi := 0
	lastMatch := -2
	for i, r := range t {
		if unicode.ToLower(r) != q.runes[pi] {
			continue
		}
		if keepPositions {
			pos = append(pos, i)
		}
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
		lastMatch = i
		pi++
		if pi == len(q.runes) {
			break
		}
	}
	if pi < len(q.runes) {
		return MatchResult{}, false
	}
	// Premio por coincidencia exacta de substring y por cadenas cortas.
	lowTarget := strings.ToLower(target)
	if lowTarget == q.lower {
		score += 200
	} else if strings.HasPrefix(lowTarget, q.lower) {
		score += 80
	} else if strings.Contains(lowTarget, q.lower) {
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
	query := compileFuzzy(filter)
	var out []scored
	for i, item := range items {
		best := 0
		matched := false
		for _, field := range fields(item) {
			if match, ok := query.match(field, false); ok {
				matched = true
				if match.Score > best {
					best = match.Score
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
