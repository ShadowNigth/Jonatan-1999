package actions

import (
	"fmt"
	"strings"

	"github.com/ShadowNigth/swaydesk/internal/ui"
)

// SelectorOptions ajusta el comportamiento de un menú.
type SelectorOptions struct {
	Filter      string
	Multi       bool
	Placeholder string
	// AllowCustom permite escribir un valor libre si el filtro no coincide.
	AllowCustom bool
}

// Pick muestra un menú y devuelve la clave elegida. El segundo valor indica si
// el usuario confirmó (false = canceló).
func Pick(theme *ui.Theme, title string, items []ui.Item, opts SelectorOptions) (string, []string, bool, error) {
	sc, err := ui.OpenScreen()
	if err != nil {
		return "", nil, false, err
	}
	defer sc.Close()

	sel := ui.NewSelector(theme, title, items)
	sel.SetFilter(opts.Filter)
	sel.Multi = opts.Multi
	if opts.Placeholder != "" {
		sel.Placeholder = opts.Placeholder
	}
	accepted := false

	err = sc.Loop(
		func(w, h int) string { return sel.View(w, h) },
		func(k ui.Key) bool {
			switch {
			case k.Is("esc"):
				return true
			case k.Is("enter"):
				accepted = true
				return true
			case k.Is("up") || k.IsCtrlRune('p'):
				sel.Move(-1)
			case k.Is("down") || k.IsCtrlRune('n'):
				sel.Move(1)
			case k.Is("pgup"):
				sel.Move(-10)
			case k.Is("pgdn"):
				sel.Move(10)
			case k.Is("home"):
				sel.Cursor = 0
			case k.Is("end"):
				sel.Cursor = len(sel.Filtered()) - 1
			case k.Is("backspace"):
				sel.Backspace()
			case k.IsCtrlRune('u'):
				sel.ClearFilter()
			case k.IsCtrlRune('w'):
				// Borra la última palabra del filtro.
				f := strings.TrimRight(sel.Filter, " ")
				if i := strings.LastIndexByte(f, ' '); i >= 0 {
					sel.SetFilter(f[:i+1])
				} else {
					sel.SetFilter("")
				}
			case k.Is("space") && opts.Multi:
				sel.ToggleMarks()
			case k.IsRune(' ') && opts.Multi:
				sel.ToggleMarks()
			case k.Rune >= 32 && !k.Ctrl && !k.Alt:
				sel.Type(k.Rune)
			}
			return false
		}, nil)
	if err != nil {
		return "", nil, false, err
	}
	if !accepted {
		return "", nil, false, nil
	}
	if opts.Multi {
		keys := sel.MarkedKeys()
		if len(keys) > 0 {
			return "", keys, true, nil
		}
	}
	if opts.AllowCustom {
		if it, ok := sel.Selected(); ok {
			return it.Key, nil, true, nil
		}
		if strings.TrimSpace(sel.Filter) != "" {
			return sel.Filter, nil, true, nil
		}
		return "", nil, false, nil
	}
	it, ok := sel.Selected()
	if !ok {
		return "", nil, false, nil
	}
	return it.Key, nil, true, nil
}

// Confirm muestra un diálogo de confirmación con dos opciones.
func Confirm(theme *ui.Theme, title, message string) (bool, error) {
	items := []ui.Item{
		{Key: "yes", Label: "Sí, continuar", Icon: theme.Icons.Check},
		{Key: "no", Label: "No, cancelar", Icon: theme.Icons.Cross},
	}
	key, _, ok, err := Pick(theme, title+"  ·  "+message, items, SelectorOptions{})
	if err != nil || !ok {
		return false, err
	}
	return key == "yes", nil
}

// PowerMenuItems devuelve las acciones de energía como entradas de menú.
func PowerMenuItems(theme *ui.Theme) []ui.Item {
	items := []ui.Item{}
	for _, a := range PowerActions(true) {
		label := a.Label
		icon := a.Icon
		desc := a.Desc
		if tl, ok := powerIcons()[a.ID]; ok {
			icon = tl(theme.Icons)
		}
		items = append(items, ui.Item{Key: a.ID, Label: label, Hint: desc, Icon: icon})
	}
	return items
}

func powerIcons() map[string]func(ui.Icons) string {
	return map[string]func(ui.Icons) string{
		"lock":     func(i ui.Icons) string { return i.Lock },
		"suspend":  func(i ui.Icons) string { return i.Suspend },
		"logout":   func(i ui.Icons) string { return i.Logout },
		"reboot":   func(i ui.Icons) string { return i.Reboot },
		"poweroff": func(i ui.Icons) string { return i.Power },
	}
}

// RunPowerAction ejecuta la acción de energía indicada.
func RunPowerAction(id string, confirmUI bool, theme *ui.Theme) error {
	for _, a := range PowerActions(confirmUI) {
		if a.ID != id {
			continue
		}
		if a.Danger && confirmUI {
			ok, err := Confirm(theme, a.Label, "¿Seguro que quieres "+strings.ToLower(a.Label)+"?")
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
		}
		return a.Run()
	}
	return fmt.Errorf("acción desconocida: %s", id)
}

// ThemePicker muestra un menú con las paletas disponibles.
func ThemePicker(theme *ui.Theme, current string) (string, bool, error) {
	var items []ui.Item
	for _, name := range ui.PaletteNames() {
		p := ui.PaletteByName(name)
		mark := " "
		if name == current {
			mark = theme.Icons.Check
		}
		items = append(items, ui.Item{
			Key: name, Label: p.Name, Hint: name, Icon: mark,
			Desc: string(p.Bg) + " " + string(p.Accent),
		})
	}
	key, _, ok, err := Pick(theme, "Apariencia · elegir tema", items, SelectorOptions{})
	return key, ok, err
}
