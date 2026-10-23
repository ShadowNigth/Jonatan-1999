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
cmd/swaynotify   daemon de notificaciones: D-Bus, capa overlay y entrada
cmd/swayosd      avisos en pantalla (volumen) con fundido, en capa overlay

internal/wayland cliente Wayland puro: conexión, registry, xdg-shell, shm y
                 wlr-layer-shell (capa background para el fondo animado)
internal/wallpaper motor de fondos de pantalla
  scenes.go      11 escenas procedurales sobre un Canvas RGB
  gif.go         carga de GIF animado (fotogramas ya compuestos)
  video.go       decodificación con ffmpeg → tramas RGB24 en memoria
  render.go      protocolos kitty y sixel, PNG, escalado y recorte
  engine.go      Spec/Engine: una sola API para escena, imagen, GIF y vídeo
  workspace.go   reglas workspace → fondo (nombre, sufijo tras ':' y número)
                 y transiciones: tipos, curva suave y mezcla de dos lienzos
  run.go         reproducción en terminal y demonio sobre layer-shell
  daemon.go      PID, arranque/parada, tamaño de la salida vía IPC y el bucle
                 que cambia de fondo al cambiar de workspace
internal/gallery selectores compartidos (galería de temas y de fondos)
internal/boot    animación de arranque: logotipo en bloques con degradado,
                 partículas por estilo, pasos en vivo y barra de progreso
internal/adaptive paletas derivadas de imágenes: cuantización, HSL, contraste
internal/bar     barra de estado dibujada en píxeles
  canvas.go      lienzo RGBA: rectángulos, esquinas redondeadas, degradados,
                 texto, escalado y composición sobre el fondo
  render.go      Snapshot de datos → tres islas (izquierda/centro/derecha) con
                 separadores, colores del tema y recorte según el espacio
internal/osd     avisos en pantalla (OSD)
  osd.go         estado del aviso: valor, silencio, dispositivo, claves
  render.go      tarjeta con etiqueta, valor grande y barra segmentada,
                 con alfa variable para los fundidos
internal/notify  notificaciones de escritorio
  notify.go      modelo, caducidad, modo «no molestar» e historial en JSON
  render.go      tarjetas en píxeles: urgencias, acciones, progreso, cuenta
                 atrás, pilas plegadas con capas y franjas de grupo, y la
                 miniatura cuadrada cuando el aviso trae imagen
  image.go       decodificación de image-data (crudo RGBA con salto de fila) y
                 de archivos PNG/JPEG, escalado por caja y recorte al centro
  center.go      modelo del centro: filtro sin acentos, orden, pilas y selección
  group.go       agrupación por aplicación o etiqueta de pila, recorte por
                 grupos y estado de plegado
  service.go     servidor y cliente de org.freedesktop.Notifications
internal/dbus    cliente y servidor D-Bus en Go puro
  message.go     formato binario: firmas, alineación, arrays, variantes, dicts
  conn.go        handshake SASL EXTERNAL, autobús de sesión, llamadas y señales
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

## Los avisos en pantalla

```
wpctl / pactl / amixer            swayosd
        │  estado del altavoz         │
        ▼                             ▼
   internal/host ──── Audio ──── internal/osd.Renderer
                                      │
                                      ▼
                             wl_surface (capa overlay)
```

- **Un proceso por aviso**: `swayosd show` dibuja el aviso, espera
  `timeout_ms` y termina; así el aviso desaparece solo aunque la sesión esté
  cargada, sin dejar una superficie viva toda la jornada.
- **`swayosd watch`** es el único proceso permanente: consulta el altavoz por
  defecto cada 300 ms y, cuando cambia el valor, lanza el aviso. Cubre los
  cambios de las teclas multimedia, de los auriculares Bluetooth y de
  cualquier aplicación.
- **Fundido sin estado**: la animación son fotogramas con alfa creciente o
  decreciente sobre la misma tarjeta; `fade_ms = 0` la desactiva.
- **Clic pasante**: la región de entrada de la superficie está vacía, así que
  el aviso no intercepta nunca el ratón.

## Las notificaciones

```
app (Chromium, pacman…)              swaynotify
        │  D-Bus Notify                    │
        ▼                                  ▼
  internal/dbus ◀── autobús de sesión ─▶ Store ──▶ Renderer (píxeles)
                                            │            │
                                            │            ▼
                                     historial JSON   wl_surface (overlay)
```

- **D-Bus a mano**: el protocolo entra en el proyecto como cualquier otro —
  formato binario con firmas, sin libdbus. Se implementan cliente y servidor,
  así que un test puede hablar con el daemon por un socket UNIX real.
- **Repintado por eventos**: el servicio avisa al bucle con un canal cuando
  cambia el estado; el bucle solo redibuja si cambia el resumen del contenido
  (y una vez por segundo por la cuenta atrás).
- **Pilas y clics**: el renderizador devuelve por cada tarjeta qué
  notificaciones representa (`IDs`), si resume una pila plegada y si la franja
  de un grupo está abierta; el daemon usa eso para decidir entre cerrar,
  plegar, desplegar o cerrar el grupo entero con el botón derecho.
- **Superficie ajustada al contenido**: cuando hay más o menos tarjetas se pide
  un tamaño nuevo al compositor y se espera su confirmación; mientras tanto no
  se pinta, para no estirar las tarjetas.
