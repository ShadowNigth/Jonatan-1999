#!/usr/bin/env bash
# Wrapper directo hacia INSTALAR_EN_ARCH.sh
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec "${SCRIPT_DIR}/INSTALAR_EN_ARCH.sh" "$@"
