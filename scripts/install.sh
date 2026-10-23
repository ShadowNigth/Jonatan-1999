#!/usr/bin/env bash
# swaydesk · instalador para Arch Linux + sway
#
#   ./scripts/install.sh              # compila e instala en ~/.local/bin
#   PREFIX=/usr/local sudo ./scripts/install.sh
#   ./scripts/install.sh --deps       # instala además las dependencias con pacman
#   ./scripts/install.sh --no-config  # no escribe la configuración
#   ./scripts/install.sh --no-path    # no toca los archivos de shell
#
# Instalación en un paso, sin clonar a mano (clona en un directorio temporal):
#   curl -fsSL https://raw.githubusercontent.com/ShadowNigth/Jonatan-1999/main/scripts/install.sh | bash -s -- --deps
#
# Variables: SWAYDESK_REPO (URL del repositorio), SWAYDESK_BRANCH (rama o
# etiqueta), PREFIX (destino de los binarios).

set -euo pipefail

# Directorio del código: cuando el script se ejecuta desde una copia local es
# el propio repositorio; cuando llega por una tubería (curl | bash) no hay
# árbol y se clona uno más abajo.
SELF="${BASH_SOURCE[0]:-}"
REPO_DIR="$(cd "$(dirname "$SELF")/.." 2>/dev/null && pwd || pwd)"
REPO_URL="${SWAYDESK_REPO:-https://github.com/ShadowNigth/Jonatan-1999.git}"
REPO_BRANCH="${SWAYDESK_BRANCH:-}"
PREFIX="${PREFIX:-$HOME/.local}"
WITH_DEPS=0
WITH_CONFIG=1
FORCE=0
WITH_PATH=1

for arg in "$@"; do
  case "$arg" in
    --deps) WITH_DEPS=1 ;;
    --no-config) WITH_CONFIG=0 ;;
    --no-path) WITH_PATH=0 ;;
    --force) FORCE=1 ;;
    -h|--help)
      # Cabecera del archivo, quitando la almohadilla inicial.
      if [[ -n "$SELF" && -r "$SELF" ]]; then
        sed -n '2,13p' "$SELF" | sed 's/^# \{0,1\}//'
      else
        printf 'swaydesk · instalador para Arch Linux + sway\n\n'
        printf '  curl -fsSL https://raw.githubusercontent.com/ShadowNigth/Jonatan-1999/main/scripts/install.sh | bash -s -- [--deps] [--no-config] [--force]\n\n'
        printf 'Variables: SWAYDESK_REPO, SWAYDESK_BRANCH, PREFIX\n'
      fi
      exit 0 ;;
    *) echo "opción desconocida: $arg" >&2; exit 2 ;;
  esac
done

info() { printf '\033[1;34m::\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31mxx\033[0m %s\n' "$*" >&2; exit 1; }

# ── Código fuente ─────────────────────────────────────────────────────────
TMP_DIR=""
cleanup() { [[ -n "$TMP_DIR" ]] && rm -rf "$TMP_DIR"; }
trap cleanup EXIT

if [[ ! -f "$REPO_DIR/go.mod" || ! -f "$REPO_DIR/Makefile" || ! -d "$REPO_DIR/cmd/swaydesk" ]]; then
  command -v git >/dev/null || die "git no está instalado. Instálalo con: sudo pacman -S git"
  TMP_DIR="$(mktemp -d)"
  info "Clonando $REPO_URL${REPO_BRANCH:+ (rama $REPO_BRANCH)}"
  if [[ -n "$REPO_BRANCH" ]]; then
    git clone --depth 1 --branch "$REPO_BRANCH" "$REPO_URL" "$TMP_DIR/swaydesk" \
      || die "no se pudo clonar $REPO_URL ($REPO_BRANCH)"
  else
    git clone --depth 1 "$REPO_URL" "$TMP_DIR/swaydesk" \
      || die "no se pudo clonar $REPO_URL"
  fi
  REPO_DIR="$TMP_DIR/swaydesk"
  if [[ ! -f "$REPO_DIR/go.mod" ]]; then
    BRANCH_HINT="main"
    [[ -n "$REPO_BRANCH" ]] && BRANCH_HINT="$REPO_BRANCH"
    die "la rama '$BRANCH_HINT' de $REPO_URL no contiene el código (¿solo el README?).
   Indica la rama del proyecto, por ejemplo:
     SWAYDESK_BRANCH=arena/5224a2d1-jonatan-1999 … | bash -s -- --deps
   o clona con: git clone -b <rama> $REPO_URL"
  fi
