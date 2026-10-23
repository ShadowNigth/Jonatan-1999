# 🦊 KitsuneDE — Escritorio para Arch Linux Estilo Anime en Go

**KitsuneDE** es un entorno de escritorio híbrido (**Tiling tipo Hyprland + Ventanas Flotantes**) y suite de *ricing* estilo **Anime** para **Arch Linux**, desarrollado íntegramente en **Go** (librería estándar pura, `0` dependencias externas, compilación en un único binario auto-contenido mediante `go:embed`).

---

## ✨ Características Principales

- **🐹 Motor 100% en Go (`kitsune-de` + `kitsune-ctl`)**:
  - Demonio de escritorio, servidor HTTP/IPC, implementación propia de **WebSockets RFC 6455** en Go estándar, colector de telemetría real de Linux desde `/proc` (`CPU por núcleo`, `RAM`, `Swap`, `Red`, `Disco`, `Procesos`), sesión interactiva de terminal y generador de *dotfiles*.
- **🌸 4 Temas Anime Multi-Estilo Intercambiables al Instante**:
  1. **`neon-tokyo`** (*Cyberpunk Neon Tokyo* — `ネオ東京 // 2099`): Lluvia nocturna neón cian/magenta.
  2. **`sakura-lofi`** (*Sakura Catppuccin* — `桜ローファイ // 宵`): Pétalos de cerezo flotantes y paleta Catppuccin Mocha.
  3. **`crimson-shonen`** (*Crimson Shonen* — `紅蓮の刃 // 月蝕`): Negro obsidiana, luna carmesí y brasas ascendentes.
  4. **`eva-midnight`** (*Midnight Evangelion* — `第3新東京市 // NERV`): Verde ácido EVA-01, ámbar táctico y violeta nocturno.
- **⊞ Gestor de Ventanas Híbrido (Tiling + Flotante + Monocle)**:
  - Mosaico automático estilo **Hyprland** (`Master-Stack / Dwindle`) con `gaps_in`, `gaps_out`, bordes luminosos y `blur` configurables en tiempo real.
  - Botón **`⧉` (`Alt+V`)** en cada ventana para desprenderla en modo **Flotante** libre redimensionable sin perder el mosaico del resto de ventanas, o arrastrar desde la barra de título.
  - **5 Espacios de Trabajo Kanji (`一 二 三 四 五`)**, barra superior estilo **Waybar**, lanzador **Rofi (`Alt+Space`)**, panel **SwayNC** y pantalla de bloqueo **Swaylock Anime (`Alt+L`)**.
- **🧰 Suite de Aplicaciones Integradas**:
  - **Kitty Anime Terminal**: Conectada por WebSocket al backend en Go con soporte ANSI, `fastfetch`, `kitsune-ctl`, `pacman` y comandos reales de Linux.
  - **Kitsune Rice Studio & Generador de Dotfiles Arch**: Personaliza temas/gaps en vivo y exporta configuraciones reales de `hyprland.conf`, `waybar/config.jsonc`, `waybar/style.css`, `kitty.conf`, `rofi/config.rasi` y `fastfetch/config.jsonc`.
  - **Btop++ System Monitor**: Gráficas de CPU por núcleo, RAM, red y procesos reales leídos por Go desde `/proc`.
  - **Yazi File Manager** + **Neovim Go Studio**: Navega y edita archivos reales del sistema con guardado directo.
  - **Kitsune Synth Radio**: Sintetizador WebAudio procedural Lo-Fi / Synthwave con analizador de espectro en vivo.
  - **Pacman & Arch Hub**: Buscador de paquetes de *ricing* e instrucciones de instalación nativa.

---

## 🚀 Instalación en Arch Linux

### Opción 1: Instalador Todo-en-Uno (`INSTALAR_EN_ARCH.sh` — desde el ZIP o desde GitHub)
```bash
# Si descargaste el ZIP:
unzip kitsune-de-arch-linux.zip
cd kitsune-de-arch-linux
chmod +x INSTALAR_EN_ARCH.sh
./INSTALAR_EN_ARCH.sh --theme neon-tokyo --with-dotfiles

# O clonando directamente desde GitHub:
git clone https://github.com/ShadowNigth/Jonatan-1999.git
cd Jonatan-1999
chmod +x INSTALAR_EN_ARCH.sh
./INSTALAR_EN_ARCH.sh --theme neon-tokyo --with-dotfiles
```

### Opción 2: Paquete nativo de Arch Linux (`PKGBUILD` + `makepkg`)
```bash
cd arch
makepkg -si
```
Una vez instalado, selecciona **`KitsuneDE (Anime Wayland)`** en tu gestor de inicio de sesión (`SDDM`, `GDM`, `LightDM`, `Greetd`) o ejecútalo directamente:
```bash
kitsune-de --kiosk --theme sakura-lofi
```

---

## 🛠️ Compilación y Ejecución Rápida con Go

```bash
# Ejecutar pruebas unitarias de todos los paquetes Go
make test

# Compilar kitsune-de y kitsune-ctl en ./build/
make build

# Iniciar el servidor del escritorio en http://0.0.0.0:8080
./build/kitsune-de --addr 0.0.0.0:8080
```

### Control en vivo mediante `kitsune-ctl` (CLI estilo `hyprctl`)
```bash
./build/kitsune-ctl status
./build/kitsune-ctl theme list
./build/kitsune-ctl theme set sakura-lofi
./build/kitsune-ctl theme next
./build/kitsune-ctl layout toggle
./build/kitsune-ctl workspace 2
./build/kitsune-ctl notify "¡Hola desde Go en Arch Linux!"
./build/kitsune-ctl export --theme eva-midnight --out ./dotfiles-arch
```

---

## ⌨️ Atajos de Teclado en el Escritorio

| Atajo | Acción |
| :--- | :--- |
| `Alt + Space` | Abrir lanzador de aplicaciones estilo **Rofi** |
| `Alt + Enter` | Abrir nueva terminal **Kitty** en mosaico |
| `Alt + T` | Alternar entre modo **Tiling**, **Flotante** y **Monocle** |
| `Alt + V` | Hacer flotante / devolver a mosaico la ventana enfocada |
| `Alt + Q` | Cerrar la ventana enfocada |
| `Alt + 1..5` | Cambiar entre los espacios de trabajo `一` a `五` |
| `Alt + L` | Bloquear pantalla con **Swaylock Anime** |

---

## 📂 Estructura del Proyecto en Go

```text
├── arch/                        # PKGBUILD, install.sh y sesiones Wayland/X11 para Arch Linux
├── cmd/
│   ├── kitsune-de/main.go       # Binario principal del escritorio y modo Kiosk nativo
│   └── kitsune-ctl/main.go      # CLI estilo hyprctl y exportador offline de dotfiles
└── internal/
    ├── config/state.go          # Estado concurrente y definición de los 4 temas Anime
    ├── fsapi/                   # API segura del explorador de archivos y editor Neovim
    ├── ricer/                   # Motor Go generador de configs Hyprland/Waybar/Kitty/Rofi
    ├── server/                  # Enrutador HTTP, endpoints IPC y bucle de telemetría
    ├── sysinfo/                 # Lector puro en Go de /proc (CPU, RAM, Red, Disco, Procesos)
    ├── terminal/                # Sesión interactiva por WebSocket + fastfetch + pacman
    ├── ui/                      # Assets estáticos embebidos en el binario con //go:embed
    └── ws/                      # Implementación propia RFC 6455 WebSocket + Hub broadcast
```
