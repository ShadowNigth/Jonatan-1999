<div align="center">

# swaydesk

**Un escritorio completo para Arch Linux + sway, escrito en Go.**

Lanzador de aplicaciones, panel de control, portapapeles, barra de estado, temas
adaptativos, fondos animados (GIF y *live wallpapers*) y sesión — todo en
**cuatro binarios estáticos**, sin dependencias externas de Go y sin un solo
script de shell.

![Lanzador de aplicaciones](docs/screenshot-menu.png)

</div>

---

## ¿Qué es esto?

`swaydesk` convierte una instalación mínima de **Arch Linux + sway** en un
escritorio utilizable y agradable sin arrastrar medio GNOME. Está escrito
íntegramente en Go (solo biblioteca estándar) e incluye su propio toolkit de
interfaz para terminal: no usa ncurses, ni Bubble Tea, ni ningún módulo de
terceros.

| Binario | Qué hace |
|---|---|
| `swaymenu` | Lanzador de aplicaciones con búsqueda difusa, historial de uso, calculadora y acciones del escritorio |
| `swayctl` | Panel de control: estado del sistema, workspaces, ventanas, temas y chuleta de atajos |
| `swayclip` | Historial del portapapeles (cliphist / clipman) con búsqueda y vista previa |
| `swaydesk` | CLI central: instala la configuración, aplica temas, diagnostica, controla la sesión |

```
╭── swaydesk · lanzador ─────────────────────────────────────────────╮
│ ⌕ fire                                                             │
│────────────────────────────────────────────────────────────────────│
│▌ ◍ Firefox                                        Navegador web    │
│  ◍ Firejail                          Ejecuta apps en una jaula      │
│                                                                     │
╰──────────────────────────── ↵ elegir · ↑↓ mover · esc cerrar ──────╯
```

## Características

- **Lanzador inteligente**: busca en aplicaciones `.desktop`, comandos del
  `PATH`, acciones integradas y calculadora (`=128*3`). Ordena por
  *frecuencia + recencia* (algoritmo de frecency), igual que `z` o `fzf`.
- **Búsqueda difusa** propia con puntuación por límites de palabra, prefijos y
  coincidencias consecutivas. Todo en Go, sin dependencias.
- **Panel de control** (`Super+i`): estado de sway y del sistema con barras,
  lista de ventanas con acciones (enfocar, cerrar, flotar, pantalla completa),
  cambio de tema en vivo y chuleta de atajos.
- **Portapapeles** con soporte de `cliphist`, `clipman` o `wl-paste`, búsqueda,
  vista previa y borrado de entradas.
- **31 paletas de color**, 21 de ellas con estética **anime** (Sakura, Miku, Asuka, Rei,
  Zero Two, Neon Tokyo, Cyberpunk, Vaporwave, Ghibli, Hoshizora, Oni, Mecha…),
  además de las clásicas (Catppuccin, Tokyo Night, Nord, Gruvbox, Dracula,
  Kanagawa, Rosé Pine, Everforest), aplicadas de forma coherente a sway,
  waybar, foot, mako, swaylock y a las propias TUI.
- **Galería de temas** (`Super+t`): selector a pantalla completa con vista
  previa real del tema — degradados, muestras de color, ventana de ejemplo y
  fondo animado — filtro difuso y navegación por categorías.
- **Temas adaptativos**: `swaydesk theme --from-wallpaper` extrae los colores
  dominantes de tu fondo (cuantización + HSL + contraste WCAG) y genera una
  paleta completa que se guarda en `~/.config/swaydesk/themes/`.
- **Fondos animados de verdad**: 11 escenas procedurales (pétalos de sakura,
  nieve, estrellas, lluvia de neón, aurora, olas, ciudad cyberpunk, brasas,
  rejilla synthwave, pradera, burbujas) y reproducción de **GIF animados**,
  **imágenes** y **vídeos** (`mp4`, `webm`, `mkv` — incluidos los de Wallpaper
  Engine) con un demonio propio que dibuja en una capa
  `wlr-layer-shell` detrás de las ventanas. Cliente Wayland implementado a mano,
  sin librerías.
- **Selector de fondos** (`Super+Shift+w`): galería con vista previa animada en
  vivo, metadatos (duración, tamaño, tipo) y exploración de tus carpetas.
- **Translucidez en todo el escritorio**: opacidad configurable para terminal,
  barra, notificaciones, ventanas activas e inactivas, reglas `for_window` para
  decenas de aplicaciones y desenfoque/esquinas cuando el compositor es swayfx.
