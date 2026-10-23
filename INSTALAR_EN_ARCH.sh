#!/usr/bin/env bash
# ==============================================================================
# 🦊 KitsuneDE — Instalador Todo-en-Uno para Arch Linux
# Funciona tanto desde el ZIP precompilado como clonando desde GitHub.
# ==============================================================================
set -euo pipefail

THEME="neon-tokyo"
WITH_DOTFILES=0
SKIP_PACMAN=0
FORCE_BUILD=0

usage() {
  cat <<EOF
🦊 KitsuneDE — Instalador Listo para Arch Linux (Motor Go)

Uso:
  ./INSTALAR_EN_ARCH.sh [opciones]

Opciones:
  --theme <id>        Tema anime inicial (default: neon-tokyo)
                      Opciones: neon-tokyo | sakura-lofi | crimson-shonen | eva-midnight
  --with-dotfiles     Instalar también configs de Hyprland, Waybar, Kitty, Rofi y Fastfetch en ~/.config
                      (crea respaldo automático .bak si ya tenías configs previas)
  --build             Forzar compilación desde el código fuente en Go aunque existan binarios precompilados
  --no-pacman         Omitir instalación de paquetes opcionales con pacman
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
      WITH_DOTFILES=1
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
echo "│  🦊 KitsuneDE // Instalador para Arch Linux (Go Engine)      │"
echo "╰──────────────────────────────────────────────────────────────╯"

ARCH_CPU="$(uname -m)"
BIN_DE=""
BIN_CTL=""

# 1. Instalar dependencias recomendadas en Arch Linux si pacman está presente
if [[ "${SKIP_PACMAN}" -eq 0 ]] && command -v pacman >/dev/null 2>&1; then
  echo ":: Arch Linux detectado. Instalando paquetes base recomendados..."
  sudo pacman -S --needed --noconfirm cage chromium fastfetch ttf-jetbrains-mono-nerd
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

# 3. Instalar binarios y sesiones de escritorio (Wayland + X11)
echo ":: Instalando binarios en /usr/bin y sesiones para SDDM/GDM/LightDM..."
sudo install -Dm755 "${BIN_DE}" /usr/bin/kitsune-de
sudo install -Dm755 "${BIN_CTL}" /usr/bin/kitsune-ctl
sudo install -Dm755 "${ROOT_DIR}/arch/kitsune-session" /usr/bin/kitsune-session
sudo install -Dm644 "${ROOT_DIR}/arch/kitsune-de.desktop" /usr/share/wayland-sessions/kitsune-de.desktop
sudo install -Dm644 "${ROOT_DIR}/arch/kitsune-de-x11.desktop" /usr/share/xsessions/kitsune-de-x11.desktop

# 4. Copiar fondos anime a ~/.config/kitsune-de/wallpapers
mkdir -p "${HOME}/.config/kitsune-de/wallpapers"
cp -f "${ROOT_DIR}/internal/ui/static/wallpapers/"*.jpg "${HOME}/.config/kitsune-de/wallpapers/"

# Guardar tema inicial elegido
"${BIN_CTL}" export --theme "${THEME}" --out "${HOME}/.config/kitsune-de/dotfiles-preview" >/dev/null

# 5. Exportar dotfiles reales a ~/.config (con respaldo seguro) si se solicitó
if [[ "${WITH_DOTFILES}" -eq 1 ]]; then
  STAMP="$(date +%Y%m%d-%H%M%S)"
  for dir in hypr waybar kitty rofi fastfetch; do
    if [[ -d "${HOME}/.config/${dir}" ]]; then
      cp -r "${HOME}/.config/${dir}" "${HOME}/.config/${dir}.kitsune-bak-${STAMP}"
      echo "   [backup] Respaldo creado en ~/.config/${dir}.kitsune-bak-${STAMP}"
    fi
  done
  echo ":: Aplicando dotfiles Anime (${THEME}) en ~/.config..."
  "${BIN_CTL}" export --theme "${THEME}" --out "${HOME}/.config"
fi

echo ""
echo "✅ ¡KitsuneDE instalado con éxito!"
echo "────────────────────────────────────────────────────────────────"
echo "  1) Desde SDDM / GDM / LightDM : elige la sesión 'KitsuneDE (Anime Wayland)'"
echo "  2) Lanzar ahora mismo         : kitsune-de --kiosk --theme ${THEME}"
echo "  3) Modo servidor local        : kitsune-de --addr 0.0.0.0:8080"
echo "  4) Cambiar tema en vivo       : kitsune-ctl theme next"
echo "────────────────────────────────────────────────────────────────"
