#!/usr/bin/env bash
# ==============================================================================
# 🦊 Kitsune Go Wayland v2.0.2 — Instalador & Reparador SDDM/Wayland (Arch Linux)
# Registra PRIMERO la sesión 'Kitsune Go (Wayland)' en /usr/share/wayland-sessions
# para que nunca quede vacío el selector '(Wayland)' de SDDM, instala cage +
# chromium + mesa + dbus de forma resiliente y reinicia SDDM si estás en TTY.
# ==============================================================================
set -uo pipefail

THEME="neon-tokyo"
WITH_DOTFILES=1
SKIP_PACMAN=0
SKIP_AUR=0
FORCE_BUILD=0
NO_RESTART_DM=0

usage() {
  cat <<EOF
🦊 Kitsune Go Wayland v2.0.2 — Instalador y Reparador para Arch Linux

Uso:
  ./INSTALAR_EN_ARCH.sh [opciones]

Opciones:
  --theme <id>        Tema anime inicial (default: neon-tokyo)
                      Opciones: neon-tokyo | sakura-lofi | crimson-shonen | eva-midnight
  --with-dotfiles     Exportar configuraciones Kitsune Go Wayland, Btop++, Yazi y Neovim (default)
  --no-dotfiles       Instalar binarios y sesión Wayland sin tocar ~/.config
  --no-aur            Usar únicamente repositorios oficiales de pacman
  --build             Forzar compilación desde el código fuente en Go
  --no-pacman         Omitir instalación de paquetes del sistema
  --no-restart-dm     No reiniciar SDDM automáticamente al terminar desde TTY
  -h, --help          Mostrar esta ayuda
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
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
    --no-aur)
      SKIP_AUR=1
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
    --no-restart-dm)
      NO_RESTART_DM=1
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

