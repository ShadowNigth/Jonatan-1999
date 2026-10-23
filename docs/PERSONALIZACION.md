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

[wallpaper]
kind = "scene"              # scene, image, gif, video, none
name = "petals"             # escena procedural (swaydesk wallpaper list)
path = ""                   # ruta del archivo cuando kind es image/gif/video
mode = "fill"               # fill, fit, stretch, center, tile
fps = 30                    # fotogramas por segundo máximos
protocol = "auto"           # auto, kitty, sixel, ansi (solo vista en terminal)
adaptive = false            # aplicar automáticamente un tema derivado del fondo
dir = "~/Imágenes/Fondos"   # carpeta que explora el selector

[effects]
terminal_opacity = 0.94     # alpha de foot (1 = opaco)
bar_opacity = 0.86          # alpha de waybar
active_opacity = 1.00       # ventana enfocada
inactive_opacity = 0.94     # ventanas sin foco
blur = false                # requiere swayfx
gaps_inner = 6
gaps_outer = 8
rounding = 12               # requiere swayfx
```

Después de editar:

```bash
swaydesk apply     # regenera waybar/foot/mako/swaylock y recarga
swaymsg reload     # solo si cambiaste la plantilla de sway
```

## 2. Temas

Con la galería interactiva (`Super+t` o `swaydesk theme --gallery`) puedes
recorrer todas las paletas viendo la vista previa en vivo: degradados, muestras,
ventana de ejemplo y el fondo animado de cada tema.

```bash
swaydesk theme                # lista agrupada por categoría
swaydesk theme sakura         # aplica una paleta
swaydesk theme --gallery      # selector a pantalla completa
swaydesk theme --from-wallpaper   # paleta derivada de los colores del fondo
swaydesk theme --json         # catálogo en JSON (para scripts)
```

Las categorías son `anime`, `oscuro`, `retro`, `claro` y `adaptativo`. El atajo
`Tab` cambia de categoría dentro de la galería y `espacio` pausa la animación de
la vista previa.

### Temas adaptativos

`swaydesk theme --from-wallpaper` (o `swaydesk wallpaper adaptive [archivo]`)
analiza el fondo, extrae sus colores dominantes y construye una paleta completa
—fondo, superficies, texto, acentos, estados y gradiente— ajustada para cumplir
un contraste mínimo legible. Se guarda como una paleta normal en
`~/.config/swaydesk/themes/adaptativo.toml`, así que puedes editarla a mano y
aplicarla como cualquier otra.

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

## 5. Fondos animados

```bash
swaydesk wallpaper list                  # escenas y archivos encontrados
swaydesk wallpaper set scene:sakura      # cambia la escena
swaydesk wallpaper set ~/Imágenes/a.gif --live
swaydesk wallpaper set ~/Vídeos/b.mp4    # requiere ffmpeg
swaydesk wallpaper status                # tipo, demonio y compositor
swaydesk wallpaper stop                  # detiene el fondo animado
```

`--live` (o `swaydesk wallpaper daemon`) arranca el demonio que dibuja el fondo
en una capa `wlr-layer-shell` **detrás de las ventanas**; el autostart lo lanza
al iniciar sesión. Sin demonio, sway muestra un fotograma estático exportado a
`~/.cache/swaydesk/fondo-estatico.png`.

Escenas disponibles: `petals`, `snow`, `stars`, `rain`, `aurora`, `wave`,
`city`, `embers`, `grid`, `meadow`, `bubbles`. Cada una toma los colores del
tema activo, así que cambiar de paleta cambia el fondo.

También puedes usar el nombre de una paleta y swaydesk toma su escena asociada:
`swaydesk wallpaper set scene:sakura` es lo mismo que `scene:petals`, y
`miku` equivale a `wave`.

Para que sway dibuje el respaldo estático se escribe
`~/.config/sway/config.d/50-wallpaper.conf`; puedes sobrescribirlo con tus
propias reglas `output * bg ...`.

### Vista previa en la terminal

```bash
swaydesk wallpaper play           # el fondo actual a pantalla completa
swaydesk wallpaper gallery        # selector con vista previa animada
```

Teclas en la reproducción: `q`/`Esc` salir, `espacio` pausar, `+`/`-` FPS,
`i` ocultar la ayuda. En terminales con protocolo gráfico (kitty, Ghostty, foot)
se envía la imagen real; si no, se dibuja con medios bloques.

## 6. Translucidez

La mayoría de las aplicaciones de Wayland respetan el alpha de sus propias
superficies, así que swaydesk ajusta cada pieza por separado:

- **foot**: `[colors] alpha` ← `effects.terminal_opacity`.
- **waybar**: `background: alpha(@bg, bar_opacity)` en los módulos y tooltips.
- **mako**: color de fondo y borde con canal alfa derivado del tema.
- **sway**: `client.focused_inactive` / `client.unfocused` con alfa y reglas
  `for_window [app_id=…] opacity …` para terminales, gestores de archivos,
  reproductores y las ventanas flotantes de swaydesk. Añade más con:

```
for_window [app_id="mi-app"] opacity 0.92
```

- **swayfx** (opcional): si está instalado, `swaydesk apply` añade
  `corner_radius` y, con `blur = true`, `blur enable`. En sway normal esas
  líneas quedan comentadas porque no existen.

## 7. Barra de estado (waybar)

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

## 8. Notificaciones e integración

```bash
swaydesk notify "Título" "Cuerpo" --urgency low
swaydesk volume up          # wpctl / pactl / amixer, lo que exista
swaydesk screenshot region  # grim + slurp, guarda en ~/Capturas
swaydesk lock
swaydesk power reboot
```

Desde un script o un bind de sway puedes usar los binarios directamente:
`swaymenu --float`, `swayctl`, `swayclip`.

## 9. Fuentes

`swaydesk doctor` indica si hay Nerd Font. Recomendado en Arch:

```bash
sudo pacman -S ttf-jetbrains-mono-nerd
```

Si prefieres no depender de Nerd Font, pon `nerd_font = false`: se usan símbolos
Unicode (◆ ▭ ◍ ⊘ ◎) que DejaVu Sans Mono y Noto dibujan correctamente.
