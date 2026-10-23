#!/usr/bin/env bash
# ==============================================================================
# 🦊 KitsuneDE v1.1.0 — Instalador Todo-en-Uno para Sway, SwayFX & Arch Linux
# Funciona tanto desde el ZIP precompilado como clonando desde GitHub.
# ==============================================================================
set -euo pipefail

COMPOSITOR="swayfx"
THEME="neon-tokyo"
WITH_DOTFILES=1
SKIP_PACMAN=0
FORCE_BUILD=0

usage() {
  cat <<EOF
🦊 KitsuneDE — Instalador para Sway, SwayFX y Arch Linux (Motor Go)

Uso:
  ./INSTALAR_EN_ARCH.sh [opciones]

Opciones:
  --compositor <wm>   Compositor a configurar: swayfx (default) | sway | hyprland
  --theme <id>        Tema anime inicial (default: neon-tokyo)
                      Opciones: neon-tokyo | sakura-lofi | crimson-shonen | eva-midnight
  --with-dotfiles     Instalar configs en ~/.config con respaldo automático .bak (activo por defecto)
  --no-dotfiles       Instalar solo los binarios Go y sesiones de escritorio sin tocar ~/.config
  --build             Forzar compilación desde el código fuente en Go
  --no-pacman         Omitir instalación de paquetes vía pacman / yay
  -h, --help          Mostrar esta ayuda
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --compositor|-c)
      COMPOSITOR="${2:-swayfx}"
      shift 2
      ;;
    --theme|-t)
      THEME="${2:-neon-tokyo}"
      shift 2
      ;;
    --with-dotfiles)
      WITH_DOTFILES=1
      shift
      ;;
    --no-dotfiles)
      WITH_DOTFILES=0
      shift
      ;;
    --build)
      FORCE_BUILD=1
      shift
      ;;
    --no-pacman)
      SKIP_PACMAN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Opción no reconocida: $1"
      usage
      exit 1
      ;;
  esac
done

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${ROOT_DIR}"

echo "╭──────────────────────────────────────────────────────────────╮"
echo "│  🦊 KitsuneDE // Instalador Sway & SwayFX para Arch Linux    │"
echo "╰──────────────────────────────────────────────────────────────╯"
echo "  • Compositor elegido : ${COMPOSITOR}"
echo "  • Tema Anime inicial : ${THEME}"

ARCH_CPU="$(uname -m)"
BIN_DE=""
BIN_CTL=""

# 1. Instalar paquetes de Sway / SwayFX en Arch Linux
if [[ "${SKIP_PACMAN}" -eq 0 ]] && command -v pacman >/dev/null 2>&1; then
  echo ":: Arch Linux detectado. Instalando dependencias base con pacman..."
  sudo pacman -S --needed --noconfirm \
    waybar kitty swaybg swww autotiling fastfetch \
    grim slurp wl-clipboard cage chromium ttf-jetbrains-mono-nerd

  if [[ "${COMPOSITOR}" == "swayfx" ]]; then
    if command -v yay >/dev/null 2>&1; then
      echo ":: Instalando SwayFX, SwayNC y Swaylock-Effects vía yay..."
      yay -S --needed --noconfirm swayfx swaync swaylock-effects-git rofi-wayland || true
    elif command -v paru >/dev/null 2>&1; then
      echo ":: Instalando SwayFX, SwayNC y Swaylock-Effects vía paru..."
      paru -S --needed --noconfirm swayfx swaync swaylock-effects-git rofi-wayland || true
    else
      echo ":: [Aviso] No se detectó yay/paru; instalando Sway oficial de extra/ mientras tanto..."
      sudo pacman -S --needed --noconfirm sway swaylock rofi
      echo "   (Para efectos blur/corner_radius de SwayFX luego puedes ejecutar: yay -S swayfx)"
    fi
  elif [[ "${COMPOSITOR}" == "sway" ]]; then
    echo ":: Instalando Sway estándar desde repositorios oficiales de Arch..."
    sudo pacman -S --needed --noconfirm sway swaylock rofi
  elif [[ "${COMPOSITOR}" == "hyprland" ]]; then
    sudo pacman -S --needed --noconfirm hyprland rofi
  fi
fi

