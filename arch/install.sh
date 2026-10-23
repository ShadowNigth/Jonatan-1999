#!/usr/bin/env bash
# ==============================================================================
# 🦊 KitsuneDE — Instalador para Arch Linux (Motor Go + Rice Anime)
# ==============================================================================
set -euo pipefail

THEME="neon-tokyo"
EXPORT_DOTFILES=0
INSTALL_PKGS=1

usage() {
  cat <<EOF
🦊 Instalador de KitsuneDE para Arch Linux

Uso:
  ./arch/install.sh [opciones]

Opciones:
  --theme <id>        Tema anime inicial: neon-tokyo | sakura-lofi | crimson-shonen | eva-midnight
  --with-dotfiles     Exportar también configs reales de Hyprland/Waybar/Kitty/Rofi a ~/.config
  --no-pacman         Omitir instalación de paquetes vía pacman (solo compilar e instalar binarios Go)
  -h, --help          Mostrar esta ayuda
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --theme)
      THEME="${2:-neon-tokyo}"
      shift 2
      ;;
    --with-dotfiles)
      EXPORT_DOTFILES=1
      shift
      ;;
    --no-pacman)
      INSTALL_PKGS=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Opción desconocida: $1"
      usage
      exit 1
      ;;
  esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

echo "╭──────────────────────────────────────────────────────────────╮"
echo "│  🦊 Instalando KitsuneDE — Escritorio Arch Linux Anime (Go)  │"
echo "╰──────────────────────────────────────────────────────────────╯"

if [[ "${INSTALL_PKGS}" -eq 1 ]] && command -v pacman >/dev/null 2>&1; then
  echo ":: Detectado Arch Linux (pacman). Instalando dependencias base..."
  sudo pacman -S --needed --noconfirm go cage chromium fastfetch ttf-jetbrains-mono-nerd
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Error: No se encontró el compilador 'go'. Instálalo con: sudo pacman -S go"
  exit 1
fi

echo ":: Compilando binarios nativos en Go (kitsune-de y kitsune-ctl)..."
mkdir -p build
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o build/kitsune-de ./cmd/kitsune-de
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o build/kitsune-ctl ./cmd/kitsune-ctl

echo ":: Instalando ejecutables y sesiones de escritorio (Wayland + X11)..."
sudo install -Dm755 build/kitsune-de /usr/bin/kitsune-de
sudo install -Dm755 build/kitsune-ctl /usr/bin/kitsune-ctl
sudo install -Dm755 arch/kitsune-session /usr/bin/kitsune-session
sudo install -Dm644 arch/kitsune-de.desktop /usr/share/wayland-sessions/kitsune-de.desktop
sudo install -Dm644 arch/kitsune-de-x11.desktop /usr/share/xsessions/kitsune-de-x11.desktop

mkdir -p "${HOME}/.config/kitsune-de/wallpapers"
cp -f internal/ui/static/wallpapers/*.jpg "${HOME}/.config/kitsune-de/wallpapers/"

if [[ "${EXPORT_DOTFILES}" -eq 1 ]]; then
  echo ":: Exportando dotfiles de Hyprland, Waybar, Kitty, Rofi y Fastfetch para el tema '${THEME}'..."
  ./build/kitsune-ctl export --theme "${THEME}" --out "${HOME}/.config"
fi

echo ""
echo "✓ Instalación completada."
echo "  • Para iniciar como sesión desde SDDM/GDM/LightDM: elige 'KitsuneDE (Anime Wayland)'"
echo "  • Para lanzar en ventana/pantalla completa ahora : kitsune-de --kiosk --theme ${THEME}"
echo "  • Para controlar en vivo desde terminal          : kitsune-ctl theme next"
