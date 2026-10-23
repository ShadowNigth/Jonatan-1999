# 🦊 KitsuneDE `v1.2.0` — Escritorio Anime Visual-Twin para Arch Linux (SwayFX, Sway & Full Anime Shell) en Go

**KitsuneDE** (`v1.2.0`) es un entorno de escritorio híbrido (**Tiling / Sway Tabbed / Flotante**) estilo **Anime** para **Arch Linux**, desarrollado íntegramente en **Go** (`0` dependencias externas, cliente binario nativo **`sway-ipc` / `$SWAYSOCK`** y binario único auto-contenido con `//go:embed`).

Esta edición **Visual-Twin (`v1.2.0`)** instala **automáticamente todas las dependencias oficiales y de AUR** (tipografías japonesas Kanji, emojis, Nerd Fonts, Waybar, Dock flotante, animaciones de fondos a 60 FPS, desenfoque SwayFX y kiosco GPU Wayland) para que en tu Arch Linux real **se vea exactamente igual que en el ejemplo web, con todas sus animaciones y colores**.

---

## ✨ Dos Modos de Sesión Incluidos (100% Paridad Visual con el Ejemplo Web)

Al instalar **KitsuneDE**, tu pantalla de login (**SDDM, GDM, LightDM, greetd**) recibe ambas sesiones listas para elegir:

1. **🌟 `KitsuneDE Full Anime Shell (Idéntico al Web)` (`kitsune-de.desktop`)**
   - Arranca el motor Go `kitsune-de` sobre **Wayland GPU (`cage` + `chromium` Ozone Wayland)** a pantalla completa.
   - Se ve **100% píxel por píxel idéntico** a la demostración web: partículas flotantes Canvas a 60 FPS, efectos sonoros pentatónicos japoneses, animaciones *spring*, Waybar cristalina, Dock inferior con rebote, lanzador Rofi, Terminal PTY real conectada a tu `bash`/`zsh` de Arch, explorador de tus archivos reales, telemetría real de CPU/RAM y lanzamiento de apps nativas.
2. **🦊 `KitsuneDE + SwayFX (Visual-Twin Wayland)` (`kitsune-swayfx.desktop`)**
   - Sesión Wayland nativa sobre **SwayFX** replicando cada elemento visual del ejemplo web:
     - **Barra superior `Waybar`** con píldoras flotantes, workspaces Kanji (`一 二 三 四 五`), degradados neón, telemetría y selector de tema en 1 clic.
     - **Dock inferior flotante `nwg-dock`** con cristal esmerilado (`layer_effects "gtk-layer-shell" blur enable`) y brillo neón.
     - **Transiciones circulares a 60 FPS (`swww`)** al cambiar entre los 4 temas anime (`Super + T` o desde **Rice Studio**).
     - **Recarga en caliente multicolor desde Go (`SyncNativeDesktop`)**: cambiar de tema actualiza al instante bordes, gaps, radio de esquinas y `shadow_color` por `$SWAYSOCK`, recarga CSS de `Waybar` (`SIGUSR2`), cambia los colores ANSI de todas las terminales `Kitty` abiertas (`SIGUSR1`), actualiza `SwayNC` (`swaync-client -rs`) y sincroniza temas oscuros **GTK3 / GTK4**.

---

## 📦 Dependencias Instaladas Automáticamente por `INSTALAR_EN_ARCH.sh`

El instalador detecta automáticamente qué paquetes están en `[extra]` y auto-instala `yay-bin` si aún no tienes un helper AUR:

- **Compositor & Efectos Anime**: `swayfx` (`corner_radius`, `blur`, `shadows`, `layer_effects`), `sway` (respaldo oficial), `autotiling`, `cage`, `chromium`
- **Barra, Dock, Lanzador & Notificaciones**: `waybar`, `nwg-dock`, `rofi-wayland`, `swaync`, `swaylock-effects-git`, `libnotify`
- **Animaciones de Wallpapers a 60 FPS**: `swww`, `swaybg`
- **Tipografías Sin Cuadros Vacíos (Kanji + Iconos + Emojis)**: `ttf-jetbrains-mono-nerd`, `ttf-firacode-nerd`, `inter-font`, `noto-fonts`, `noto-fonts-cjk`, `noto-fonts-emoji`
- **Iconos, Cursores & GTK3/GTK4 Oscuro**: `papirus-icon-theme`, `bibata-cursor-theme-bin`, `gnome-themes-extra`, `nwg-look`
- **Apps & Utilidades Wayland**: `kitty`, `fastfetch`, `btop`, `yazi`, `neovim`, `thunar`, `pavucontrol`, `grim`, `slurp`, `wl-clipboard`, `polkit-gnome`, `xdg-desktop-portal-wlr`, `xdg-desktop-portal-gtk`

---

## 🌸 4 Temas Anime con Transición en Vivo

1. **`neon-tokyo`** (*Cyberpunk Neon Tokyo* — `ネオ東京 // 2099` — `#ff2e97` / `#00f0ff`)
2. **`sakura-lofi`** (*Sakura Catppuccin* — `桜ローファイ // 宵` — `#f5a9b8` / `#cba6f7`)
3. **`crimson-shonen`** (*Crimson Shonen* — `紅蓮の刃 // 月蝕` — `#ff3355` / `#ff9e3b`)
4. **`eva-midnight`** (*Midnight Evangelion* — `第3新東京市 // NERV` — `#a855f7` / `#4ade80`)

---

## 🚀 Descarga e Instalación en Arch Linux (`v1.2.0`)

```bash
curl -fLO https://github.com/ShadowNigth/Jonatan-1999/raw/v1.2.0/release/kitsune-de-sway-arch-linux.zip
unzip kitsune-de-sway-arch-linux.zip
cd kitsune-de-sway-arch-linux
chmod +x INSTALAR_EN_ARCH.sh

# Instala TODAS las dependencias (pacman + AUR), fuentes Kanji, temas y sesiones:
./INSTALAR_EN_ARCH.sh --compositor swayfx --theme neon-tokyo
```

---

## 🛠️ Uso de `kitsune-ctl`

```bash
# Cambiar al siguiente tema anime en vivo (anima wallpaper a 60fps + recarga Waybar/Kitty/SwayFX)
kitsune-ctl theme next
kitsune-ctl theme set sakura-lofi

# Alternar layout (tiling -> tabbed -> floating -> monocle)
kitsune-ctl layout tabbed

# Exportar los 17 dotfiles Visual-Twin en ~/.config
kitsune-ctl export --compositor swayfx --theme neon-tokyo --out ~/.config
```

---

## ⌨️ Atajos Principales

| Atajo en SwayFX (`$mod = Super`) | Atajo en Full Anime Shell / Web | Acción |
| :--- | :--- | :--- |
| `Super + T` | Botón Tema en Waybar | Cambiar tema Anime en vivo con animación 60 FPS |
| `Super + G` | Icono **Rice Studio** en Dock | Abrir **Kitsune Rice Studio** (temas, blur, gaps) |
| `Super + Space` | `Alt + Space` | Lanzador **Rofi** estilo Anime |
| `Super + Enter` | `Alt + Enter` | Nueva terminal **Kitty** |
| `Super + W` | `Alt + W` | Modo **Sway Tabbed** (pestañas) |
| `Super + Shift + Space` | `Alt + V` | Alternar ventana **Flotante / Tiling** |
| `Super + Q` | `Alt + Q` | Cerrar ventana enfocada |
| `Super + 1..5` | `Alt + 1..5` | Espacios de trabajo `一 二 三 四 五` |
| `Super + L` / `Super + X` | `Alt + L` | Bloquear pantalla (**Swaylock Anime**) |
