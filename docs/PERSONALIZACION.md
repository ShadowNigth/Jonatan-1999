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
browser = "chromium"
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

[boot]
enabled = true              # animación al iniciar la sesión
style = "sakura"            # sakura (pétalos), neon (lluvia), matrix, minimal
duration_ms = 2600          # 0 = automática
logo = true                 # logotipo grande en bloques

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
quiet chromium
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

## 6. Animación de arranque

```bash
swaydesk boot                     # la reproduce ahora mismo
swaydesk boot --style=neon        # otro estilo sin tocar la configuración
swaydesk boot --duration=1500     # más corta
swaydesk boot --list              # estilos disponibles
swaydesk boot --no-logo           # sin el logotipo grande
```

La lanza el autostart en una ventana de terminal a pantalla completa
(`app_id=swaydesk-boot`), dentro de sway, con la clase ya configurada en
`assets/files/sway/config`. Se salta con cualquier tecla y desaparece sola.

Estilos:

| Estilo | Efecto |
|---|---|
| `sakura` | pétalos y motas cayendo con la paleta del tema (por defecto) |
| `neon` | lluvia vertical de trazos tipo lluvia digital |
| `matrix` | glifos y dígitos verdes sobre negro |
| `minimal` | sin partículas: solo logotipo, pasos y barra |

La lista de pasos se genera en vivo (`boot.DefaultSteps`): compositor, barra,
notificaciones, portapapeles, tema, fondo y ventanas; cada uno con su estado real.

Para desactivarla del todo, `enabled = false` en `[boot]` (o borra la línea
`swaydesk boot` del autostart). Vuelve a verla cuando quieras con `Super+Shift+b`.

## 7. Translucidez

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

### Añadir cualquier aplicación

`swaydesk opacity` gestiona reglas de translucidez sin editar archivos a mano:

```bash
swaydesk opacity presets              # catálogo de apps conocidas
swaydesk opacity all                  # escribe las ~40 reglas del catálogo
swaydesk opacity add obsidian 0.92    # añade (o ajusta) una app
swaydesk opacity add --class Spotify 0.9   # por clase en vez de app_id
swaydesk opacity remove obsidian      # quita la regla
swaydesk opacity list                 # reglas propias activas
swaydesk opacity reset                # vuelve a la plantilla de sway
```

Las reglas se guardan en `~/.config/sway/config.d/60-opacity.conf` y se aplican
en caliente por IPC a las ventanas abiertas, además de a las futuras. Para
descubrir el `app_id` real de una ventana:

```bash
swaymsg -t get_tree | grep app_id
```

## 8. Barra de estado propia (`swaybar`)

Desde la versión con `swaybar`, el escritorio trae su propia barra: un solo
binario que dibuja islas redondeadas, translúcidas, con tipografía de mapa de
bits (PSF) y los colores del tema activo. No necesita waybar, GTK ni ninguna
librería gráfica. Todo se configura en `[bar]` de `~/.config/swaydesk/config.toml`:

```toml
[bar]
enabled = true             # false = vuelve a waybar automáticamente
position = "top"           # top · bottom
height = 38                # alto en píxeles
margin = 6                 # separación con los bordes de la pantalla
padding = 14               # relleno interior de cada isla
gap = 10                   # hueco entre islas
radius = 0                 # 0 = pastilla completa
opacity = 0.88             # transparencia de las islas
islands = true             # false = una sola barra continua
logo = true
font = ""                  # ruta a un .psf / .psf.gz; vacío = fuente del sistema
font_scale = 1.0
shadow = true              # sombra suave bajo las islas
exclusive = true           # las ventanas no se solapan con la barra
modules = ["media", "notify", "cpu", "ram", "volume", "network", "battery", "clock"]
```

Módulos disponibles: `media` (reproductor vía `playerctl`), `recording`
(indicador de `wf-recorder`), `cpu`, `ram`, `disk`, `temp`, `uptime`,
`volume`, `network`, `battery`, `updates`, `title` (ventana enfocada), `clock`,
`clock24` y `date`.