if [[ "${EUID}" -eq 0 && -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
  REAL_USER="${SUDO_USER}"
  REAL_HOME="$(getent passwd "${SUDO_USER}" | cut -d: -f6)"
  SUDO=""
else
  REAL_USER="$(id -un)"
  REAL_HOME="${HOME}"
  if [[ "${EUID}" -eq 0 ]]; then
    SUDO=""
  else
    SUDO="sudo"
  fi
fi
REAL_GROUP="$(id -gn "${REAL_USER}" 2>/dev/null || echo "${REAL_USER}")"

run_as_user() {
  if [[ "${EUID}" -eq 0 && "${REAL_USER}" != "root" ]]; then
    sudo -u "${REAL_USER}" -H "$@"
  else
    "$@"
  fi
}

echo "╭──────────────────────────────────────────────────────────────────────╮"
echo "│  🦊 Kitsune Go Wayland v2.0.2 // Instalador & Reparador SDDM Login   │"
echo "╰──────────────────────────────────────────────────────────────────────╯"
echo "  • Usuario destino : ${REAL_USER} (${REAL_HOME})"
echo "  • Motor Wayland   : Kitsune Go Wayland (Sin Sway / SwayFX / Hyprland)"
echo "  • Modo Escritorio : TABBED (Sin Dock inferior)"
echo "  • Tema Inicial    : ${THEME}"

ARCH_CPU="$(uname -m)"
BIN_DE=""
BIN_CTL=""

# ==============================================================================
# PASO 1: Preparar e instalar INMEDIATAMENTE binarios y archivos .desktop de SDDM
# (Así /usr/share/wayland-sessions/kitsune-de.desktop NUNCA queda sin instalar)
# ==============================================================================
if [[ "${FORCE_BUILD}" -eq 0 && "${ARCH_CPU}" == "x86_64" && -x "${ROOT_DIR}/bin/kitsune-de" && -x "${ROOT_DIR}/bin/kitsune-ctl" ]]; then
  echo ":: [1/5] Usando binarios estáticos Go precompilados (linux/amd64)..."
  BIN_DE="${ROOT_DIR}/bin/kitsune-de"
  BIN_CTL="${ROOT_DIR}/bin/kitsune-ctl"
else
  if ! command -v go >/dev/null 2>&1; then
    if command -v pacman >/dev/null 2>&1; then
      echo ":: Instalando compilador Go desde pacman..."
      ${SUDO} pacman -S --needed --noconfirm go || ${SUDO} pacman -Syu --needed --noconfirm go
    else
      echo "Error: No se encontró 'go' para compilar en ${ARCH_CPU}."
      exit 1
    fi
  fi
  echo ":: [1/5] Compilando binarios nativos en Go (kitsune-de y kitsune-ctl)..."
  mkdir -p "${ROOT_DIR}/build"
  CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${ROOT_DIR}/build/kitsune-de" ./cmd/kitsune-de
  CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${ROOT_DIR}/build/kitsune-ctl" ./cmd/kitsune-ctl
  BIN_DE="${ROOT_DIR}/build/kitsune-de"
  BIN_CTL="${ROOT_DIR}/build/kitsune-ctl"
fi

echo ":: [2/5] Registrando sesión 'Kitsune Go (Wayland)' en SDDM / Display Manager..."
${SUDO} install -d -m755 /usr/bin /usr/share/wayland-sessions /usr/share/xsessions /usr/share/applications /usr/share/kitsune-de/wallpapers
${SUDO} install -Dm755 "${BIN_DE}" /usr/bin/kitsune-de
${SUDO} install -Dm755 "${BIN_CTL}" /usr/bin/kitsune-ctl
${SUDO} install -Dm755 "${ROOT_DIR}/arch/kitsune-session" /usr/bin/kitsune-session
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-de.desktop" /usr/share/wayland-sessions/kitsune-de.desktop
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-de-x11.desktop" /usr/share/xsessions/kitsune-de-x11.desktop
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-studio.desktop" /usr/share/applications/kitsune-studio.desktop
${SUDO} chmod 755 /usr/share/wayland-sessions /usr/share/xsessions
${SUDO} chmod 644 /usr/share/wayland-sessions/kitsune-de.desktop /usr/share/xsessions/kitsune-de-x11.desktop
${SUDO} install -Dm644 "${ROOT_DIR}/internal/ui/static/wallpapers/"*.jpg /usr/share/kitsune-de/wallpapers/

# Limpiar sesiones antiguas y caché corrupta de estado en SDDM (que causaba "(Wayland)" vacío)
${SUDO} rm -f /usr/bin/kitsune-sway-session \
  /usr/share/wayland-sessions/kitsune-sway.desktop \
  /usr/share/wayland-sessions/kitsune-swayfx.desktop \
  /var/lib/sddm/state.conf 2>/dev/null || true

# ==============================================================================
# PASO 2: Instalar dependencias críticas Wayland (cage + chromium + mesa + dbus)
# ==============================================================================
if [[ "${SKIP_PACMAN}" -eq 0 ]] && command -v pacman >/dev/null 2>&1; then
  echo ":: [3/5] Instalando motor gráfico Wayland (cage, chromium, mesa, dbus, polkit)..."
  ${SUDO} rm -f /var/lib/pacman/db.lck 2>/dev/null || true

  CORE_WAYLAND_PKGS=(
    dbus polkit mesa
    cage chromium xorg-xwayland
    labwc weston
    openbox xorg-xrandr xorg-xset xorg-xsetroot
  )

  # Intentar instalación directa; si los espejos requieren actualizar DB, usar -Syu
  if ! ${SUDO} pacman -S --needed --noconfirm "${CORE_WAYLAND_PKGS[@]}"; then
    echo ":: Actualizando base de datos de espejos con pacman -Syu..."
    ${SUDO} pacman -Syu --needed --noconfirm "${CORE_WAYLAND_PKGS[@]}" || {
      # Instalar paquete por paquete para que ninguno opcional bloquee cage/chromium
      for p in "${CORE_WAYLAND_PKGS[@]}"; do
        ${SUDO} pacman -S --needed --noconfirm "${p}" || true
      done
    }
  fi

  EXTRA_PKGS=(
    base-devel git curl jq
    btop yazi neovim kitty fastfetch pavucontrol
    wl-clipboard grim slurp libnotify polkit-gnome
    fontconfig ttf-jetbrains-mono-nerd ttf-firacode-nerd inter-font
    noto-fonts noto-fonts-cjk noto-fonts-emoji
    papirus-icon-theme gnome-themes-extra
  )
  ${SUDO} pacman -S --needed --noconfirm "${EXTRA_PKGS[@]}" || {
    for p in "${EXTRA_PKGS[@]}"; do
      ${SUDO} pacman -S --needed --noconfirm "${p}" || true
    done
  }

  # Permisos de acceso directo a GPU AMD/Intel/NVIDIA e input
  for grp in video render input seat; do
    if getent group "${grp}" >/dev/null 2>&1; then
      ${SUDO} usermod -aG "${grp}" "${REAL_USER}" 2>/dev/null || true
    fi
  done

  # Cursor Bibata opcional desde AUR (solo si ya hay helper o no estamos en root puro)
  if [[ "${SKIP_AUR}" -eq 0 ]]; then
    if command -v yay >/dev/null 2>&1; then
      run_as_user yay -S --needed --noconfirm bibata-cursor-theme-bin || true
    elif command -v paru >/dev/null 2>&1; then
      run_as_user paru -S --needed --noconfirm bibata-cursor-theme-bin || true
    fi
  fi

  ${SUDO} fc-cache -fv >/dev/null 2>&1 || true
fi

# ==============================================================================
# PASO 3: Reparar permisos de usuario y exportar dotfiles del tema elegido
# ==============================================================================
echo ":: [4/5] Reparando permisos de ${REAL_USER} y exportando configuraciones..."
${SUDO} mkdir -p "${REAL_HOME}/.config/kitsune-de/wallpapers" "${REAL_HOME}/.config/kitsune-de/shell-profile" "${REAL_HOME}/.cache"
${SUDO} rm -f "${REAL_HOME}/.config/kitsune-de/shell-profile/SingletonLock" \
              "${REAL_HOME}/.config/kitsune-de/shell-profile/SingletonCookie" \
              "${REAL_HOME}/.config/kitsune-de/shell-profile/SingletonSocket" 2>/dev/null || true
${SUDO} chown -R "${REAL_USER}:${REAL_GROUP}" "${REAL_HOME}/.config/kitsune-de" "${REAL_HOME}/.cache" 2>/dev/null || true

for wall in "${ROOT_DIR}/internal/ui/static/wallpapers/"*.jpg; do
  run_as_user cp -f "${wall}" "${REAL_HOME}/.config/kitsune-de/wallpapers/" 2>/dev/null || true
done

if [[ "${WITH_DOTFILES}" -eq 1 && "${REAL_USER}" != "root" ]]; then
  STAMP="$(date +%Y%m%d-%H%M%S)"
  BACKUP_ROOT="${REAL_HOME}/.config/kitsune-backups/${STAMP}"
  for dir in kitsune-wayland rofi btop yazi nvim kitty fastfetch gtk-3.0 gtk-4.0; do
    if [[ -d "${REAL_HOME}/.config/${dir}" ]]; then
      run_as_user mkdir -p "${BACKUP_ROOT}"
      run_as_user cp -r "${REAL_HOME}/.config/${dir}" "${BACKUP_ROOT}/${dir}" 2>/dev/null || true
    fi
  done
  run_as_user "${BIN_CTL}" export --theme "${THEME}" --out "${REAL_HOME}/.config" || true
  ${SUDO} chown -R "${REAL_USER}:${REAL_GROUP}" "${REAL_HOME}/.config" 2>/dev/null || true
fi

# ==============================================================================
# PASO 4: Verificación final de archivos y recarga automática de SDDM en TTY
# ==============================================================================
echo ":: [5/5] Verificando instalación de sesión SDDM Wayland:"
for f in /usr/bin/kitsune-de /usr/bin/kitsune-session /usr/share/wayland-sessions/kitsune-de.desktop; do
  if [[ -e "${f}" ]]; then
    echo "   [OK] ${f}"
  else
    echo "   [FALTA] ${f}"
  fi
done
command -v cage >/dev/null 2>&1 && echo "   [OK] Compositor Wayland: $(command -v cage)" || echo "   [AVISO] Instala cage con: sudo pacman -S cage"
command -v chromium >/dev/null 2>&1 && echo "   [OK] Renderizador GPU  : $(command -v chromium)" || echo "   [AVISO] Instala chromium con: sudo pacman -S chromium"

echo ""
echo "╭──────────────────────────────────────────────────────────────────────╮"
echo "│  ✅ Kitsune Go Wayland v2.0.2 Instalado — SDDM Reparado              │"
echo "╰──────────────────────────────────────────────────────────────────────╯"
echo "  • Ahora en SDDM te aparecerá : Kitsune Go (Wayland)"
echo "  • Selecciona 'Kitsune Go (Wayland)', pon tu contraseña y entra."

# Si el usuario ejecutó esto desde una consola TTY (Ctrl+Alt+F2/F3) con SDDM activo,
# reiniciar SDDM automáticamente para que cargue 'Kitsune Go (Wayland)' al instante.
if [[ "${NO_RESTART_DM}" -eq 0 && -z "${DISPLAY:-}" && -z "${WAYLAND_DISPLAY:-}" ]]; then
  if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet sddm; then
    echo ""
    echo ":: Reiniciando SDDM automáticamente para mostrar 'Kitsune Go (Wayland)'..."
    sleep 1
    ${SUDO} systemctl restart sddm || true
  fi
fi