- **Entrada selectiva**: la región de entrada solo cubre las tarjetas, así que
  el resto de la franja deja pasar el ratón a las ventanas de debajo.
- **Sin mako**: el autostart arranca `swaynotify` si `notify.enabled` y si no
  hay otro servidor; en caso contrario, `mako`.
- **Autostart idempotente**: sway lanza `autostart.sh` con `exec_always`, es
  decir, también en cada `swaymsg reload`. El script arranca cada servicio con
  `once` (solo si `pgrep -x` no lo encuentra) y lo que es de una sola vez por
  sesión (animación de arranque, bienvenida) con `una_vez`, cuya marca lleva el
  `SWAYSOCK` del arranque. Resultado: recargar no duplica la barra ni los
  demonios, y un servicio caído se recupera al recargar.

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
| D-Bus implementado a mano | Es el único camino para sustituir a mako/dunst: sin libdbus ni cgo, un binario estático más y un protocolo que se puede probar con sockets de verdad. |

## Pruebas

Además de los tests del toolkit (`internal/ui`), de los protocolos
(`internal/swayipc`, `internal/wayland`) y de los fondos
(`internal/wallpaper`), hay suites propias para:

- **`internal/setup`** — los atajos: análisis de `bindsym`/`bindcode` con la
  misma expansión de variables que hace sway (una sola pasada, sin anidar),
  bloques `mode` separados del modo normal, detección de teclas repetidas y de
  programas que no existen, y `GET_CONFIG` por IPC para auditar la configuración
  que la sesión tiene cargada de verdad.
- **`internal/dbus`** — ida y vuelta de mensajes con todos los tipos del
  formato (arrays de cadenas y bytes, estructuras, diccionarios con variantes),
  validación de firmas, y un autobús de pruebas sobre socket UNIX que verifica
  el saludo, las llamadas, los errores y las respuestas.
- **`internal/notify`** — la agrupación: clave por aplicación o etiqueta de
  pila (normalizada, así `Marta` y `marta` son la misma), orden de los grupos,
  vista previa del plegado, recorte de `max_visible` contando grupos completos,
  plegado y despliegue sin perder la selección, y las marcas que devuelve el
  dibujo (`Collapsed`, `Header`, `IDs`) para que el daemon sepa si un clic debe
  cerrar una tarjeta, plegar una pila o abrirla.
- **`internal/notify`** — las imágenes: decodificación de `image-data` con
  salto de fila y canal alfa, rechazo de tamaños desmedidos (lado, bytes y
  hint), escalado y recorte cuadrado, el paso de `image-path` a imagen ya
  cargada en la tarjeta, y el historial que guarda la ruta pero no los píxeles.
- **`internal/notify`** — el modelo del centro (vivas primero, sin duplicar lo
  que ya está en pantalla, filtro que ignora mayúsculas y acentos, selección y
  recuento por urgencia), el saneado de respuestas (`SanitizeReply`: una línea,
  sin caracteres de control, recorte por runas), y la respuesta rápida de
  extremo a extremo: el servicio emite `NotificationReplied` **solo al
  remitente** y cierra la tarjeta salvo que sea `resident`.
- **`internal/notify`** — alta, sustitución por `replaces_id` y caducidad,
  modo «no molestar» con excepción para las urgentes, historial en disco,
  marcado limitado del protocolo (etiquetas y entidades), ajuste de texto en
  líneas y el layout de las tarjetas (sin solapes, transparencia fuera de
  ellas, acciones que no caben). Incluye una prueba de extremo a extremo con
  un cliente y el servicio hablando D-Bus de verdad.
- **`internal/osd`** — acotado del valor, relleno de la barra (el 100% llena;
  por encima cambia de color), textos del aviso, tamaños según la escala y
  fundido: con alfa 0 no queda ni un píxel encendido y con alfa intermedia el
  fondo es menos opaco que a plena intensidad.
- **`internal/bar`** — colores, composición alfa, esquinas redondeadas,
  medidas de texto, elección de variantes cuando falta espacio y zonas
  pulsables (el indicador de notificaciones se publica como `Region` y
  `RegionAt` resuelve el clic del ratón).
- **`internal/font`** — PSF1/PSF2 sintéticos con gzip y tabla Unicode,
  archivos corruptos y la fuente incorporada.

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
  de dos fotogramas, protocolos kitty (troceado) y sixel, vista previa y escalado;
  y —nuevo— las reglas por workspace (precedencia nombre → sufijo → número,
  normalización, edición y orden de las entradas) y las transiciones (nombres en
  español e inglés, duración acotada, progreso y curva de suavizado, las cuatro
  mezclas con sus píxeles esperados y el reescalado cuando los fondos tienen
  tamaños distintos).
- `adaptive`: paleta generada desde una imagen tris-color, colores válidos,
  gradiente, guardado en TOML y error con archivo inexistente.
- `boot`: estilos válidos, logotipo rectangular (todas las filas del mismo
  ancho, imprescindible para centrarlo), partículas acotadas a la pantalla,
  fotogramas completos a cualquier progreso y barra con su porcentaje.
- `config`: valores por defecto de `[boot]` y `[effects]`, y carga de la
  sección completa desde un archivo.