else
  info "Usando el código de $REPO_DIR"
fi

# ── Comprobaciones ────────────────────────────────────────────────────────
if [[ -f /etc/os-release ]]; then
  . /etc/os-release
  info "Distribución detectada: ${PRETTY_NAME:-desconocida}"
fi

command -v go >/dev/null || die "Go no está instalado. Instálalo con: sudo pacman -S go"
command -v make >/dev/null || die "make no está instalado. Instálalo con: sudo pacman -S base-devel"
[[ -f "$REPO_DIR/go.mod" ]] || die "no se encuentra el código fuente de swaydesk (go.mod)"
info "Go $(go version | awk '{print $3}')"

# ── Dependencias ──────────────────────────────────────────────────────────
DEP_PKGS=(
  waybar foot mako wl-clipboard grim slurp swayidle swaylock
  cliphist ttf-jetbrains-mono-nerd
  # kbd trae las fuentes de consola PSF que usa la barra propia (swaybar)
  kbd
)

# sway y swayfx están declarados en conflicto: quien ya usa swayfx (sway con
# blur y esquinas redondeadas) no debe recibir el paquete sway, o pacman
# aborta con «paquetes con conflictos sin resolver».
if pacman -Qq swayfx >/dev/null 2>&1; then
  info "swayfx detectado: se usa ese compositor y no se pide sway"
elif pacman -Qq sway >/dev/null 2>&1; then
  info "sway detectado"
else
  DEP_PKGS+=(sway)
fi

if [[ $WITH_DEPS -eq 1 ]]; then
  command -v pacman >/dev/null || die "pacman no está disponible"
  info "Instalando dependencias: ${DEP_PKGS[*]}"
  # Un fallo aquí no debe impedir compilar: puede que ya esté todo y solo
  # falte un paquete opcional, o que el usuario no tenga permisos de sudo.
  if ! sudo pacman -S --needed --noconfirm "${DEP_PKGS[@]}"; then
    warn "no se pudieron instalar todas las dependencias; se continúa igualmente"
    warn "puedes revisarlas después con: swaydesk doctor"
  fi
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
for bin in swaydesk swaymenu swayctl swayclip swaybar swaynotify swayosd; do
  install -Dm755 "$REPO_DIR/bin/$bin" "$PREFIX/bin/$bin"
done

PATH_LINE="export PATH=\"$PREFIX/bin:\$PATH\""
case ":$PATH:" in
  *":$PREFIX/bin:"*) info "$PREFIX/bin ya está en el PATH" ;;
  *)
    if [[ $WITH_PATH -eq 1 ]]; then
      # Se añade a los archivos de shell habituales (una sola vez). Es lo que
      # hace que `swaydesk` funcione en la siguiente terminal, sin tener que
      # copiar y pegar nada.
      touch_one() {
        [[ -f "$1" ]] || return 0
        grep -qF "$PATH_LINE" "$1" 2>/dev/null && return 0
        printf '\n# swaydesk\n%s\n' "$PATH_LINE" >>"$1"
        info "PATH añadido a ${1/#$HOME/~}"
      }
      touch_one "$HOME/.bashrc"
      touch_one "$HOME/.zshrc"
      touch_one "$HOME/.profile"
      info "En esta terminal ejecuta:  export PATH=\"$PREFIX/bin:\$PATH\"  (o abre una nueva)"
    else
      warn "$PREFIX/bin no está en el PATH. Añade esta línea a ~/.bashrc o ~/.zshrc:"
      warn "  $PATH_LINE"
    fi ;;
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