- **Generador de configuración**: `swaydesk init` escribe una configuración
  completa y comentada de sway, waybar, foot, mako y swaylock, adaptada a tu
  hardware, tu terminal y tu fuente (detecta Nerd Fonts automáticamente).
- **Clipe IPC de sway implementado desde cero** (protocolo `i3-ipc`), con
  soporte de eventos, árbol de contenedores y comandos. Sin `swaymsg` como
  dependencia obligatoria.
- **Iconos adaptativos**: usa Nerd Font si la tienes; si no, cae a símbolos
  Unicode que DejaVu/Noto sí dibujan (nada de cuadraditos).
- **Cero dependencias**: `go build` y listo. Binarios estáticos de ~3 MB.

## Capturas

| Panel de control | Portapapeles |
|---|---|
| ![Panel](docs/screenshot-panel-estado.png) | ![Portapapeles](docs/screenshot-portapapeles.png) |

| Cambio de tema | Calculadora |
|---|---|
| ![Temas](docs/screenshot-temas.png) | ![Calculadora](docs/screenshot-calculadora.png) |

| Galería de temas (vista previa en vivo) | Selector de fondos animados |
|---|---|
| ![Galería de temas](docs/screenshot-galeria-temas.png) | ![Selector de fondos](docs/screenshot-selector-fondos.png) |

| Escenas animadas disponibles | Paletas por categoría |
|---|---|
| ![Escenas](docs/screenshot-escenas.png) | ![Paletas](docs/screenshot-paletas.png) |

| Ventanas | Atajos |
|---|---|
| ![Ventanas](docs/screenshot-panel-ventanas.png) | ![Atajos](docs/screenshot-panel-atajos.png) |

---

## Instalación en Arch Linux

### Opción 1: instalador automático

```bash
git clone https://github.com/ShadowNigth/swaydesk.git
cd swaydesk
./scripts/install.sh --deps     # instala dependencias con pacman y compila
```

El script compila, instala en `~/.local/bin`, genera la configuración y avisa de
lo que falte. Con `PREFIX=/usr/local sudo ./scripts/install.sh` instala en el
sistema.

### Opción 2: manual

```bash
sudo pacman -S --needed go sway waybar foot mako wl-clipboard \
                          grim slurp swayidle swaylock cliphist \
                          ttf-jetbrains-mono-nerd
make && make install      # ~/.local/bin
swaydesk init             # escribe ~/.config/{sway,waybar,foot,mako,swaylock,swaydesk}
swaymsg reload            # recarga sway (o cierra y abre sesión)
```

Comprueba el resultado con:

```bash
swaydesk doctor
```

### Dependencias

| Obligatorio | Recomendado | Opcional |
|---|---|---|
| `sway`, `go` (solo para compilar) | `waybar`, `foot`, `mako`, `swaylock`, `grim`, `slurp`, `wl-clipboard`, `cliphist`, `swayidle` | `pavucontrol`, `playerctl`, `brightnessctl`, `nm-connection-editor`, `wf-recorder`, `ffmpeg` (vídeos de fondo), `swayfx` (blur y esquinas) |

Ninguna de las recomendadas es necesaria para que `swaydesk` funcione: cada
acción comprueba si su binario existe y se oculta del menú si no está.

---

## Uso

```
swaydesk · escritorio para Arch Linux + sway, escrito en Go

Configuración
  init [--force] [--only sway|waybar|foot|mako]   Escribe la configuración
  apply                     Regenera los archivos del tema y recarga waybar/sway
  theme [nombre]            Lista o cambia la paleta activa
  theme --gallery           Galería interactiva de temas con vista previa
  theme --from-wallpaper    Tema adaptativo a partir del fondo actual
  config                    Rutas y valores actuales

Fondos de pantalla
  wallpaper list [--json]   Escenas animadas y archivos encontrados
  wallpaper set <fondo>     scene:petals · gif:x.gif · image:x.png · video:x.mp4
  wallpaper gallery         Selector interactivo con vista previa animada
  wallpaper daemon|--stop   Demonio de fondo animado (live wallpaper)
  wallpaper play [fondo]    Reproduce el fondo a pantalla completa en la terminal
  wallpaper adaptive [img]  Genera y aplica un tema desde los colores del fondo
  wallpaper mode <modo>     fill · fit · stretch · center · tile

Sesión
  status [--json] [--module M]   Estado (sway, CPU, RAM, disco, batería…)
  doctor                         Diagnóstico de dependencias y configuración
  notify <título> [cuerpo]       Notificación de escritorio
  volume <up|down|mute>          Volumen
  screenshot [region|full]       Captura con grim
  lock | reload | power [acción] Sesión y energía
```