```bash
swaydesk bar                  # estado y módulos actuales
swaydesk bar modules cpu ram volume network battery clock
swaydesk bar off              # desactiva swaybar; el autostart usa waybar
swaydesk bar preview --width 1600 --out ~/barra.png   # PNG sin sesión gráfica
swaybar --demo --preview      # datos de ejemplo, útil para probar el diseño
```

Plantea la barra con varios anchos y comprueba que el centro no se solapa; si
quieres la barra flotante estilo "isla", deja `islands = true` y sube `margin`.

### Añadir un módulo nuevo

Los módulos viven en `internal/bar/render.go` (`rightItems`) y los datos en
`cmd/swaybar/data.go`. Un módulo es una entrada de texto con color opcional y
un estado alternativo (`ok`/`warn`/`error`) para casos como batería baja o red
caída.

### Todo el camino de vuelta a waybar

Si prefieres waybar y sus módulos personalizados, pon `enabled = false` en
`[bar]` (o ejecuta `swaydesk bar off`) y reinicia la sesión: la plantilla de
waybar sigue instalada y se colorea con el tema igual que antes.

## 8c. Notificaciones propias (`swaynotify`)

El escritorio trae su propio daemon de notificaciones: implementa
`org.freedesktop.Notifications` (así que Chromium, mpd, pacman, tu script de
copias… todo funciona sin tocar nada) y dibuja las tarjetas en píxeles sobre
una capa overlay, con la fuente y los colores del tema activo.

```toml
[notify]
enabled = true             # false = usar mako
position = "top-right"     # top-right, top-left, bottom-right, bottom-left
width = 360                # ancho de cada tarjeta
margin = 14                # separación con el borde de la pantalla
padding = 14               # relleno interior
gap = 8                    # separación entre tarjetas
radius = 14                # esquinas redondeadas
opacity = 0.94             # transparencia de las tarjetas
timeout_ms = 6000          # tiempo en pantalla de las normales
critical_timeout_ms = 0    # 0 = las urgentes se quedan hasta cerrarlas
max_visible = 4            # cuántas se apilan
history = 50               # entradas que se guardan
max_lines = 3              # líneas del cuerpo
dnd = false                # arrancar en modo «no molestar»
group = true               # agrupar por aplicación en pilas desplegables
font = ""                  # fuente PSF (vacío = automática)
font_scale = 0             # 0 = automática
```

### Qué se dibuja

- **Punto de color** según la urgencia: acento (normal), atenuado (baja) y
  error (crítica, que además lleva el borde marcado).
- **Aplicación y hora** arriba, con el resumen en tamaño doble y el cuerpo
  recortado a `max_lines` líneas con puntos suspensivos.
- **Acciones** como botones `[Etiqueta]`: se pulsan con el ratón y el cliente
  recibe `ActionInvoked`.
- **Barra de progreso** con el hint `progress` (descargas, copias…).
- **Cuenta atrás**: una línea fina se va vaciando hasta que la tarjeta
  desaparece sola. Un clic en cualquier parte de la tarjeta la cierra
  (`NotificationClosed` con motivo 2).

### Notificaciones agrupadas

Con `group = true` (por defecto), todos los avisos de una misma aplicación se
apilan en **una sola tarjeta**: las capas asoman por debajo, un contador indica
cuántos hay y la línea grande muestra el último mensaje seguido de «y N más».

- **Un clic** en la tarjeta (o en `[ver N]`) despliega la pila: aparece la franja
  del grupo —con `[plegar]`— y debajo una tarjeta por aviso, en orden.
- **Clic derecho** en cualquier tarjeta de la pila la cierra entera; el clic
  izquierdo en una tarjeta suelta la cierra como siempre y en la franja pliega
  el grupo.
- El modo «no molestar» y la caducidad funcionan igual; `max_visible` cuenta
  **pilas**, no avisos, así que cinco mensajes del mismo chat ocupan el hueco de
  uno solo.
- Cada cliente puede separar sus pilas con una etiqueta: se aceptan
  `x-dunst-stack-tag`, `x-kde-group-id` y `group` como hints de cadena. Sirve
  para no mezclar «chat de Marta» y «chat de Luis» de la misma aplicación:

