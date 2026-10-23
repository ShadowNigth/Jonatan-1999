// Package calc evalúa expresiones aritméticas sencillas para la calculadora
// integrada del lanzador.
package calc

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Eval evalúa una expresión y devuelve su resultado.
func Eval(input string) (float64, error) {
	p := &parser{src: []rune(strings.ReplaceAll(input, " ", ""))}
	if len(p.src) == 0 {
		return 0, fmt.Errorf("expresión vacía")
	}
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos < len(p.src) {
		return 0, fmt.Errorf("carácter inesperado %q", string(p.src[p.pos]))
	}
	return v, nil
}

// Format da formato legible al resultado (sin decimales innecesarios).
func Format(v float64) string {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return "indefinido"
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	s := strconv.FormatFloat(v, 'f', 6, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

type parser struct {
	src []rune
	pos int
}

func (p *parser) peek() rune {
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *parser) next() rune {
	r := p.peek()
	p.pos++
	return r
}

func (p *parser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '+':
			p.next()
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left += right
		case '-':
			p.next()
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left -= right
		default:
			return left, nil
		}
	}
}

func (p *parser) parseTerm() (float64, error) {
	left, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '*':
			p.next()
			if p.peek() == '*' {
				p.next()
				right, err := p.parseUnary()
				if err != nil {
					return 0, err
				}
				left = math.Pow(left, right)
				continue
			}
			right, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			left *= right
		case '/':
			p.next()
			right, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("división por cero")
			}
			left /= right
		case '%':
			p.next()
			right, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("módulo por cero")
			}
			left = math.Mod(left, right)
		default:
			return left, nil
		}
	}
}

func (p *parser) parseUnary() (float64, error) {
	switch p.peek() {
	case '-':
		p.next()
		v, err := p.parseUnary()
		return -v, err
	case '+':
		p.next()
		return p.parseUnary()
	}
	return p.parsePower()
}

func (p *parser) parsePower() (float64, error) {
	base, err := p.parsePrimary()
	if err != nil {
		return 0, err
	}
	if p.peek() == '^' {
		p.next()
		exp, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

func (p *parser) parsePrimary() (float64, error) {
	switch r := p.peek(); {
	case r == '(':
		p.next()
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("falta ')'")
		}
		p.next()
		return v, nil
	case unicode.IsDigit(r) || r == '.':
		return p.parseNumber()
	case unicode.IsLetter(r):
		return p.parseIdent()
	}
	return 0, fmt.Errorf("expresión incompleta")
}

func (p *parser) parseNumber() (float64, error) {
	start := p.pos
	for p.pos < len(p.src) && (unicode.IsDigit(p.src[p.pos]) || p.src[p.pos] == '.') {
		p.pos++
	}
	// Exponente: 1e9, 2.5e-3
	if p.pos < len(p.src) && (p.src[p.pos] == 'e' || p.src[p.pos] == 'E') {
		save := p.pos
		p.pos++
		if p.pos < len(p.src) && (p.src[p.pos] == '+' || p.src[p.pos] == '-') {
			p.pos++
		}
		if p.pos < len(p.src) && unicode.IsDigit(p.src[p.pos]) {
			for p.pos < len(p.src) && unicode.IsDigit(p.src[p.pos]) {
				p.pos++
			}
		} else {
			p.pos = save
		}
	}
	text := string(p.src[start:p.pos])
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("número inválido: %s", text)
	}
	return v, nil
}

func (p *parser) parseIdent() (float64, error) {
	start := p.pos
	for p.pos < len(p.src) && (unicode.IsLetter(p.src[p.pos]) || unicode.IsDigit(p.src[p.pos])) {
		p.pos++
	}
	name := strings.ToLower(string(p.src[start:p.pos]))
	consts := map[string]float64{"pi": math.Pi, "e": math.E, "tau": 2 * math.Pi}
	if v, ok := consts[name]; ok {
		return v, nil
	}
	if p.peek() != '(' {
		return 0, fmt.Errorf("función desconocida: %s", name)
	}
	p.next()
	arg, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.peek() != ')' {
		return 0, fmt.Errorf("falta ')' en %s", name)
	}
	p.next()
	fns := map[string]func(float64) float64{
		"sqrt": math.Sqrt, "abs": math.Abs, "sin": math.Sin, "cos": math.Cos,
		"tan": math.Tan, "ln": math.Log, "log": math.Log10, "floor": math.Floor,
		"ceil": math.Ceil, "round": math.Round, "exp": math.Exp,
	}
	fn, ok := fns[name]
	if !ok {
		return 0, fmt.Errorf("función desconocida: %s", name)
	}
	return fn(arg), nil
}
