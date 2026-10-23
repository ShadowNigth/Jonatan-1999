#!/usr/bin/env bash
# Inicia el Dock flotante inferior (nwg-dock) idéntico al Dock Anime de KitsuneDE
pkill -f nwg-dock 2>/dev/null || true
sleep 0.4
if command -v nwg-dock >/dev/null 2>&1; then
  exec nwg-dock -d -p bottom -i 34 -mb 10 -ml 12 -mr 12 -s style.css -c "rofi -show drun"
fi