```bash
swaynotify send --app Telegram --group chat-marta --reply "Marta" "¿Comemos?"
swaynotify send --app Telegram --group chat-marta "Marta" "Traigo el postre"
```

- En el centro (`swaydesk notifications center`) las pilas se ven como una fila
  con el número de avisos: `Enter`/`Tab` la abre, `g` quita o pone el
  agrupamiento y `x` cierra la pila completa.

### Responder desde la tarjeta (`inline-reply`)

Los chats (Telegram, Signal, Discord, el cliente de Matrix…) pueden pedir una
**respuesta rápida**. Cuando una aplicación declara la capacidad `inline-reply`
y añade la acción con clave `inline-reply`, la tarjeta muestra un último botón,
**Responder**, con el texto de ayuda que manda la app en el hint
`x-kde-reply-placeholder-text`:

```bash
swaynotify send --app "Chat" --reply --reply-hint "Responder a Marta…" \
    --timeout 20000 "Marta" "¿Comemos a las dos?"
```

Al pulsarlo se abre una **ventana flotante de terminal** (`swaynotify reply
<id>`) con el campo de escritura; la tarjeta queda retenida (`Hold`) para que
no caduque mientras escribes. Al confirmar con `Enter`:

1. Se sanea el texto: una sola línea, sin caracteres de control, máximo
   `MaxReplyLen` (512) caracteres.
2. El daemon emite **`NotificationReplied(id, texto)` en modo *unicast*, solo
   al cliente que creó la notificación**. El texto no se difunde por el bus.
3. La tarjeta se cierra salvo que la app la haya marcado como `resident`.

La continuación natural es el **centro de notificaciones**: una TUI con todo lo
que hay en pantalla y el historial, filtro incremental, y las mismas acciones.

```bash
swaydesk notifications center      # también con clic en el indicador de la barra
swaynotify reply 7 "voy saliendo"  # respuesta directa, sin interfaz
```

Teclas del centro: `↑`/`↓` mover, `Enter` abrir la pila o actuar (responder si
se puede, si no cerrar), `Tab` desplegar o plegar el grupo, `g` agrupar por
aplicación, `r` responder, `x` cerrar (la pila entera si es un grupo), `m` no
molestar, `c` cerrar todas, `X` borrar historial, `F5` recargar, `Esc` salir (o
borrar el filtro).

El indicador de la barra es el módulo `notify`: muestra una pastilla con el
número de avisos en pantalla (con `+` si alguno admite respuesta), `dnd` en
ámbar cuando el modo «no molestar» está activo y `avisos` en gris cuando no hay
nada. Un clic izquierdo abre el centro en una terminal flotante.

### Desde el terminal

```bash
swaydesk notify "Compilación terminada" "Todo en verde"        # lo normal
swaynotify send --app pacman --urgency urgente --timeout -1 \
    --action "default=Abrir" "12 paquetes" "Reinicia al terminar"

swaydesk notifications status          # daemon, DND, en pantalla, historial
swaydesk notifications dnd on          # silenciar (las urgentes pasan)
swaydesk notifications history         # últimas 20, con --json para scripts
swaydesk notifications clear --all     # cerrar y borrar historial
swaydesk notifications preview         # PNG con tarjetas de ejemplo
```

El daemon atiende además una interfaz propia, `org.swaydesk.Notifications`,
con `SetDND`, `ToggleDND`, `GetStatus`, `ListHistory`, `CloseAll`,
`ClearHistory` y `Dismiss`; sirve para integrarla en la barra o en scripts con
`busctl --user call org.freedesktop.Notifications /org/freedesktop/Notifications org.swaydesk.Notifications ToggleDND`.

### Detalles que conviene saber

- Sin bus de sesión (`DBUS_SESSION_BUS_ADDRESS`) el daemon no puede arrancar.
  En una sesión de sway normal lo levanta `dbus-update-activation-environment` /
  `dbus-launch`; el autostart lo comprueba antes de lanzarlo.
- Solo puede haber un servidor de notificaciones: si ya hay `mako` o `dunst`,
  `swaynotify` avisa y sale. El autostart arranca uno u otro, nunca los dos.
