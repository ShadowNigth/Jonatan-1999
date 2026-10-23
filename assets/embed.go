// Package assets contiene las plantillas de configuración del escritorio
// (sway, waybar, foot, mako, swaylock, archivos .desktop) embebidas en el
// binario para que `swaydesk init` funcione sin necesidad del código fuente.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:files
var embedded embed.FS

// Files devuelve el sistema de archivos con las plantillas, sin el prefijo
// "files/".
func Files() fs.FS {
	sub, err := fs.Sub(embedded, "files")
	if err != nil {
		panic(err)
	}
	return sub
}

// List devuelve las rutas relativas de todas las plantillas.
func List() ([]string, error) {
	var out []string
	err := fs.WalkDir(Files(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}
