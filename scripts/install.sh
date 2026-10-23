#!/usr/bin/env bash
# swaydesk · instalador para Arch Linux + sway
#
#   ./scripts/install.sh              # compila e instala en ~/.local/bin
#   PREFIX=/usr/local sudo ./scripts/install.sh
#   ./scripts/install.sh --deps       # instala además las dependencias con pacman
#   ./scripts/install.sh --no-config  # no escribe la configuración

set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${PREFIX:-$HOME/.local}"
WITH_DEPS=0
WITH_CONFIG=1
FORCE=0

for arg in "$@"; do
  case "$arg" in
    --deps) WITH_DEPS=1 ;;
    --no-config) WITH_CONFIG=0 ;;
    --force) FORCE=1 ;;
    -h|--help)
      sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) echo "opción desconocida: $arg" >&2; exit 2 ;;
  esac
done

info() { printf '\033[1;34m::\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31mxx\033[0m %s\n' "$*" >&2; exit 1; }

# ── Comprobaciones ────────────────────────────────────────────────────────
if [[ -f /etc/os-release ]]; then
  . /etc/os-release
  info "Distribución detectada: ${PRETTY_NAME:-desconocida}"
fi

command -v go >/dev/null || die "Go no está instalado. Instálalo con: sudo pacman -S go"
info "Go $(go version | awk '{print $3}')"

# ── Dependencias ──────────────────────────────────────────────────────────
DEP_PKGS=(
  sway waybar foot mako wl-clipboard grim slurp swayidle swaylock
  cliphist ttf-jetbrains-mono-nerd
  # kbd trae las fuentes de consola PSF que usa la barra propia (swaybar)
  kbd
)
if [[ $WITH_DEPS -eq 1 ]]; then
  command -v pacman >/dev/null || die "pacman no está disponible"
  info "Instalando dependencias: ${DEP_PKGS[*]}"
  sudo pacman -S --needed --noconfirm "${DEP_PKGS[@]}"
else
  MISSING=()
  for pkg in "${DEP_PKGS[@]}"; do
    pacman -Qq "$pkg" >/dev/null 2>&1 || MISSING+=("$pkg")
  done
  if [[ ${#MISSING[@]} -gt 0 ]]; then
    warn "Paquetes recomendados que faltan: ${MISSING[*]}"
    warn "Puedes instalarlos con: sudo pacman -S --needed ${MISSING[*]}"
    warn "(o ejecuta este script con --deps)"
  fi
fi

# ── Compilación e instalación ─────────────────────────────────────────────
info "Compilando binarios (CGO desactivado, binarios estáticos)..."
cd "$REPO_DIR"
make build

info "Instalando en $PREFIX/bin"
for bin in swaydesk swaymenu swayctl swayclip swaybar; do
  install -Dm755 "$REPO_DIR/bin/$bin" "$PREFIX/bin/$bin"
done

case ":$PATH:" in
  *":$PREFIX/bin:"*) ;;
  *) warn "$PREFIX/bin no está en el PATH. Añade esta línea a ~/.bashrc o ~/.zshrc:"
     warn "  export PATH=\"$PREFIX/bin:\$PATH\"" ;;
esac

# ── Configuración ─────────────────────────────────────────────────────────
if [[ $WITH_CONFIG -eq 1 ]]; then
  info "Generando configuración del escritorio"
  if [[ $FORCE -eq 1 ]]; then
    "$PREFIX/bin/swaydesk" init --force
  else
    "$PREFIX/bin/swaydesk" init
  fi
fi

# ── Resumen ───────────────────────────────────────────────────────────────
echo
info "Instalación completada."
echo "  1. Revisa ~/.config/sway/config y ~/.config/swaydesk/config.toml"
echo "  2. Recarga sway:    swaymsg reload"
echo "  3. Comprueba todo:  swaydesk doctor"
echo "  4. Lanzador:        Super+d   ·   Panel: Super+i"
echo
warn "Si ya tenías sway, waybar, foot o mako configurados, se respaldaron como *.bak-<fecha>"