### Atajos por defecto

| Atajo | Acción | Atajo | Acción |
|---|---|---|---|
| `Super+Return` | Terminal | `Super+d` | Lanzador |
| `Super+Shift+d` | Lanzador + comandos | `Super+b` | Navegador |
| `Super+i` | Panel de control | `Super+c` | Portapapeles |
| `Super+p` | Menú de energía | `Super+t` | Galería de temas |
| `Super+Shift+t` | Galería de temas | `Super+Shift+w` | Selector de fondos |
| `Super+Shift+l` | Bloquear | `Print` | Captura de pantalla |
| `Super+q` | Cerrar ventana | `Super+f` | Pantalla completa |
| `Super+h/j/k/l` | Foco | `Super+Shift+hjkl` | Mover ventana |
| `Super+r` | Modo redimensionar | `Super+1..0` | Workspaces |
| `Super+Shift+1..0` | Mover al workspace | `Super+Shift+c` | Recargar sway |

La chuleta completa está en `Super+i` → *Atajos*, y en
[`assets/files/sway/config`](assets/files/sway/config).

### Lanzador

| Tecla | Acción |
|---|---|
| Escribir | Filtro difuso (tolera letras salteadas: `ffx` → Firefox) |
| `=` al principio | Calculadora: `=2^10`, `=sqrt(16)*3`, `=20*1.19` |
| `↑` `↓` / `Ctrl+p` `Ctrl+n` | Mover selección |
| `Enter` | Lanzar / copiar el resultado al portapapeles |
| `Esc` | Borrar filtro, o cerrar |

### Panel de control (`swayctl`)

`Tab` cambia de sección; `↑↓` navega; `Enter` actúa; `q` sale.

- **Estado** — versión de sway, ventana enfocada, workspaces, salidas, CPU, RAM,
  disco, temperatura, batería, audio, red y tiempo encendido.
- **Workspaces** — salta a cualquier workspace, con número de ventanas.
- **Ventanas** — `Enter` enfoca, `x` cierra, `f` alterna flotante,
  `m` pantalla completa, `n` envía al scratchpad.
- **Apariencia** — aplica cualquiera de las 8 paletas al escritorio completo.
- **Atajos** — referencia rápida de todos los binds.

---

## Configuración

Dos archivos, y solo uno es tuyo:

```
~/.config/swaydesk/config.toml   ← tus ajustes (se conserva al regenerar)
~/.config/sway/config            ← generado por swaydesk init
~/.config/sway/config.d/90-user.conf   ← tus overrides (incluido al final)
```

```toml
[general]
theme = "catppuccin"       # catppuccin, tokyonight, nord, gruvbox, dracula,
                           # kanagawa, rosepine, everforest
nerd_font = true           # false = símbolos Unicode
terminal = "foot"          # foot, kitty, alacritty, wezterm, ghostty, xterm, st
launcher = "swaymenu --float"
editor = "nvim"
file_manager = "thunar"
browser = "firefox"
confirm_power = true

[ui]
font = ""                  # vacío = detección automática
font_size = 11
bar_font_size = 13
width = 72                 # tamaño de la ventana flotante del lanzador
height = 20
border = "rounded"         # rounded, square, double, thick, ascii

[input]
keyboard_layout = "latam"

[wallpaper]
kind = "scene"             # scene · image · gif · video · none
name = "petals"            # escena procedural (ver "swaydesk wallpaper list")
path = ""                  # ruta para image/gif/video
mode = "fill"              # fill · fit · stretch · center · tile
fps = 30
protocol = "auto"          # auto · kitty · sixel · ansi (para la vista en terminal)
adaptive = false           # genera un tema desde los colores del fondo
dir = "~/Imágenes/Fondos"  # carpeta que explora el selector

[effects]
terminal_opacity = 0.94    # foot
bar_opacity = 0.86         # waybar
active_opacity = 1.00
inactive_opacity = 0.94
blur = false               # solo swayfx
gaps_inner = 6
gaps_outer = 8
rounding = 12              # solo swayfx
```

Cambiar de tema en caliente:

```bash
swaydesk theme gruvbox     # o Super+t / swaymenu --themes
```

Esto reescribe `waybar/style.css`, `foot/foot.ini`, `mako/config`,
`swaylock/config`, los colores de borde de sway (vía IPC) y avisa a waybar con
`SIGUSR2`. Nada se reinicia.

Fondos animados:

