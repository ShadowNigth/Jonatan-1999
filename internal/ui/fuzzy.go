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
	var pos []int
	if keepPositions {
		pos = make([]int, 0, len(q.runes))
	}
	score := 0
	pi := 0
	lastMatch := -2
	runeIndex := 0
	var previous rune
	for _, r := range target {
		r = unicode.ToLower(r)
		if pi < len(q.runes) && r == q.runes[pi] {
			if keepPositions {
				pos = append(pos, runeIndex)
			}
			score += 10
			if runeIndex == 0 {
				score += 20
			} else if isBoundary(previous) {
				score += 15
			}
			if runeIndex == lastMatch+1 {
				score += 12
			} else if lastMatch >= 0 {
				score -= (runeIndex - lastMatch - 1) * 2
			}
			lastMatch = runeIndex
			pi++
		}
		previous = r
		runeIndex++
	}
	if pi < len(q.runes) {
		return MatchResult{}, false
	}
	// Solo se normaliza el string completo cuando ya hubo una coincidencia.
	lowTarget := strings.ToLower(target)
	// Premio por coincidencia exacta de substring y por cadenas cortas.
	if lowTarget == q.lower {
		score += 200
	} else if strings.HasPrefix(lowTarget, q.lower) {
		score += 80
	} else if strings.Contains(lowTarget, q.lower) {
		score += 40
	}
	score -= runeIndex / 4
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
