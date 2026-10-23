# Personalización

## 1. Ajustes generales

`~/.config/swaydesk/config.toml` es tu archivo. `swaydesk init` no lo pisa (solo
con `--force`, y entonces crea `.bak-<fecha>`).

```toml
[general]
theme = "tokyonight"
nerd_font = true
terminal = "kitty"          # foot, kitty, alacritty, wezterm, ghostty, xterm, st
launcher = "swaymenu --float"
editor = "nvim"
file_manager = "thunar"
browser = "firefox"
confirm_power = true        # pedir confirmación antes de reiniciar/apagar

[ui]
font = "JetBrainsMono Nerd Font"   # vacío = autodetección con fc-list
font_size = 11
bar_font_size = 13
width = 72                  # columnas de la ventana flotante (900px aprox.)
height = 20
border = "rounded"          # rounded, square, double, thick, ascii

[input]
keyboard_layout = "latam"   # latam, us, es, de, fr, br...
```

Después de editar:

```bash
swaydesk apply     # regenera waybar/foot/mako/swaylock y recarga
swaymsg reload     # solo si cambiaste la plantilla de sway
```

## 2. Temas

```bash
swaydesk theme                  # lista con muestras de color
swaydesk theme gruvbox          # cambia y aplica en caliente
swaymenu --themes               # selector interactivo (Super+t)
swayctl                         # pestaña "Apariencia" (Super+i, Tab ×3)
```

`swaydesk theme X` reescribe:

| Archivo | Qué cambia |
|---|---|
| `~/.config/waybar/style.css` | barra de estado (recarga con `SIGUSR2`) |
| `~/.config/foot/foot.ini` | terminal |
| `~/.config/mako/config` | notificaciones |
| `~/.config/swaylock/config` | pantalla de bloqueo |
| sesión de sway vía IPC | colores de borde y de título de las ventanas |

### Añadir tu propia paleta

En `internal/ui/theme.go`, dentro de `BuiltinPalettes`:

```go
"mi-tema": {
    Name: "Mi tema",
    Bg: "#101418", BgAlt: "#0b0e11", BgSel: "#1d242c",
    Fg: "#e6e6e6", FgDim: "#b0b6bd", Muted: "#6b7280",
    Accent: "#7dd3fc", Accent2: "#c084fc",
    Ok: "#86efac", Warn: "#fde047", Err: "#f87171", Border: "#2b3440",
},
```

Añade el nombre a `PaletteNames()` y recompila. Aparecerá en todos los menús.

## 3. Atajos de teclado

Los binds viven en `~/.config/sway/config`, generado desde
[`assets/files/sway/config`](../assets/files/sway/config).

- **Override rápido sin tocar lo generado**: edita
  `~/.config/sway/config.d/90-user.conf` (se incluye al final, así que gana):

```conf
bindsym Super+Shift+s exec spotify
bindsym Super+F1 exec swaydesk screenshot region
for_window [app_id="pavucontrol"] floating enable, resize set 800 500
output HDMI-A-1 resolution 2560x1440@144Hz position 0,0 scale 1
output eDP-1 scale 1.25
```

- **Cambiar algo de la plantilla** (para que sobreviva a `init --force`): edita
  el archivo en `assets/files/sway/config` y ejecuta
  `swaydesk init --force --only sway`.

Recarga siempre con `Super+Shift+c` o `swaymsg reload`.

## 4. Menús y acciones

### Lanzador

- Los comandos del `PATH` no se listan por defecto (son cientos y ensucian).
  `Super+Shift+d` (`swaymenu --float --commands`) los incluye.
- El historial está en `~/.cache/swaydesk/history`. Bórralo para reiniciar el
  orden de resultados.
- La calculadora se activa empezando el filtro por `=`:
  `=2^10`, `=sqrt(144)+3`, `=100*1.19` (IVA), `=round(2.5)`.

### Añadir una acción al menú

En `cmd/swaymenu/main.go`, dentro de `buildItems`, añade una entrada a `builtin`:

```go
{"captura-todo", "Captura: pantalla completa", "grim", theme.Icons.Camera, "grim"},
```

El último campo es el binario requerido: si no está instalado, la acción no se
muestra. Para lógica propia, añade un caso en `runAction`:

```go
case "captura-todo":
    fail(actions.Screenshot("full"))
```

### Portapapeles

```bash
swayclip                 # autodetecta cliphist, clipman o wl-paste
swayclip --backend clipman
```

Para tener historial persistente, `~/.config/swaydesk/autostart.sh` arranca
`wl-paste --type text --watch cliphist store` si `cliphist` está instalado.
Puedes borrar la entrada seleccionada con `Ctrl+d`.

### Autostart

`~/.config/swaydesk/autostart.sh` es el script que sway lanza al arrancar
(`exec_always`). Cada bloque comprueba que el programa exista, así que no falla
si te falta algo. Añade tus programas al final:

```sh
quiet firefox
quiet nm-applet
quiet blueman-applet
```

## 5. Barra de estado (waybar)

`waybar/config.json` y `waybar/style.css` son plantillas. Los módulos de sistema
llaman a `swaydesk status --module`:

```json
"custom/recording": {
  "exec": "swaydesk status --module recording",
  "interval": 2
}
```

Módulos disponibles: `logo`, `recording`, `clock`, `cpu`, `ram`, `battery`,
`volume`, `uptime`, `sway`. Ejemplo de módulo nuevo:

```json
"custom/uptime": {
  "exec": "swaydesk status --module uptime",
  "interval": 60,
  "format": "⏱ {}"
}
```

Para ver el estado completo en JSON (útil para scripts):

```bash
swaydesk status --json | jq '.system.cpu_percent, .sway.focused'
```

## 6. Notificaciones e integración

```bash
swaydesk notify "Título" "Cuerpo" --urgency low
swaydesk volume up          # wpctl / pactl / amixer, lo que exista
swaydesk screenshot region  # grim + slurp, guarda en ~/Capturas
swaydesk lock
swaydesk power reboot
```

Desde un script o un bind de sway puedes usar los binarios directamente:
`swaymenu --float`, `swayctl`, `swayclip`.

## 7. Fuentes

`swaydesk doctor` indica si hay Nerd Font. Recomendado en Arch:

```bash
sudo pacman -S ttf-jetbrains-mono-nerd
```

Si prefieres no depender de Nerd Font, pon `nerd_font = false`: se usan símbolos
Unicode (◆ ▭ ◍ ⊘ ◎) que DejaVu Sans Mono y Noto dibujan correctamente.
