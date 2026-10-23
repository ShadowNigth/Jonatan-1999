# Arquitectura de swaydesk

Este documento explica cómo está organizado el proyecto, las decisiones de
diseño principales y cómo funciona el protocolo IPC de sway.

## Principios

1. **Solo biblioteca estándar.** El módulo no declara dependencias. El toolkit
   TUI, el parser TOML, el evaluador de expresiones y el cliente IPC están
   escritos a mano. Ventaja: `go build` funciona sin red, los binarios son
   pequeños y no hay cadena de suministro que auditar.
2. **Un binario por tarea.** Nada de un demonio monolítico: `swaymenu` arranca y
   muere en milisegundos, `swayctl` vive mientras miras el panel. Waybar llama a
   `swaydesk status --module X` solo cuando necesita un dato.
3. **La configuración del usuario nunca se pisa.** Los archivos generados se
   respaldan (`*.bak-<fecha>`) y los ajustes propios viven en
   `~/.config/swaydesk/config.toml`, que se conserva entre regeneraciones.
4. **Degradación elegante.** Sin Nerd Font → símbolos Unicode. Sin
   `notify-send` → silencio. Sin `cliphist` → se usa `wl-paste`. Sin sway →
   mensaje claro en lugar de pánico.

## Mapa de paquetes

```
cmd/swaydesk     CLI principal y orquestación (init/apply/theme/status/doctor)
cmd/swaymenu     lanzador: busca, puntúa y lanza
cmd/swayctl      panel de control: 5 pestañas y acciones sobre sway
cmd/swayclip     portapapeles: lista, filtra, copia, borra
cmd/swaybar      barra de estado: CLI, recogida de datos y bucle de dibujo

internal/wayland cliente Wayland puro: conexión, registry, xdg-shell, shm y
                 wlr-layer-shell (capa background para el fondo animado)
internal/wallpaper motor de fondos de pantalla
  scenes.go      11 escenas procedurales sobre un Canvas RGB
  gif.go         carga de GIF animado (fotogramas ya compuestos)
  video.go       decodificación con ffmpeg → tramas RGB24 en memoria
  render.go      protocolos kitty y sixel, PNG, escalado y recorte
  engine.go      Spec/Engine: una sola API para escena, imagen, GIF y vídeo
  run.go         reproducción en terminal y demonio sobre layer-shell
  daemon.go      PID, arranque/parada y tamaño de la salida vía IPC
internal/gallery selectores compartidos (galería de temas y de fondos)
internal/boot    animación de arranque: logotipo en bloques con degradado,
                 partículas por estilo, pasos en vivo y barra de progreso
internal/adaptive paletas derivadas de imágenes: cuantización, HSL, contraste
internal/bar     barra de estado dibujada en píxeles
  canvas.go      lienzo RGBA: rectángulos, esquinas redondeadas, degradados,
                 texto, escalado y composición sobre el fondo
  render.go      Snapshot de datos → tres islas (izquierda/centro/derecha) con
                 separadores, colores del tema y recorte según el espacio
internal/font    fuentes de mapa de bits
  psf.go         PSF1/PSF2, con gzip y tabla Unicode opcional
  builtin.go     fuente 5x7 incorporada (ASCII + símbolos de la barra)
  font.go        cara activa: Best/Builtin/Load/LoadSystem y escalado
  draw.go        volcado de texto sobre un lienzo RGBA

internal/ui      toolkit TUI (lo más grande del proyecto)
  width.go       ancho de celdas (CJK, emoji, combinantes), truncado, wrap
  style.go       colores 24-bit → 256, estilos, bordes, JoinHorizontal
  term.go        modo crudo, pantalla alternativa, tamaño, dibujado de frames
  input.go       decodificación de teclas y secuencias de escape
  theme.go       tema, iconos Nerd Font / Unicode
  palettes.go    31 paletas por categoría con registro dinámico
  decor.go       degradados, muestras, ventana de ejemplo, cajas
  imageview.go   imágenes en terminal (kitty/sixel/medios bloques)
  fuzzy.go       emparejamiento difuso con puntuación
  panel.go       marcos con título y pie, pistas de teclado, scrollbar
  select.go      lista seleccionable con filtro y modo múltiple
  screen.go      ciclo de vida de la pantalla interactiva
  loop.go        bucle dibujar → tecla → redibujar

internal/swayipc cliente del protocolo i3-ipc (ver más abajo)
internal/host    CPU, memoria, disco, red, audio, batería, temperatura
internal/launcher .desktop, PATH, emuladores de terminal, historial
internal/actions energía, bloqueo, capturas, notificaciones, menús comunes
internal/config  parser TOML y ajustes tipados
internal/setup   plantillas embebidas → archivos del sistema
internal/calc    evaluador de expresiones (descenso recursivo)
assets/files     plantillas con marcadores {{CLAVE}}
```

## La barra propia

```
sway IPC ──eventos──▶ cmd/swaybar/run.go ──Snapshot──▶ internal/bar/render.go
   │                        │                               │
   └── estado ──────────────┘                               ▼
   host/playerctl ── métricas                        Canvas RGBA (píxeles)
                                                           │
                              wlr-layer-shell ◀── shm ◀─────┘
```

