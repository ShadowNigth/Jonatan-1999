#!/usr/bin/env bash
# ==============================================================================
# 🦊 KitsuneDE v1.2.0 — Instalador Visual-Twin Completo para Arch Linux
# Instala TODAS las dependencias (oficiales + AUR auto-bootstrap), tipografías
# Kanji/Emoji/NerdFont, animaciones 60fps, dock flotante, blur SwayFX y sesión
# Full Anime Shell para que en Arch Linux se vea 100% IDÉNTICO al ejemplo web.
# ==============================================================================
set -euo pipefail

COMPOSITOR="swayfx"
THEME="neon-tokyo"
WITH_DOTFILES=1
SKIP_PACMAN=0
SKIP_AUR=0
FORCE_BUILD=0

usage() {
  cat <<EOF
🦊 KitsuneDE v1.2.0 — Instalador Visual-Twin para Arch Linux, SwayFX & Sway

Uso:
  ./INSTALAR_EN_ARCH.sh [opciones]

Opciones:
  --compositor <wm>   Compositor principal: swayfx (default) | sway | hyprland
  --theme <id>        Tema anime inicial (default: neon-tokyo)
                      Opciones: neon-tokyo | sakura-lofi | crimson-shonen | eva-midnight
  --with-dotfiles     Exportar los 17 dotfiles en ~/.config con respaldo .bak (activo por defecto)
  --no-dotfiles       Instalar binarios, dependencias y sesiones sin modificar ~/.config
  --no-aur            Usar únicamente repositorios oficiales de pacman (sin yay/AUR)
  --build             Forzar compilación desde el código fuente en Go
  --no-pacman         Omitir instalación de paquetes del sistema
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

# Detectar usuario real incluso si el script fue invocado con sudo
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
echo "│  🦊 KitsuneDE v1.2.0 // Edición Visual-Twin Completa (Arch Linux)    │"
echo "╰──────────────────────────────────────────────────────────────────────╯"
echo "  • Usuario destino    : ${REAL_USER} (${REAL_HOME})"
echo "  • Compositor elegido : ${COMPOSITOR}"
echo "  • Tema Anime inicial : ${THEME}"

ARCH_CPU="$(uname -m)"
BIN_DE=""
BIN_CTL=""

# ==============================================================================
# 1. Instalar TODAS las dependencias visuales, fuentes, audio, dock y animaciones
# ==============================================================================
if [[ "${SKIP_PACMAN}" -eq 0 ]] && command -v pacman >/dev/null 2>&1; then
  echo ":: Sincronizando base de datos de paquetes de Arch Linux..."
  ${SUDO} pacman -Sy --noconfirm

  DESIRED_OFFICIAL=(
    # Herramientas base
    base-devel git curl jq
    # Barra cristalina, Dock flotante, Wallpapers animados 60fps y Tiling espiral
    waybar nwg-dock nwg-look swaybg swww autotiling
    # Terminal GPU, monitor, explorador TUI/GUI, notificaciones y audio
    kitty fastfetch btop yazi neovim thunar pavucontrol libnotify
    # Portales Wayland, capturas, portapapeles, polkit y XWayland
    grim slurp wl-clipboard polkit-gnome xorg-xwayland
    xdg-desktop-portal xdg-desktop-portal-wlr xdg-desktop-portal-gtk
    # Sesión Full Anime Shell (100% idéntica al ejemplo web con GPU + Audio + Partículas)
    cage chromium
    # Tipografías completas (Nerd Fonts + Inter + Kanji Japonés + Color Emoji) e Iconos
    fontconfig ttf-jetbrains-mono-nerd ttf-firacode-nerd inter-font
    noto-fonts noto-fonts-cjk noto-fonts-emoji
    papirus-icon-theme gnome-themes-extra
    # Notificaciones y Lanzador Wayland (si están en extra/, se instalan directo)
    swaync rofi-wayland
  )

  PACMAN_INSTALL=()
  AUR_FALLBACK=()

  for pkg in "${DESIRED_OFFICIAL[@]}"; do
    if pacman_has_pkg "${pkg}"; then
      PACMAN_INSTALL+=("${pkg}")
    else
      if [[ "${pkg}" == "rofi-wayland" ]] && pacman_has_pkg "rofi"; then
        PACMAN_INSTALL+=("rofi")
      else
        AUR_FALLBACK+=("${pkg}")
      fi
    fi
  done

  if [[ "${COMPOSITOR}" == "sway" || "${SKIP_AUR}" -eq 1 ]]; then
    PACMAN_INSTALL+=("sway" "swaylock")
  elif [[ "${COMPOSITOR}" == "hyprland" ]]; then
    PACMAN_INSTALL+=("hyprland")
  fi

  if [[ "${#PACMAN_INSTALL[@]}" -gt 0 ]]; then
    echo ":: Instalando dependencias oficiales con pacman (${#PACMAN_INSTALL[@]} paquetes)..."
    ${SUDO} pacman -S --needed --noconfirm "${PACMAN_INSTALL[@]}"
  fi

  # Auto-instalar yay-bin desde AUR si no existe yay ni paru (y no es root puro)
  AUR_HELPER=""
  if [[ "${SKIP_AUR}" -eq 0 ]]; then
    if command -v yay >/dev/null 2>&1; then
      AUR_HELPER="yay"
    elif command -v paru >/dev/null 2>&1; then
      AUR_HELPER="paru"
    elif [[ "${REAL_USER}" != "root" ]]; then
      echo ":: No se detectó yay/paru. Auto-instalando 'yay-bin' para paquetes visuales AUR..."
      TMP_YAY="$(run_as_user mktemp -d /tmp/kitsune-yay.XXXXXX)"
      if run_as_user git clone --depth 1 https://aur.archlinux.org/yay-bin.git "${TMP_YAY}/yay-bin"; then
        (cd "${TMP_YAY}/yay-bin" && run_as_user makepkg -si --needed --noconfirm) || true
      fi
      rm -rf "${TMP_YAY}" || true
      if command -v yay >/dev/null 2>&1; then
        AUR_HELPER="yay"
      fi
    fi
  fi

  # Instalar paquetes visuales de AUR (SwayFX con blur/corner_radius, Swaylock-effects, Cursor Bibata)
  if [[ -n "${AUR_HELPER}" && "${SKIP_AUR}" -eq 0 ]]; then
    AUR_INSTALL=("bibata-cursor-theme-bin" "${AUR_FALLBACK[@]}")
    if [[ "${COMPOSITOR}" == "swayfx" ]]; then
      AUR_INSTALL=("swayfx" "swaylock-effects-git" "${AUR_INSTALL[@]}")
    fi
    echo ":: Instalando efectos visuales Anime desde AUR vía ${AUR_HELPER}: ${AUR_INSTALL[*]}..."
    run_as_user "${AUR_HELPER}" -S --needed --noconfirm --ask 4 "${AUR_INSTALL[@]}" || true
  fi

  # Garantizar que /usr/bin/sway y /usr/bin/swaylock siempre queden disponibles
  if [[ "${COMPOSITOR}" == "swayfx" || "${COMPOSITOR}" == "sway" ]]; then
    if ! command -v sway >/dev/null 2>&1; then
      echo ":: Instalando Sway oficial como respaldo garantizado..."
      ${SUDO} pacman -S --needed --noconfirm sway
    fi
    if ! command -v swaylock >/dev/null 2>&1; then
      ${SUDO} pacman -S --needed --noconfirm swaylock || true
    fi
  fi

  echo ":: Actualizando caché tipográfico (Kanji + Nerd Fonts + Color Emoji)..."
  ${SUDO} fc-cache -fv >/dev/null 2>&1 || true
fi

# ==============================================================================
# 2. Usar binarios estáticos incluidos en el ZIP (x86_64) o compilar con Go
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
# 3. Instalar binarios, wallpapers globales y sesiones para SDDM / GDM / LightDM
# ==============================================================================
echo ":: Instalando binarios en /usr/bin, wallpapers globales y sesiones Wayland..."
${SUDO} install -Dm755 "${BIN_DE}" /usr/bin/kitsune-de
${SUDO} install -Dm755 "${BIN_CTL}" /usr/bin/kitsune-ctl
${SUDO} install -Dm755 "${ROOT_DIR}/arch/kitsune-session" /usr/bin/kitsune-session
${SUDO} install -Dm755 "${ROOT_DIR}/arch/kitsune-sway-session" /usr/bin/kitsune-sway-session
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-swayfx.desktop" /usr/share/wayland-sessions/kitsune-swayfx.desktop
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-sway.desktop" /usr/share/wayland-sessions/kitsune-sway.desktop
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-de.desktop" /usr/share/wayland-sessions/kitsune-de.desktop
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-de-x11.desktop" /usr/share/xsessions/kitsune-de-x11.desktop
${SUDO} install -Dm644 "${ROOT_DIR}/arch/kitsune-studio.desktop" /usr/share/applications/kitsune-studio.desktop
${SUDO} install -d /usr/share/kitsune-de/wallpapers
${SUDO} install -Dm644 "${ROOT_DIR}/internal/ui/static/wallpapers/"*.jpg /usr/share/kitsune-de/wallpapers/

# ==============================================================================
# 4. Copiar fondos anime a ~/.config/kitsune-de/wallpapers del usuario
# ==============================================================================
run_as_user mkdir -p "${REAL_HOME}/.config/kitsune-de/wallpapers"
for wall in "${ROOT_DIR}/internal/ui/static/wallpapers/"*.jpg; do
  run_as_user cp -f "${wall}" "${REAL_HOME}/.config/kitsune-de/wallpapers/"
done

# ==============================================================================
# 5. Exportar los 17 dotfiles Visual-Twin (SwayFX, Waybar, Dock, GTK3/4, Kitty, Rofi)
# ==============================================================================
if [[ "${WITH_DOTFILES}" -eq 1 ]]; then
  STAMP="$(date +%Y%m%d-%H%M%S)"
  BACKUP_ROOT="${REAL_HOME}/.config/kitsune-backups/${STAMP}"
  for dir in sway swayfx waybar nwg-dock swaylock swaync gtk-3.0 gtk-4.0 kitty rofi fastfetch hypr; do
    if [[ -d "${REAL_HOME}/.config/${dir}" ]]; then
      run_as_user mkdir -p "${BACKUP_ROOT}"
      run_as_user cp -r "${REAL_HOME}/.config/${dir}" "${BACKUP_ROOT}/${dir}"
    fi
  done
  if [[ -d "${BACKUP_ROOT}" ]]; then
    echo "   [backup] Configuraciones previas respaldadas en ${BACKUP_ROOT}"
  fi

  echo ":: Exportando los 17 dotfiles Visual-Twin (${COMPOSITOR} • ${THEME}) en ${REAL_HOME}/.config..."
  run_as_user "${BIN_CTL}" export --compositor "${COMPOSITOR}" --theme "${THEME}" --out "${REAL_HOME}/.config"
  if [[ -f "${REAL_HOME}/.config/sway/start-dock.sh" ]]; then
    run_as_user chmod +x "${REAL_HOME}/.config/sway/start-dock.sh"
  fi
fi

# Preferencia oscura GTK/Libadwaita para que todas las apps nativas coincidan
if command -v gsettings >/dev/null 2>&1; then
  run_as_user gsettings set org.gnome.desktop.interface color-scheme 'prefer-dark' >/dev/null 2>&1 || true
  run_as_user gsettings set org.gnome.desktop.interface gtk-theme 'Adwaita-dark' >/dev/null 2>&1 || true
  run_as_user gsettings set org.gnome.desktop.interface icon-theme 'Papirus-Dark' >/dev/null 2>&1 || true
  run_as_user gsettings set org.gnome.desktop.interface cursor-theme 'Bibata-Modern-Ice' >/dev/null 2>&1 || true
fi

echo ""
echo "╭──────────────────────────────────────────────────────────────────────╮"
echo "│  ✅ KitsuneDE v1.2.0 instalado con todas sus dependencias visuales    │"
echo "╰──────────────────────────────────────────────────────────────────────╯"
echo "  En tu pantalla de login (SDDM / GDM / LightDM / greetd) tienes 2 modos:"
echo ""
echo "  1) ✨ KitsuneDE Full Anime Shell (Idéntico al Web)"
echo "     -> Píxel por píxel IGUAL a la demo web (partículas 60fps, audio,"
echo "        animaciones elásticas, Dock, Waybar, Terminal real y Rice Studio)"
echo "     -> Comando directo desde TTY: kitsune-session"
echo ""
echo "  2) 🦊 KitsuneDE + SwayFX (Visual-Twin Wayland Nativo)"
echo "     -> SwayFX con blur + corner_radius + Waybar Kanji + nwg-dock flotante"
echo "        + swww wallpapers 60fps + Rofi + SwayNC + recarga de temas en vivo"
echo "     -> Comando directo desde TTY: kitsune-sway-session swayfx"
echo ""
echo "  Atajos rápidos dentro del escritorio:"
echo "    • Super + T        -> Cambiar tema anime en vivo (con transición 60fps)"
echo "    • Super + G        -> Abrir Kitsune Rice Studio"
echo "    • Super + Space    -> Abrir lanzador anime Rofi"
echo "    • Super + Return   -> Abrir terminal Kitty"