```bash
swaydesk wallpaper set scene:petals --live   # escena procedural + demonio
swaydesk wallpaper set ~/Imágenes/lofi.gif --live
swaydesk wallpaper set ~/Vídeos/4k.mp4 --live   # necesita ffmpeg
swaydesk theme --from-wallpaper             # paleta derivada del fondo
```

Más detalles en [`docs/PERSONALIZACION.md`](docs/PERSONALIZACION.md).

---

## Cómo está hecho

```
cmd/
  swaydesk/     CLI: init, apply, theme, status, doctor, notify, volume…
  swaymenu/     lanzador (búsqueda difusa + frecency + calculadora)
  swayctl/      panel de control multi-pestaña
  swayclip/     historial del portapapeles
internal/
  swayipc/      cliente IPC de sway escrito desde cero (protocolo i3-ipc)
  wayland/      cliente Wayland puro: xdg-shell, shm y wlr-layer-shell
  wallpaper/    motor de fondos: escenas, GIF, vídeo, protocolos kitty/sixel
  gallery/      selectores compartidos: galería de temas y de fondos
  adaptive/     paletas derivadas de imágenes (cuantización + HSL + contraste)
  ui/           toolkit TUI: estilos, colores, entrada, paneles, listas, decoración
  host/         métricas del sistema desde /proc y /sys
  launcher/     .desktop, PATH, terminales, historial
  actions/      energía, sesión, capturas, menús
  config/       parser TOML propio + ajustes
  setup/        generador de configuración (plantillas embebidas con embed)
  calc/         evaluador de expresiones
assets/files/   plantillas de sway, waybar, foot, mako, swaylock, .desktop
```

El paquete `ui` implementa lo necesario para una TUI: medición de ancho con
caracteres CJK y emoji, estilos con color de 24 bits que degradan a 256 colores,
lector de teclas con secuencias de escape, paneles con bordes, listas con filtro
difuso y bucle de pantalla alternativa con manejo de `SIGWINCH`.

El paquete `swayipc` habla el protocolo binario de sway (`i3-ipc` + longitud +
tipo + JSON) sobre el socket UNIX: enviar comandos, leer el árbol de
contenedores, listar workspaces/salidas/entradas y suscribirse a eventos.

Detalles de diseño en [`docs/ARQUITECTURA.md`](docs/ARQUITECTURA.md).

## Desarrollo

```bash
make build     # binarios en bin/
make test      # pruebas (incluye un servidor IPC de sway simulado)
make fmt vet   # formato y análisis estático
make dist      # binarios estáticos para amd64 y arm64
```

El proyecto no tiene dependencias: `go.mod` solo declara el módulo. Las pruebas
cubren el protocolo IPC con un servidor de sway simulado, el **protocolo
Wayland** (codificación de mensajes, alineación, secuencias kitty y sixel), el
**motor de fondos** (las 11 escenas, GIF animados, imágenes y vídeos), las
**paletas adaptativas**, el parser TOML, la búsqueda difusa, el cálculo, las
plantillas y el sistema de layout del toolkit.

## Solución de problemas

| Síntoma | Causa / solución |
|---|---|
| `no se encontró el socket de sway` | No estás dentro de una sesión de sway o `SWAYSOCK` no está exportado. Ejecuta `swaydesk doctor`. |
| El lanzador no abre ventana flotante | Falta un terminal soportado (`foot`, `kitty`, `alacritty`…). Revisa `terminal` en `config.toml`. |
| Cuadraditos en la barra | No hay Nerd Font instalada. Instala `ttf-jetbrains-mono-nerd` o pon `nerd_font = false`. |
| Waybar no cambia de color | Comprueba que waybar corre con el `style.css` generado: `pkill -SIGUSR2 waybar`. |
| `swaydesk init` no sobrescribe | Es intencionado: usa `--force` (crea respaldos `.bak-<fecha>`). |
| El fondo animado no aparece | Necesita un compositor con `wlr-layer-shell` (sway lo tiene). Comprueba `swaydesk wallpaper status` y que el demonio esté activo. |
| Un vídeo no se reproduce | Instala `ffmpeg`; sin él los `.mp4`/`.webm` no se pueden decodificar. |
| El teclado escribe mal los símbolos | Ajusta `[input] keyboard_layout` (`latam`, `us`, `es`, `de`…) y recarga sway. |

## Desinstalar

```bash
make uninstall                      # quita los binarios de ~/.local/bin
rm -rf ~/.config/swaydesk           # ajustes
# la configuración generada está en ~/.config/{sway,waybar,foot,mako,swaylock}
```

## Licencia

MIT. Consulta [LICENSE](LICENSE).
