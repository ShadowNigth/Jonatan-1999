package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const configPrueba = `
set $mod Mod4
set $term foot
set $swaydesk /usr/local/bin/swaydesk
set $menu /usr/local/bin/swaymenu --float

bindsym $mod+Return exec $term
bindsym $mod+d exec $menu
bindsym $mod+q kill
bindsym $mod+Shift+e exec $swaydesk power
# bindsym $mod+t exec falso
bindsym $mod+t exec $noexiste

mode "resize" {
    bindsym h resize shrink width 20 px
    bindsym Escape mode "default"
}
bindsym $mod+r mode "resize"
`

func TestParseBindings(t *testing.T) {
	bs := ParseBindings(configPrueba)
	if len(bs) != 8 {
		t.Fatalf("se esperaban 8 atajos, hay %d: %+v", len(bs), bs)
	}
	porTecla := map[string]Binding{}
	for _, b := range bs {
		porTecla[b.Sym] = b
	}
	if b := porTecla["Mod4+Return"]; b.Exec != "foot" || b.Command != "foot" {
		t.Fatalf("Mod4+Return: %+v", b)
	}
	if b := porTecla["Mod4+d"]; b.Exec != "/usr/local/bin/swaymenu" ||
		strings.Join(b.Args, " ") != "--float" {
		t.Fatalf("Mod4+d: %+v", b)
	}
	if b := porTecla["Mod4+q"]; b.Exec != "" {
		t.Fatalf("kill no es un exec: %+v", b)
	}
	// Una variable sin definir se queda tal cual y se ve el problema.
	if b := porTecla["Mod4+t"]; !strings.Contains(b.Command, "$noexiste") {
		t.Fatalf("variable sin definir: %+v", b)
	}
	// Los comentarios no crean atajos.
	if _, ok := porTecla["Mod4+Shift+t"]; ok {
		t.Fatal("un atajo comentado no debe aparecer")
	}
	// Los del bloque van marcados con su modo.
	if b := porTecla["h"]; b.Mode != "resize" {
		t.Fatalf("modo del bloque: %+v", b)
	}
	if b := porTecla["Escape"]; b.Mode != "resize" {
		t.Fatalf("Escape del bloque: %+v", b)
	}
	if b := porTecla["Mod4+r"]; b.Mode != "default" {
		t.Fatalf("Mod4+r: %+v", b)
	}
	// h y Escape del modo resize no cuentan como duplicados de nada.
	for _, b := range bs {
		if b.Error != "" {
			t.Fatalf("error inesperado en %q: %s", b.Sym, b.Error)
		}
	}
}

func TestPowerMenuShortcutOpensFloatingMenu(t *testing.T) {
	path := filepath.Join("..", "..", "assets", "files", "sway", "config")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leer plantilla Sway: %v", err)
	}
	for _, binding := range ParseBindings(string(data)) {
		if binding.Sym != "Mod4+p" {
			continue
		}
		if binding.Error != "" {
			t.Fatalf("atajo Super+P inválido: %s", binding.Error)
		}
		if binding.Exec != "{{SWAYMENU_BIN}}" || strings.Join(binding.Args, " ") != "--power --float" {
			t.Fatalf("Super+P debe abrir el menú flotante: %+v", binding)
		}
		return
	}
	t.Fatal("la plantilla Sway no define Super+P para el menú de energía")
}

func TestParseBindingsDuplicados(t *testing.T) {
	cfg := `set $mod Mod4
bindsym $mod+d exec foot
bindsym $mod+d exec foot
`
	bs := ParseBindings(cfg)
	if len(bs) != 2 {
		t.Fatalf("atajos = %d", len(bs))
	}
	if bs[1].Error == "" || !strings.Contains(bs[1].Error, "repetida") {
		t.Fatalf("no se detectó el duplicado: %+v", bs[1])
	}
	if !strings.Contains(bs[1].Error, "línea 2") {
		t.Fatalf("la pista debe apuntar a la primera línea: %q", bs[1].Error)
	}
}

func TestCheckBindings(t *testing.T) {
	trueBin := "/bin/sh"
	bs := []Binding{
		{Sym: "a", Command: trueBin, Exec: trueBin},
		{Sym: "b", Command: "no-existe-jamas", Exec: "no-existe-jamas"},
		{Sym: "c", Command: "kill", Exec: ""},
		{Sym: "d", Command: "x", Exec: "", Error: "tecla repetida"},
	}
	if bad := CheckBindings(bs); bad != 2 {
		t.Fatalf("problemas = %d: %+v", bad, bs)
	}
	if bs[0].Error != "" {
		t.Fatalf("/bin/sh existe: %q", bs[0].Error)
	}
	if !strings.Contains(bs[1].Error, "no-existe-jamas") {
		t.Fatalf("no se marcó el programa inexistente: %q", bs[1].Error)
	}
	if !ProgramAvailable("") || ProgramAvailable("no-existe-jamas") {
		t.Fatal("ProgramAvailable mal")
	}
	if ProgramAvailable("/tmp") {
		t.Fatal("un directorio no es un programa")
	}
}

func TestExpandVarsUnaSolaPasada(t *testing.T) {
	vars := map[string]string{"$a": "$b", "$b": "valor"}
	if got := expandVars("$a", vars); got != "$b" {
		t.Fatalf("sway no expande en cascada: %q", got)
	}
	if got := expandVars("x $a y", vars); got != "x $b y" {
		t.Fatalf("expansión: %q", got)
	}
	vars["$mod"] = "Mod4"
	if got := expandVars("$mod+Shift+$nada", vars); got != "Mod4+Shift+$nada" {
		t.Fatalf("variable desconocida: %q", got)
	}
}

func TestBindingSourceFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Sin archivo ni función de config viva no hay nada.
	if text, _ := BindingSource(nil); text != "" {
		t.Fatalf("debería estar vacío: %q", text)
	}
	// La config viva tiene prioridad.
	viva := func() (string, error) { return "bindsym a exec b", nil }
	text, origin := BindingSource(viva)
	if text != "bindsym a exec b" || !strings.Contains(origin, "vivo") {
		t.Fatalf("origen = %q (%q)", origin, text)
	}
	// Si falla, se usa el archivo instalado.
	falla := func() (string, error) { return "", errFalso{} }
	if _, origin := BindingSource(falla); strings.Contains(origin, "vivo") {
		t.Fatalf("no debería usar la config viva: %q", origin)
	}
}

type errFalso struct{}

func (errFalso) Error() string { return "sin sway" }

func TestSortBindings(t *testing.T) {
	bs := []Binding{{Sym: "b"}, {Sym: "a"}, {Sym: "c"}}
	SortBindings(bs)
	if bs[0].Sym != "a" || bs[2].Sym != "c" {
		t.Fatalf("orden: %+v", bs)
	}
}
