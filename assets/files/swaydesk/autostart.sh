#!/bin/sh
# swaydesk · autostart de la sesión de sway
#
# Lo lanza ~/.config/sway/config con:
#   exec_always --no-startup-id ~/.config/swaydesk/autostart.sh
#
# Cada bloque comprueba que el programa exista, así que puedes instalarlo o
# quitarlo sin editar este archivo. Añade tus propias líneas al final.

have() { command -v "$1" >/dev/null 2>&1; }
quiet() { "$@" >/dev/null 2>&1 & }

# --- Barra de estado -------------------------------------------------------
if have waybar; then
    pkill -SIGUSR2 -x waybar >/dev/null 2>&1
    sleep 0.3
    quiet waybar
fi

# --- Notificaciones --------------------------------------------------------
if have mako; then
    if [ -f "$HOME/.config/mako/config" ]; then
        quiet mako -c "$HOME/.config/mako/config"
    else
        quiet mako
    fi
fi

# --- Bloqueo y ahorro de energía -------------------------------------------
# 10 min: bloquear · 15 min: apagar pantalla · al suspender: bloquear
if have swayidle; then
    quiet swayidle -w \
        timeout 600 'swaydesk lock' \
        timeout 900 'swaymsg "output * dpms off"' \
        resume 'swaymsg "output * dpms on"' \
        before-sleep 'swaydesk lock'
fi

# --- Historial del portapapeles --------------------------------------------
if have cliphist && have wl-paste; then
    quiet wl-paste --type text --watch cliphist store
    quiet wl-paste --type image --watch cliphist store
fi

# --- Agente de autenticación y llavero -------------------------------------
if [ -x /usr/lib/polkit-gnome/polkit-gnome-authentication-agent-1 ]; then
    quiet /usr/lib/polkit-gnome/polkit-gnome-authentication-agent-1
fi
if have gnome-keyring-daemon; then
    quiet gnome-keyring-daemon --start --components=secrets
fi

# --- Portales de escritorio (compartir pantalla, capturas) -----------------
if have xdg-desktop-portal-wlr; then
    quiet /usr/lib/xdg-desktop-portal-wlr
fi

# --- Fondo de pantalla de reserva (si no lo pone sway) ---------------------
if have swaybg && [ -n "$SWAYDESK_WALLPAPER" ] && [ -f "$SWAYDESK_WALLPAPER" ]; then
    quiet swaybg -m fill -i "$SWAYDESK_WALLPAPER"
fi

# --- Bienvenida ------------------------------------------------------------
sleep 2
if have swaydesk; then
    swaydesk notify "swaydesk" "Escritorio listo · Super+d lanzador · Super+i panel" >/dev/null 2>&1
fi

# --- Tus programas ---------------------------------------------------------
# quiet firefox
# quiet spotify
# quiet nm-applet
# quiet blueman-applet