- **Una sola suscripción**: `watchSway` escucha `workspace`, `window::focus`,
  `window::title` y `mode` por IPC; los eventos despiertan el bucle de dibujo en
  lugar de sondear sway cada segundo.
- **Sin toolkit gráfico**: la superficie es un `wl_shm` que se rellena a mano.
  `internal/wayland.Panel` envuelve `zwlr_layer_shell_v1` (anclas, zona
  exclusiva, `ack_configure`, redimensionado).
- **Sin fuentes externas obligatorias**: si no hay ningún `.psf` en el sistema,
  se usa la fuente 5×7 embebida; así la barra funciona en una instalación limpia.
- **Degradación**: si con todos los módulos el centro no cabe, se quita el
  reproductor y después el título antes que solapar islas.

## Flujo de `swaydesk init`

```
config.Load()                        # ~/.config/swaydesk/config.toml
  └─ setup.Render(cfg)               # aplica tokens a las plantillas
       ├─ detecta fuente con fc-list
       ├─ resuelve la paleta del tema
       ├─ elige iconos según nerd_font
       └─ genera sway/config, waybar/*, foot/foot.ini, mako/config,
          swaylock/config, .desktop y swaydesk/config.toml
setup.Install(cfg, dest, force)      # escribe con respaldo si hace falta
```

Las plantillas usan marcadores `{{THEME_BG_HEX}}`, `{{FONT}}`, `{{TERMINAL}}`…
`setup.Render` los sustituye en una sola pasada con `strings.NewReplacer`. Una
prueba verifica que no quede ningún `{{` sin sustituir.

`swaydesk apply` es la variante en caliente: regenera solo los archivos
dependientes del tema (waybar, foot, mako, swaylock) y además

- envía `SIGUSR2` a waybar (recarga el CSS sin reiniciar),
- aplica los colores de borde a sway en vivo mediante IPC
  (`client.focused …`, `focused_border_color …`).

## Protocolo IPC de sway

Sway implementa el protocolo de i3. Cada mensaje es:

```
┌──────────┬──────────────────┬──────────────────┬──────────────────┐
│ "i3-ipc" │ longitud (u32 LE)│ tipo (u32 LE)    │ payload JSON     │
│ 6 bytes  │ 4 bytes          │ 4 bytes          │ N bytes          │
└──────────┴──────────────────┴──────────────────┴──────────────────┘
```

sobre un socket UNIX (`$SWAYSOCK`, o `sway --get-socketpath`).

Tipos usados por swaydesk:

| Tipo | Nombre | Uso |
|---|---|---|
| 0 | `RUN_COMMAND` | ejecutar comandos (`[con_id=N] kill`, `workspace 2`…) |
| 1 | `GET_WORKSPACES` | lista de workspaces con estado y salida |
| 2 | `SUBSCRIBE` | eventos (`workspace`, `window`, `mode`, `tick`…) |
| 3 | `GET_OUTPUTS` | monitores con resolución y workspace actual |
| 4 | `GET_TREE` | árbol completo de contenedores |
| 7 | `GET_VERSION` | versión y archivo de configuración cargado |
| 12 | `SYNC` | sincronización del bucle de eventos |
| 100 / 101 | `GET_INPUTS` / `GET_SEATS` | dispositivos (extensiones de sway) |

Los eventos viajan por la misma conexión suscrita y llevan el bit
`0x80000000` puesto en el tipo.

Implementación en `internal/swayipc`:

- `Conn` serializa las peticiones con un mutex (el protocolo no permite
  entrelazar respuestas), aplica un `deadline` configurable y valida la cabecera
  con `ErrBadMagic` si el socket no es de sway.
- `Node.Walk` recorre el árbol; `Leaves()` devuelve ventanas y `FindFocused()`
  la ventana activa.
- `decodeTree` hace una segunda pasada sobre el JSON para recoger campos
  variables (`app_id`, `window_properties.class`, `title`) que sway y X11
  exponen de forma distinta.
- Un servidor falso en `ipc_test.go` (socket UNIX reales + frames reales)
  verifica el protocolo de punta a punta, incluidos los eventos.

## El toolkit TUI

**Ancho de celdas.** `ui.Width` ignora secuencias ANSI y aplica una tabla
reducida de `wcwidth`: 0 para combinantes y selectores de variación, 2 para CJK
y emoji, 1 para el resto. Sin esto, cualquier texto con acentos combinantes o
ideogramas rompe la alineación de los marcos.

**Dibujado.** Cada aplicación calcula un `string` de líneas y `Terminal.Draw`
mueve el cursor a casa, escribe cada línea con `\x1b[K` (borrar hasta el final)
y termina con `\x1b[J`. Es un enfoque de "frame completo": simple, sin estado de
diffs y suficientemente rápido a 2–10 Hz. Se oculta el cursor y se usa la
pantalla alternativa (`\x1b[?1049h`), restaurada con `defer sc.Close()`.