# 2. Usar binarios estáticos incluidos en el ZIP (x86_64) o compilar con Go
if [[ "${FORCE_BUILD}" -eq 0 && "${ARCH_CPU}" == "x86_64" && -x "${ROOT_DIR}/bin/kitsune-de" && -x "${ROOT_DIR}/bin/kitsune-ctl" ]]; then
  echo ":: Usando binarios estáticos Go precompilados incluidos en el paquete (linux/amd64)..."
  BIN_DE="${ROOT_DIR}/bin/kitsune-de"
  BIN_CTL="${ROOT_DIR}/bin/kitsune-ctl"
else
  if ! command -v go >/dev/null 2>&1; then
    if command -v pacman >/dev/null 2>&1; then
      echo ":: Instalando compilador Go desde pacman..."
      sudo pacman -S --needed --noconfirm go
    else
      echo "Error: No se encontró 'go' para compilar en ${ARCH_CPU}. Instala Go primero."
      exit 1
    fi
  fi
  echo ":: Compilando binarios nativos en Go (kitsune-de y kitsune-ctl)..."
  mkdir -p "${ROOT_DIR}/build"
  CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${ROOT_DIR}/build/kitsune-de" ./cmd/kitsune-de
  CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${ROOT_DIR}/build/kitsune-ctl" ./cmd/kitsune-ctl
  BIN_DE="${ROOT_DIR}/build/kitsune-de"
  BIN_CTL="${ROOT_DIR}/build/kitsune-ctl"
fi

# 3. Instalar binarios y entradas de sesión para SDDM / GDM / LightDM / Greetd
echo ":: Instalando binarios en /usr/bin y sesiones Wayland (SwayFX, Sway y Kiosk)..."
sudo install -Dm755 "${BIN_DE}" /usr/bin/kitsune-de
sudo install -Dm755 "${BIN_CTL}" /usr/bin/kitsune-ctl
sudo install -Dm755 "${ROOT_DIR}/arch/kitsune-session" /usr/bin/kitsune-session
sudo install -Dm755 "${ROOT_DIR}/arch/kitsune-sway-session" /usr/bin/kitsune-sway-session
sudo install -Dm644 "${ROOT_DIR}/arch/kitsune-swayfx.desktop" /usr/share/wayland-sessions/kitsune-swayfx.desktop
sudo install -Dm644 "${ROOT_DIR}/arch/kitsune-sway.desktop" /usr/share/wayland-sessions/kitsune-sway.desktop
sudo install -Dm644 "${ROOT_DIR}/arch/kitsune-de.desktop" /usr/share/wayland-sessions/kitsune-de.desktop
sudo install -Dm644 "${ROOT_DIR}/arch/kitsune-de-x11.desktop" /usr/share/xsessions/kitsune-de-x11.desktop

# 4. Copiar fondos anime a ~/.config/kitsune-de/wallpapers
mkdir -p "${HOME}/.config/kitsune-de/wallpapers"
cp -f "${ROOT_DIR}/internal/ui/static/wallpapers/"*.jpg "${HOME}/.config/kitsune-de/wallpapers/"

# 5. Exportar dotfiles reales de Sway, SwayFX, Waybar, Swaylock, SwayNC, Kitty y Rofi
if [[ "${WITH_DOTFILES}" -eq 1 ]]; then
  STAMP="$(date +%Y%m%d-%H%M%S)"
  for dir in sway swayfx waybar swaylock swaync kitty rofi fastfetch hypr; do
    if [[ -d "${HOME}/.config/${dir}" ]]; then
      cp -r "${HOME}/.config/${dir}" "${HOME}/.config/${dir}.kitsune-bak-${STAMP}"
      echo "   [backup] Respaldo creado en ~/.config/${dir}.kitsune-bak-${STAMP}"
    fi
  done
  echo ":: Exportando dotfiles Anime (${COMPOSITOR} • ${THEME}) en ~/.config..."
  "${BIN_CTL}" export --compositor "${COMPOSITOR}" --theme "${THEME}" --out "${HOME}/.config"
fi

echo ""
echo "✅ ¡KitsuneDE (${COMPOSITOR^^}) instalado con éxito!"
echo "────────────────────────────────────────────────────────────────"
echo "  1) En SDDM / GDM / LightDM : elige 'KitsuneDE + SwayFX (Anime Wayland)' o 'KitsuneDE + Sway'"
echo "  2) Desde TTY directamente  : kitsune-sway-session"
echo "  3) Panel interactivo Go    : kitsune-de --kiosk --compositor ${COMPOSITOR} --theme ${THEME}"
echo "  4) Cambiar tema en vivo    : kitsune-ctl theme next   (actualiza $SWAYSOCK al instante)"
echo "────────────────────────────────────────────────────────────────"
