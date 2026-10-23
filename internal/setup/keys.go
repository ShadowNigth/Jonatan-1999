package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Binding es un atajo de sway ya expandido.
type Binding struct {
	// Sym es la combinación de teclas tal como la escribió el usuario ($mod
	// ya sustituido).
	Sym string
	// Command es la orden completa, con las variables expandidas.
	Command string
	// Exec es el programa que se lanza ("" si el atajo no es un exec).
	Exec string
	// Args son los argumentos que acompañan al programa.
	Args []string
	// Line es el número de línea del archivo de configuración.
	Line int
	// Mode es el modo de sway al que pertenece ("default" fuera de bloques).
	Mode string
	// Error describe un problema detectado (variable sin definir, programa que
	// no existe, tecla repetida…).
	Error string
	// Missing es el programa que falta, si el problema es ese.
	Missing string
}

// ParseBindings extrae y expande los atajos de una configuración de sway.
//
// Se recorre el texto como sway: `set $x valor` define variables (sustitución
// de una sola pasada, sin anidar), y cada `bindsym` se expande con las
// variables definidas hasta ese punto.
func ParseBindings(config string) []Binding {
	vars := map[string]string{}
	var out []Binding
	seen := map[string]int{}
	inBlock := false
	modo := "default"

	for i, raw := range strings.Split(config, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Los bloques (mode "x" { … }) no son atajos del modo normal: se
		// marcan para poder avisar en vez de contarlos como duplicados.
		if strings.HasSuffix(line, "{") {
			inBlock = true
			modo = "default"
			if rest, ok := cutPrefix(line, "mode "); ok {
				nom := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "{"))
				modo = strings.Trim(nom, `"`)
			}
			continue
		}
		if line == "}" {
			inBlock = false
			modo = "default"
			continue
		}
		if rest, ok := cutPrefix(line, "set "); ok {
			fields := strings.SplitN(strings.TrimSpace(rest), " ", 2)
			if len(fields) == 2 && strings.HasPrefix(fields[0], "$") {
				vars[fields[0]] = strings.TrimSpace(fields[1])
			}
			continue
		}
		var rest string
		var ok bool
		if rest, ok = cutPrefix(line, "bindsym "); !ok {
			if rest, ok = cutPrefix(line, "bindcode "); !ok {
				continue
			}
		}
		rest = strings.TrimSpace(rest)
		// Opciones delante de la tecla: --release, --locked, --no-warn…
		for strings.HasPrefix(rest, "--") {
			if sp := strings.IndexByte(rest, ' '); sp >= 0 {
				rest = strings.TrimSpace(rest[sp+1:])
			} else {
				rest = ""
			}
		}
		sp := strings.IndexByte(rest, ' ')
		if sp < 0 {
			continue
		}
		sym := expandVars(rest[:sp], vars)
		cmdText := expandVars(strings.TrimSpace(rest[sp+1:]), vars)
		b := Binding{Sym: sym, Command: cmdText, Line: i + 1, Mode: modo}
		if !inBlock {
			if prev, dup := seen[sym]; dup {
				b.Error = fmt.Sprintf("tecla repetida (también en la línea %d)", prev)
			}
			seen[sym] = b.Line
		}
		if !strings.HasPrefix(cmdText, "exec") && !strings.HasPrefix(cmdText, "exec_always") {
			// Órdenes propias de sway (kill, focus, move…): no hay programa.
			out = append(out, b)
			continue
		}
		// exec [--no-startup-id] [--no-warn] <orden>
		cmdText = strings.TrimSpace(strings.TrimPrefix(cmdText, "exec_always"))
		cmdText = strings.TrimSpace(strings.TrimPrefix(cmdText, "exec"))
		for strings.HasPrefix(cmdText, "--") {
			if sp := strings.IndexByte(cmdText, ' '); sp >= 0 {
				cmdText = strings.TrimSpace(cmdText[sp+1:])
			} else {
				cmdText = ""
			}
		}
		b.Command = cmdText
		if rest, ok := cutPrefix(cmdText, "sh -c "); ok {
			// Un sh -c "…" puede encadenar varios programas: se comprueba el
			// primero que parezca un binario propio.
			b.Exec, b.Args = firstProgram(strings.Trim(rest, `'"`))
			out = append(out, b)
			continue
		}
		b.Exec, b.Args = firstProgram(cmdText)
		out = append(out, b)
	}
	return out
}

// firstProgram separa el primer programa de una orden de shell.
func firstProgram(cmd string) (string, []string) {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return "", nil
	}
	prog := strings.Trim(fields[0], `'"`)
	if prog == "" {
		return "", nil
	}
	return prog, fields[1:]
}

// expandVars sustituye $variables de una sola pasada, como sway.
func expandVars(s string, vars map[string]string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '$' {
			out.WriteByte(s[i])
			i++
			continue
		}
		j := i + 1
		for j < len(s) && isVarChar(s[j]) {
			j++
		}
		name := s[i:j]
		if v, ok := vars[name]; ok {
			out.WriteString(v)
		} else {
			out.WriteString(name)
		}
		i = j
	}
	return out.String()
}

func isVarChar(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func cutPrefix(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):], true
	}
	return s, false
}

// ProgramAvailable indica si el programa se puede ejecutar: ruta absoluta
// existente o binario en el PATH de swaydesk.
func ProgramAvailable(prog string) bool {
	if prog == "" {
		return true
	}
	if strings.Contains(prog, "/") {
		st, err := os.Stat(prog)
		return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
	}
	_, err := exec.LookPath(prog)
	return err == nil
}

// CheckBindings marca los atajos cuyo programa no existe y los que tienen la
// tecla repetida. Devuelve cuántos problemas hay.
func CheckBindings(bs []Binding) int {
	bad := 0
	for i := range bs {
		b := &bs[i]
		if b.Error == "" && b.Exec != "" && !ProgramAvailable(b.Exec) {
			b.Error = fmt.Sprintf("no se encuentra el programa %q", b.Exec)
			if b.Missing == "" {
				b.Missing = b.Exec
			}
		}
		if b.Error != "" {
			bad++
		}
	}
	return bad
}

// BindingSource lee la configuración de sway que está usando la sesión, con la
// configuración instalada como alternativa, y devuelve el texto y su origen.
func BindingSource(live func() (string, error)) (string, string) {
	if live != nil {
		if text, err := live(); err == nil && strings.TrimSpace(text) != "" {
			return text, "sway (config en vivo)"
		}
	}
	path := filepath.Join(configDir(), "sway", "config")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", path
	}
	return string(data), path
}

// SortBindings ordena por combinación para que la lista sea estable.
func SortBindings(bs []Binding) {
	sort.SliceStable(bs, func(i, j int) bool { return bs[i].Sym < bs[j].Sym })
}
