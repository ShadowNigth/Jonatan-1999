package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Este archivo implementa un subconjunto de TOML suficiente para los archivos
// de configuración de swaydesk: secciones, claves simples, cadenas, números,
// booleanos y arrays de una línea. No pretende cubrir la especificación
// completa, pero acepta comentarios y la notación "seccion.clave".

// parseTOML convierte el texto en un mapa plano "seccion.clave" -> valor.
func parseTOML(text string) (map[string]any, error) {
	values := map[string]any{}
	section := ""
	lines := strings.Split(text, "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return nil, fmt.Errorf("línea %d: sección sin cerrar: %s", i+1, line)
			}
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			return nil, fmt.Errorf("línea %d: falta '=' en %q", i+1, line)
		}
		key := unquoteKey(strings.TrimSpace(line[:eq]))
		valText := strings.TrimSpace(line[eq+1:])
		if key == "" {
			return nil, fmt.Errorf("línea %d: clave vacía", i+1)
		}
		if section != "" {
			key = section + "." + key
		}
		val, err := parseValue(valText)
		if err != nil {
			return nil, fmt.Errorf("línea %d (%s): %w", i+1, key, err)
		}
		values[key] = val
	}
	return values, nil
}

// unquoteKey admite claves entre comillas (necesario para nombres de
// workspace con ':' o espacios: "3:web", "chat de tarde").
func unquoteKey(key string) string {
	if len(key) >= 2 {
		if key[0] == '"' && key[len(key)-1] == '"' {
			if s, err := strconv.Unquote(key); err == nil {
				return s
			}
		}
		if key[0] == '\'' && key[len(key)-1] == '\'' {
			return key[1 : len(key)-1]
		}
	}
	return key
}

// stripComment elimina un comentario # que no esté dentro de comillas.
func stripComment(line string) string {
	inDouble, inSingle := false, false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			if !inSingle && (i == 0 || line[i-1] != '\\') {
				inDouble = !inDouble
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '#':
			if !inDouble && !inSingle {
				return line[:i]
			}
		}
	}
	return line
}

func parseValue(s string) (any, error) {
	if s == "" {
		return nil, fmt.Errorf("valor vacío")
	}
	switch {
	case strings.HasPrefix(s, "\""):
		str, rest, err := parseQuoted(s, '"')
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(rest) != "" {
			return nil, fmt.Errorf("texto sobrante tras la cadena: %q", rest)
		}
		return str, nil
	case strings.HasPrefix(s, "'"):
		str, rest, err := parseQuoted(s, '\'')
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(rest) != "" {
			return nil, fmt.Errorf("texto sobrante tras la cadena: %q", rest)
		}
		return str, nil
	case strings.HasPrefix(s, "["):
		return parseArray(s)
	case s == "true":
		return true, nil
	case s == "false":
		return false, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return int(n), nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}
	return nil, fmt.Errorf("valor no reconocido: %q", s)
}

func parseQuoted(s string, quote byte) (string, string, error) {
	var b strings.Builder
	escaped := false
	for i := 1; i < len(s); i++ {
		c := s[i]
		if escaped {
			switch c {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\', '"', '\'':
				b.WriteByte(c)
			default:
				b.WriteByte('\\')
				b.WriteByte(c)
			}
			escaped = false
			continue
		}
		if c == '\\' && quote == '"' {
			escaped = true
			continue
		}
		if c == quote {
			return b.String(), s[i+1:], nil
		}
		b.WriteByte(c)
	}
	return "", "", fmt.Errorf("cadena sin cerrar: %q", s)
}

func parseArray(s string) ([]string, error) {
	inner := strings.TrimSpace(s)
	if !strings.HasPrefix(inner, "[") || !strings.HasSuffix(inner, "]") {
		return nil, fmt.Errorf("array sin cerrar: %q", s)
	}
	inner = strings.TrimSpace(inner[1 : len(inner)-1])
	if inner == "" {
		return []string{}, nil
	}
	var out []string
	rest := inner
	for strings.TrimSpace(rest) != "" {
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, ",") {
			rest = rest[1:]
			continue
		}
		var val any
		var err error
		if strings.HasPrefix(rest, "\"") || strings.HasPrefix(rest, "'") {
			var str, tail string
			str, tail, err = parseQuoted(rest, rest[0])
			if err != nil {
				return nil, err
			}
			val = str
			rest = tail
		} else {
			end := strings.IndexAny(rest, ",")
			if end < 0 {
				end = len(rest)
			}
			val, err = parseValue(strings.TrimSpace(rest[:end]))
			if err != nil {
				return nil, err
			}
			rest = rest[end:]
		}
		out = append(out, fmt.Sprint(val))
	}
	return out, nil
}