**Teclado.** `KeyReader` traduce bytes a `Key{Rune, Name, Ctrl, Alt}`:
- caracteres UTF-8 completos (`ñ`, `→`),
- `Ctrl+a`…`Ctrl+z` (bytes 0x01–0x1a),
- secuencias `CSI` (`\x1b[A`, `\x1b[1;5C` = Ctrl+flecha, `\x1b[3~` = Supr),
- `SS3` (`\x1bOP` = F1),
- y distingue `Esc` solo (timeout de 35 ms) de `Alt+tecla`.

**Colores.** Las paletas se declaran en `#rrggbb`. En `truecolor` se emiten
`\x1b[38;2;r;g;bm`; si el terminal solo soporta 256 colores se aproxima al índice
xterm-256 más cercano comparando con el cubo 6×6×6 y la rampa de grises; si
`NO_COLOR` está definido o `TERM=dumb`, todo se degrada a texto plano.

**Búsqueda difusa.** `Fuzzy("chrm", "Chromium")` comprueba subsecuencia y puntúa:
+20 al inicio, +15 tras separador, +12 por caracteres consecutivos, −2 por hueco,
+200 por coincidencia exacta. `FilterItems` puntúa cada campo y ordena estable.

## Frecency del lanzador

`launcher.History` guarda `clave<TAB>usos<TAB>timestamp` en
`~/.cache/swaydesk/history` (texto plano, inspeccionable a mano). La puntuación
es `usos × decaimiento`, con decaimiento `100/(1+días/3)` limitado a 5, de modo
que lo usado ayer gana a lo usado cien veces hace un mes.

## Decisiones y compromisos

| Decisión | Motivo |
|---|---|
| Sin dependencias de Go | Compilación sin red, binarios pequeños, cero riesgo de módulos comprometidos. |
| TUI en lugar de GTK/Qt | Coherente con sway, rápido en cualquier hardware, se puede probar en un pty. |
| Plantillas embebidas con `embed` | El binario es autosuficiente: se puede instalar sin el código fuente. |
| `syscall` directo para modo crudo | Evita `golang.org/x/term`; el struct `Termios` de Linux es estable. |
| Fondo animado en un demonio aparte | Es el único proceso que necesita Wayland nativo; si el compositor no lo soporta, sway sigue mostrando un fotograma estático. |
| Opacidad en las plantillas, no por IPC | sway no permite cambiar el alpha de una ventana ya abierta; escribir `for_window` al generar la configuración es lo único fiable. |
| Escenas procedurales en vez de vídeos de regalo | Sin binarios pesados en el repositorio y adaptadas a la paleta activa. Los GIF y vídeos del usuario se reproducen igual. |
| `blur`/`rounding` solo en swayfx | Sway no implementa desenfoque ni esquinas; se detecta el compositor y las líneas quedan comentadas si no aplica. |

## Pruebas

Además de los tests del toolkit y de los protocolos, `internal/bar` y
`internal/font` tienen sus propias suites: colores y composición alfa, esquinas
redondeadas, medidas de texto, elección de variantes cuando falta espacio, y
carga de PSF1/PSF2 sintéticos (con gzip y tabla Unicode) incluyendo archivos
corruptos.

```bash
go test ./...          # incluye decenas de miles de comprobaciones internas
go test -race ./...    # IPC y bucle de teclado concurrentes
```

Cobertura destacada:

- `swayipc`: frames binarios, magic inválido, payloads, con/sin argumentos,
  comandos fallidos, árbol, eventos y suscripción.
- `ui`: anchos CJK/emoji, truncado, wrap de palabras largas, diff de secuencias
  de teclas, dimensiones exactas de los paneles, `SGR` en 3 perfiles de color,
  validación de las 8 paletas.
- `config`: TOML (comentarios, arrays, comillas simples, errores), aplicación de
  valores y `SaveTheme` conservando el resto del archivo.
- `setup`: render sin marcadores, colores por tema, iconos según Nerd Font,
  instalación con respaldos y `apply` sobre directorio vacío.
- `launcher`: `.desktop` (localización, `NoDisplay`, `Hidden`), `Exec` con
  comillas y códigos `%f`, historial persistente y recortado, `PATH`.
- `wayland`: cabeceras de mensaje, alineación a 4 bytes de strings, decodificación
  de eventos, descriptores y detección de compositor sin servidor.
- `wallpaper`: las 11 escenas (tamaño, búfer y no-negro), `ParseSpec` con sus
  formatos y errores, ciclo de vida del motor para escena e imagen, GIF animado
  de dos fotogramas, protocolos kitty (troceado) y sixel, vista previa y escalado.
- `adaptive`: paleta generada desde una imagen tris-color, colores válidos,
  gradiente, guardado en TOML y error con archivo inexistente.
- `boot`: estilos válidos, logotipo rectangular (todas las filas del mismo
  ancho, imprescindible para centrarlo), partículas acotadas a la pantalla,
  fotogramas completos a cualquier progreso y barra con su porcentaje.
- `config`: valores por defecto de `[boot]` y `[effects]`, y carga de la
  sección completa desde un archivo.
