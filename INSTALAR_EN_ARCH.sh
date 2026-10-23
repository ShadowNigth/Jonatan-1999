#!/usr/bin/env bash
# ==============================================================================
# 🦊 Kitsune Go Wayland v2.0.0 — Instalador Oficial para Arch Linux
# Escritorio Anime Tabbed puro en Go para Wayland (SIN Sway, SwayFX ni Hyprland
# y SIN Dock inferior). Instala todas las dependencias necesarias para que luzca
# 100% idéntico con sus 4 temas Anime, Blur y Bordes Luminosos en vivo.
# ==============================================================================
set -euo pipefail

THEME="neon-tokyo"
WITH_DOTFILES=1
SKIP_PACMAN=0
SKIP_AUR=0
FORCE_BUILD=0

usage() {
  cat <<EOF
🦊 Kitsune Go Wayland v2.0.0 — Instalador para Arch Linux

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

run_as_user() {
  if [[ "${EUID}" -eq 0 && "${REAL_USER}" != "root" ]]; then
    sudo -u "${REAL_USER}" -H "$@"
  else
    "$@"
  fi
}

pacman_has_pkg() {
  pacman -Si "$1" >/dev/null 2>&1
}

echo "╭──────────────────────────────────────────────────────────────────────╮"
echo "│  🦊 Kitsune Go Wayland v2.0.0 // Escritorio Anime Tabbed (Sin Dock)  │"
echo "╰──────────────────────────────────────────────────────────────────────╯"
echo "  • Usuario destino : ${REAL_USER} (${REAL_HOME})"
echo "  • Motor Wayland   : Kitsune Go Wayland (Sin Sway / SwayFX / Hyprland)"
echo "  • Modo Escritorio : TABBED (Sin Dock inferior)"
echo "  • Tema Inicial    : ${THEME}"

ARCH_CPU="$(uname -m)"
BIN_DE=""
BIN_CTL=""

# ==============================================================================
# 1. Instalar dependencias puras de Kitsune Go Wayland (0% Sway / Hyprland)
# ==============================================================================
if [[ "${SKIP_PACMAN}" -eq 0 ]] && command -v pacman >/dev/null 2>&1; then
  echo ":: Sincronizando base de datos de paquetes de Arch Linux..."
  ${SUDO} pacman -Sy --noconfirm

  DESIRED_PKGS=(
    # Base & utilidades
    base-devel git curl jq
    # Servidor de pantalla Wayland minimalista + motor GPU Ozone Wayland
    cage chromium xorg-xwayland
    # Suite de aplicaciones integrada y sincronizada
    btop yazi neovim kitty fastfetch pavucontrol
    # Portapapeles y capturas Wayland
    wl-clipboard grim slurp libnotify polkit-gnome
    # Tipografías completas (Nerd Fonts + Inter + Kanji Japonés + Color Emoji) e Iconos
    fontconfig ttf-jetbrains-mono-nerd ttf-firacode-nerd inter-font
    noto-fonts noto-fonts-cjk noto-fonts-emoji
    papirus-icon-theme gnome-themes-extra
  )

  PACMAN_INSTALL=()
  for pkg in "${DESIRED_PKGS[@]}"; do
    if pacman_has_pkg "${pkg}"; then
      PACMAN_INSTALL+=("${pkg}")
    fi
  done

  if [[ "${#PACMAN_INSTALL[@]}" -gt 0 ]]; then
    echo ":: Instalando dependencias de Kitsune Go Wayland con pacman (${#PACMAN_INSTALL[@]} paquetes)..."
    ${SUDO} pacman -S --needed --noconfirm "${PACMAN_INSTALL[@]}"
  fi

  # Cursor Bibata opcional desde AUR
  AUR_HELPER=""
  if [[ "${SKIP_AUR}" -eq 0 ]]; then
    if command -v yay >/dev/null 2>&1; then
      AUR_HELPER="yay"
    elif command -v paru >/dev/null 2>&1; then
      AUR_HELPER="paru"
    elif [[ "${REAL_USER}" != "root" ]]; then
      echo ":: Auto-instalando 'yay-bin' para tema de cursor Bibata-Modern-Ice..."
      TMP_YAY="$(run_as_user mktemp -d /tmp/kitsune-yay.XXXXXX)"
      if run_as_user git clone --depth 1 https://aur.archlinux.org/yay-bin.git "${TMP_YAY}/yay-bin"; then
        (cd "${TMP_YAY}/yay-bin" && run_as_user makepkg -si --needed --noconfirm) || true
      fi
      rm -rf "${TMP_YAY}" || true
      if command -v yay >/dev/null 2>&1; then
        AUR_HELPER="yay"
      fi
    fi

    if [[ -n "${AUR_HELPER}" ]]; then
      run_as_user "${AUR_HELPER}" -S --needed --noconfirm bibata-cursor-theme-bin || true
    fi
  fi

  echo ":: Actualizando caché tipográfico (Kanji 一 二 三 四 五 + Nerd Fonts + Color Emoji)..."
  ${SUDO} fc-cache -fv >/dev/null 2>&1 || true
fi

# ==============================================================================
# 2. Usar binarios estáticos Go incluidos en el ZIP o compilar desde fuente
# ==============================================================================
if [[ "${FORCE_BUILD}" -eq 0 && "${ARCH_CPU}" == "x86_64" && -x "${ROOT_DIR}/bin/kitsune-de" && -x "${ROOT_DIR}/bin/kitsune-ctl" ]]; then
  echo ":: Usando binarios estáticos Go precompilados incluidos en el paquete (linux/amd64)..."
  BIN_DE="${ROOT_DIR}/bin/kitsune-de"
  BIN_CTL="${ROOT_DIR}/bin/kitsune-ctl"
else
  if ! command -v go >/dev/null 2>&1; then
    if command -v pacman >/dev/null 2>&1; then
      echo ":: Instalando compilador Go desde pacman..."
      ${SUDO} pacman -S --needed --noconfirm go
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

# ==============================================================================
# 3. Instalar binarios, wallpapers y sesión única Kitsune Go Wayland
# ==============================================================================
echo ":: Instalando Kitsune Go Wayland en /usr/bin y /usr/share/wayland-sessions..."
${SUDO} install -Dm755 "${BIN_DE}" /usr/bin/kitsune-de
${SUDO} install -Dm755 "${BIN_CTL}" /usr/bin/kitsune-ctl
${SUDO} install -Dm755 "${ROOT_DIR}/arch/kitsune-session" /usr/bin/kitsune-session
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-de.desktop" /usr/share/wayland-sessions/kitsune-de.desktop
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-de-x11.desktop" /usr/share/xsessions/kitsune-de-x11.desktop
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-studio.desktop" /usr/share/applications/kitsune-studio.desktop
${SUDO} install -d /usr/share/kitsune-de/wallpapers
${SUDO} install -Dm644 "${ROOT_DIR}/internal/ui/static/wallpapers/"*.jpg /usr/share/kitsune-de/wallpapers/

# Limpiar sesiones antiguas de Sway/SwayFX si existían de versiones previas
${SUDO} rm -f /usr/bin/kitsune-sway-session \
  /usr/share/wayland-sessions/kitsune-sway.desktop \
  /usr/share/wayland-sessions/kitsune-swayfx.desktop 2>/dev/null || true

# ==============================================================================
# 4. Copiar fondos Anime y configurar modo Tabbed inicial del usuario
# ==============================================================================
run_as_user mkdir -p "${REAL_HOME}/.config/kitsune-de/wallpapers"
for wall in "${ROOT_DIR}/internal/ui/static/wallpapers/"*.jpg; do
  run_as_user cp -f "${wall}" "${REAL_HOME}/.config/kitsune-de/wallpapers/"
done

# ==============================================================================
# 5. Exportar configuraciones de Kitsune Go Wayland, Btop++, Yazi, Neovim y Kitty
# ==============================================================================
if [[ "${WITH_DOTFILES}" -eq 1 ]]; then
  STAMP="$(date +%Y%m%d-%H%M%S)"
  BACKUP_ROOT="${REAL_HOME}/.config/kitsune-backups/${STAMP}"
  for dir in kitsune-wayland rofi btop yazi nvim kitty fastfetch gtk-3.0 gtk-4.0; do
    if [[ -d "${REAL_HOME}/.config/${dir}" ]]; then
      run_as_user mkdir -p "${BACKUP_ROOT}"
      run_as_user cp -r "${REAL_HOME}/.config/${dir}" "${BACKUP_ROOT}/${dir}"
    fi
  done
  if [[ -d "${BACKUP_ROOT}" ]]; then
    echo "   [backup] Configuraciones previas respaldadas en ${BACKUP_ROOT}"
  fi

  echo ":: Exportando configuraciones de Kitsune Go Wayland (${THEME}) en ${REAL_HOME}/.config..."
  run_as_user "${BIN_CTL}" export --theme "${THEME}" --out "${REAL_HOME}/.config"
fi

if command -v gsettings >/dev/null 2>&1; then
  run_as_user gsettings set org.gnome.desktop.interface color-scheme 'prefer-dark' >/dev/null 2>&1 || true
  run_as_user gsettings set org.gnome.desktop.interface gtk-theme 'Adwaita-dark' >/dev/null 2>&1 || true
  run_as_user gsettings set org.gnome.desktop.interface icon-theme 'Papirus-Dark' >/dev/null 2>&1 || true
fi

echo ""
echo "╭──────────────────────────────────────────────────────────────────────╮"
echo "│  ✅ Kitsune Go Wayland v2.0.0 instalado correctamente                │"
echo "╰──────────────────────────────────────────────────────────────────────╯"
echo "  • Sesión en SDDM / GDM / LightDM / greetd : Kitsune Go Wayland"
echo "  • Comando directo desde TTY               : kitsune-session"
echo "  • Modo predeterminado                     : Tabbed (Sin Dock inferior)"
echo "  • Atajos rápidos                          : Alt+Space (Rofi), Alt+Tab (Pestañas), Alt+1..5 (一..五)"
