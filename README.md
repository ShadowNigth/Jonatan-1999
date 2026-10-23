# 🦊 KitsuneDE — Escritorio Anime para Arch Linux (Sway, SwayFX & Wayland) en Go

**KitsuneDE** (`v1.1.0`) es un entorno de escritorio híbrido (**Tiling / Tabbed / Flotante**) y suite de *ricing* estilo **Anime** para **Arch Linux** diseñado especialmente para **Sway** y **SwayFX** (además de perfil opcional Hyprland), desarrollado íntegramente en **Go** (librería estándar pura, `0` dependencias externas, cliente binario nativo **`sway-ipc` / `$SWAYSOCK`** y binario único auto-contenido con `//go:embed`).

---

## ✨ Novedades de la Edición Sway & SwayFX (`v1.1.0`)

- **󰖲 Integración Nativa en Go con `$SWAYSOCK` (`internal/swayipc`)**:
  - Implementación propia del protocolo binario **`i3-ipc` / `sway-ipc`** sobre sockets UNIX en Go puro.
  - Cuando estás dentro de una sesión real de **Sway** o **SwayFX** en Arch Linux, ejecutar `kitsune-ctl theme next` o mover los *sliders* de **Rice Studio** aplica al instante `gaps inner`, `gaps outer`, `client.focused` y (en **SwayFX**) `corner_radius`, `blur enable` y `shadows enable` sin reiniciar tu sesión.
- **✨ Soporte Dual Sin Errores (`SwayFX` + `Sway` Puro)**:
  - **`swayfx/config` y `sway/swayfx-effects.conf`**: Incluye todo el *eye-candy* anime de **SwayFX** (`corner_radius`, `blur_passes 3`, `shadows`, `default_dim_inactive` y `layer_effects` para Waybar, Rofi y notificaciones).
  - **`sway/config`**: 100% compatible con **`extra/sway`** oficial de Arch Linux sin directivas inválidas ni barras rojas de error, detectando automáticamente en `kitsune-sway-session` qué binario tienes instalado.
  - **Configuraciones completas acompañantes**: `waybar/config-sway.jsonc` (con `sway/workspaces`, `sway/mode` y `sway/window`), `swaylock/config` (`swaylock-effects`), `swaync/style.css`, `kitty/kitty.conf`, `rofi/config.rasi` y `fastfetch/config.jsonc`.
- **🗂️ Contenedores `Tabbed` de Sway (`Alt+W` / `$mod+w`) + Mosaico + Flotante**:
  - Alterna en vivo entre **Sway Split Tiling**, **Sway Tabbed (barra de pestañas anime)**, **Flotante** y **Monocle**.
- **🌸 4 Temas Anime Incluidos**:
  1. **`neon-tokyo`** (*Cyberpunk Neon Tokyo* — `ネオ東京 // 2099`)
  2. **`sakura-lofi`** (*Sakura Catppuccin* — `桜ローファイ // 宵`)
  3. **`crimson-shonen`** (*Crimson Shonen* — `紅蓮の刃 // 月蝕`)
  4. **`eva-midnight`** (*Midnight Evangelion* — `第3新東京市 // NERV`)

---

## 🚀 Descarga e Instalación Rápida desde GitHub Release (`v1.1.0`)

```bash
curl -fLO https://github.com/ShadowNigth/Jonatan-1999/raw/v1.1.0/release/kitsune-de-sway-arch-linux.zip
unzip kitsune-de-sway-arch-linux.zip
cd kitsune-de-sway-arch-linux
chmod +x INSTALAR_EN_ARCH.sh

# Para SwayFX (con blur, esquinas redondeadas y sombras anime):
./INSTALAR_EN_ARCH.sh --compositor swayfx --theme neon-tokyo

# O para Sway estándar (solo repositorios oficiales extra/ de Arch):
./INSTALAR_EN_ARCH.sh --compositor sway --theme sakura-lofi
```

También puedes instalarlo como paquete nativo de Arch Linux con `makepkg`:
```bash
cd arch
makepkg -si
```
En tu pantalla de inicio de sesión (**SDDM, GDM, LightDM, Greetd**) aparecerán las sesiones:
- **`KitsuneDE + SwayFX (Anime Wayland)`**
- **`KitsuneDE + Sway (Anime Wayland)`**
- **`KitsuneDE (Anime Wayland)`** *(Shell interactivo en Go)*

---

## 🛠️ Uso de `kitsune-ctl` y `swaymsg`

```bash
# Cambiar perfil entre swayfx, sway y hyprland
kitsune-ctl compositor swayfx
kitsune-ctl compositor sway

# Cambiar tema anime en caliente (sincroniza UI + $SWAYSOCK)
kitsune-ctl theme next
kitsune-ctl theme set sakura-lofi

# Alternar layout (tiling -> tabbed -> floating -> monocle)
kitsune-ctl layout tabbed

# Exportar todos los dotfiles de Sway y SwayFX en cualquier carpeta
kitsune-ctl export --compositor swayfx --theme eva-midnight --out ~/.config
```

---

## ⌨️ Atajos Principales (en Sway/SwayFX y en el Escritorio Go)

| Atajo en Sway (`$mod = Super`) | Atajo en Preview Web | Acción |
| :--- | :--- | :--- |
| `Super + Space` | `Alt + Space` | Lanzador **Rofi** estilo Anime |
| `Super + Enter` | `Alt + Enter` | Nueva terminal **Kitty** |
| `Super + W` | `Alt + W` | Modo **Sway Tabbed** (pestañas) |
| `Super + A` / `Super + E` | `Alt + T` | Alternar modo de distribución de ventanas |
| `Super + Shift + Space` | `Alt + V` | Alternar ventana **Flotante / Tiling** |
| `Super + Q` | `Alt + Q` | Cerrar ventana enfocada |
| `Super + 1..5` | `Alt + 1..5` | Espacios de trabajo `一 二 三 四 五` |
| `Super + X` | `Alt + L` | Bloquear pantalla (**Swaylock Anime**) |
| `Super + T` | Botón Tema Waybar | Siguiente tema Anime (`kitsune-ctl theme next`) |