- El historial se guarda en `~/.cache/swaydesk/notificaciones.json` cada cinco
  segundos y al salir, así que sobrevive a un reinicio de la sesión.
- Si un aviso llega con `replaces_id`, se actualiza la tarjeta en el sitio en
  lugar de apilar otra (barras de progreso que se refrescan, por ejemplo).

## 8d. Avisos en pantalla (`swayosd`)

El nivel de volumen aparece en una **tarjeta flotante** sobre las ventanas
—etiqueta, porcentaje en grande y barra segmentada— que se va sola con un
fundido de entrada y salida. No hace falta que ninguna aplicación (Spotify,
Chromium, mpv…) haga nada: el aviso lo dibuja el escritorio.

```toml
[osd]
enabled = true             # false = sin avisos (el volumen sigue funcionando)
position = "bottom-center" # bottom-right, bottom-left, top-center, center
width = 320                # ancho de la tarjeta en píxeles
margin = 72                # separación con el borde de la pantalla
padding = 16               # relleno interior
radius = 18                # esquinas redondeadas
opacity = 0.95             # transparencia del fondo
timeout_ms = 1400          # cuánto tiempo se queda en pantalla
fade_ms = 140              # fundido de entrada y salida (0 = sin animación)
segments = 24              # marcas de la barra de nivel
font = ""                  # fuente PSF (vacío = automática)
font_scale = 0             # 0 = el doble que la barra, para leerse de reojo
```

### Cómo se dispara

```bash
swaydesk volume up            # sube 5% y muestra el aviso
swaydesk volume down
swaydesk volume mute
swayosd volume up             # lo mismo, sin pasar por swaydesk
swayosd volume mute --step 3  # pasos más finos
swayosd show                  # solo mostrar el volumen actual
swayosd watch                 # vigilar y avisar de cualquier cambio
swaydesk osd                  # configuración y volumen actual
swaydesk osd preview osd.png  # PNG del aviso, sin sesión gráfica
```

Los atajos `XF86AudioRaiseVolume`, `XF86AudioLowerVolume` y `XF86AudioMute`
llaman a `swaydesk volume …`, que usa `swayosd` cuando está instalado. Si
quieres reaccionar a cambios que no vengan de esos atajos —auriculares
Bluetooth, otro programa, `pactl` a mano— el autostart deja `swayosd watch`
en marcha: vigila el volumen del altavoz por defecto y lanza el aviso cuando
cambie, venga de donde venga.

### Detalles que conviene saber

- La lectura del volumen usa `wpctl` (PipeWire), luego `pactl` y luego
  `amixer`; con ninguno de los tres el aviso no puede mostrarse, pero el
  escritorio sigue funcionando.
- El nombre del dispositivo sale de `wpctl inspect`/`pactl get-default-sink` y
  se recorta si no cabe; si no se puede averiguar, la tarjeta ocupa una línea
  menos.
- Por encima del 100% la barra se llena y cambia a color de aviso (volumen
  amplificado); silenciado, la barra queda vacía y una raya la cruza.
- La superficie es una capa `overlay` con la **región de entrada vacía**: el
  aviso nunca se come un clic de lo que haya debajo.
- El fundido se hace dibujando varios fotogramas con alfa creciente; con
  `fade_ms = 0` aparece y desaparece de golpe (menos repintados).

## 8b. Barra de estado (waybar)

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

## 9. Notificaciones e integración

```bash
swaydesk notify "Título" "Cuerpo" --urgency low
swaydesk volume up          # wpctl / pactl / amixer, lo que exista
swaydesk screenshot region  # grim + slurp, guarda en ~/Capturas
swaydesk lock
swaydesk power reboot
```

Desde un script o un bind de sway puedes usar los binarios directamente:
`swaymenu --float`, `swayctl`, `swayclip`.

## 10. Fuentes

`swaydesk doctor` indica si hay Nerd Font. Recomendado en Arch:

```bash
sudo pacman -S ttf-jetbrains-mono-nerd
```

Si prefieres no depender de Nerd Font, pon `nerd_font = false`: se usan símbolos
Unicode (◆ ▭ ◍ ⊘ ◎) que DejaVu Sans Mono y Noto dibujan correctamente.
