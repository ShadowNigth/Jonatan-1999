#!/usr/bin/env bash
# ==============================================================================
# 🦊 Kitsune Go Wayland v2.0.2 — Reparador Rápido de 1 Línea desde TTY (SDDM)
# Uso desde consola TTY (Ctrl+Alt+F3):
#   curl -fL https://github.com/ShadowNigth/Jonatan-1999/raw/v2.0.2/fix.sh | bash
# No requiere 'unzip' instalado (usa bsdtar incluido por defecto en Arch Linux).
# ==============================================================================
set -euo pipefail

ZIP_URL="https://github.com/ShadowNigth/Jonatan-1999/raw/v2.0.2/release/kitsune-go-wayland-arch-linux.zip"
WORK_DIR="$(mktemp -d /tmp/kitsune-fix.XXXXXX)"
trap 'rm -rf "${WORK_DIR}"' EXIT

echo ":: Descargando Kitsune Go Wayland v2.0.2..."
curl -fL "${ZIP_URL}" -o "${WORK_DIR}/kitsune.zip"

echo ":: Extrayendo paquete (usando bsdtar nativo de Arch Linux)..."
if command -v bsdtar >/dev/null 2>&1; then
  bsdtar -xf "${WORK_DIR}/kitsune.zip" -C "${WORK_DIR}"
elif command -v unzip >/dev/null 2>&1; then
  unzip -oq "${WORK_DIR}/kitsune.zip" -d "${WORK_DIR}"
else
  sudo pacman -S --needed --noconfirm unzip
  unzip -oq "${WORK_DIR}/kitsune.zip" -d "${WORK_DIR}"
fi

chmod +x "${WORK_DIR}/kitsune-go-wayland-arch-linux/INSTALAR_EN_ARCH.sh"
exec "${WORK_DIR}/kitsune-go-wayland-arch-linux/INSTALAR_EN_ARCH.sh" "$@"
