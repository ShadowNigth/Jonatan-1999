#!/bin/sh
# swaydesk · autostart de la sesión de sway
#
# Lo lanza ~/.config/sway/config con:
#   exec_always --no-startup-id ~/.config/swaydesk/autostart.sh
#
# Cada bloque comprueba que el programa exista, así que puedes instalarlo o
# quitarlo sin editar este archivo. Añade tus propias líneas al final.
#
# IMPORTANTE: `exec_always` significa que este script se ejecuta también en
# cada `swaymsg reload`. Por eso todo lo que arranca pasa por `once` (no
# duplica lo que ya está en marcha) y lo que solo debe ocurrir una vez por
# sesión (animación de arranque, bienvenida) pasa por `una_vez`.

have() { command -v "$1" >/dev/null 2>&1; }
quiet() { "$@" >/dev/null 2>&1 & }

# alive NOMBRE — ¿hay un proceso con ese nombre exacto?
alive() {
    if ! have pgrep; then return 1; fi
    pgrep -x "$1" >/dev/null 2>&1
}

# once NOMBRE [args…] — arranca solo si no está ya en marcha. Si el programa
# se cerró (o nunca llegó a arrancar), el siguiente reload lo recupera.
once() {
    _nombre="$1"; shift
    alive "$_nombre" && return 0
    quiet "$_nombre" "$@"
}

# una_vez CLAVE — verdadero la primera vez en esta sesión de sway. SWAYSOCK
# cambia en cada arranque del compositor, así que un reload no repite nada.
una_vez() {
    _dir="${XDG_RUNTIME_DIR:-/tmp}"
    _marca="$_dir/swaydesk-$1-$(printf '%s' "${SWAYSOCK:-sesion}" | cksum 2>/dev/null | cut -d' ' -f1)"
    [ -e "$_marca" ] && return 1
    : >"$_marca" 2>/dev/null || return 1
    return 0
}

# --- Animación de arranque -------------------------------------------------
# Se muestra a pantalla completa y termina sola (o al pulsar una tecla).
# Desactívala con boot.enabled = false en ~/.config/swaydesk/config.toml.
if have swaydesk && una_vez arranque; then
    quiet swaydesk boot
fi

# --- Barra de estado -------------------------------------------------------
# swaydesk trae su propia barra (swaybar). Si `bar.enabled = true` en
# config.toml y swaybar está instalado, se usa esa; si no, se usa waybar.
if have swaybar && swaydesk bar --enabled >/dev/null 2>&1; then
    once swaybar
elif have waybar; then
    if alive waybar; then
        # Ya corriendo: solo se le pide que relea la hoja de estilos.
        pkill -SIGUSR2 -x waybar >/dev/null 2>&1
    else
        quiet waybar
    fi
fi

# --- Notificaciones --------------------------------------------------------
# swaynotify (propio) dibuja las tarjetas en una capa overlay con el tema
# activo. Si no está instalado o está desactivado (notify.enabled = false),
# se usa mako.
if have swaynotify && swaydesk notify-enabled >/dev/null 2>&1; then
    once swaynotify daemon
elif have mako; then
    if [ -f "$HOME/.config/mako/config" ]; then
        once mako -c "$HOME/.config/mako/config"
    else
        once mako
    fi
fi

# --- Avisos en pantalla (volumen) ------------------------------------------
# swayosd vigila el volumen y muestra una tarjeta flotante con el nivel cada
# vez que cambia (teclas multimedia, auriculares, cualquier programa).
# Desactívalo con osd.enabled = false en config.toml.
if have swayosd && swaydesk osd-enabled >/dev/null 2>&1; then
    once swayosd watch
fi

# --- Bloqueo y ahorro de energía -------------------------------------------
# 10 min: bloquear · 15 min: apagar pantalla · al suspender: bloquear
if have swayidle; then
    once swayidle -w \
        timeout 600 'swaydesk lock' \
        timeout 900 'swaymsg "output * dpms off"' \
        resume 'swaymsg "output * dpms on"' \
        before-sleep 'swaydesk lock'
fi

# --- Historial del portapapeles --------------------------------------------
# Dos vigilantes (texto e imágenes); se comprueba cada uno por su línea de
# órdenes para no acabar con cuatro tras varios reload.
if have cliphist && have wl-paste; then
    for _tipo in text image; do
        if ! pgrep -f "wl-paste --type $_tipo --watch" >/dev/null 2>&1; then
            quiet wl-paste --type "$_tipo" --watch cliphist store
        fi
    done
fi

# --- Agente de autenticación y llavero -------------------------------------
if [ -x /usr/lib/polkit-gnome/polkit-gnome-authentication-agent-1 ]; then
    if ! pgrep -f polkit-gnome-authentication-agent-1 >/dev/null 2>&1; then
        quiet /usr/lib/polkit-gnome/polkit-gnome-authentication-agent-1
    fi
fi
if have gnome-keyring-daemon; then
    if ! pgrep -f gnome-keyring-daemon >/dev/null 2>&1; then
        quiet gnome-keyring-daemon --start --components=secrets
    fi
fi

# --- Portales de escritorio (compartir pantalla, capturas) -----------------
if have xdg-desktop-portal-wlr; then
    if ! pgrep -f xdg-desktop-portal-wlr >/dev/null 2>&1; then
        quiet xdg-desktop-portal-wlr
    fi
fi

# --- Fondo animado (GIF, vídeo o escena) -----------------------------------
# El demonio dibuja detrás de las ventanas con wlr-layer-shell. `--if-needed`
# evita reiniciarlo en cada reload (parpadeo) y lo arranca si no estaba.
if have swaydesk; then
    quiet swaydesk wallpaper daemon --if-needed
fi

# --- Fondo de pantalla de reserva (si no lo pone sway) ---------------------
if have swaybg && [ -n "$SWAYDESK_WALLPAPER" ] && [ -f "$SWAYDESK_WALLPAPER" ]; then
    once swaybg -m fill -i "$SWAYDESK_WALLPAPER"
fi

# --- Bienvenida ------------------------------------------------------------
if una_vez bienvenida; then
    sleep 2
    if have swaydesk; then
        swaydesk notify "swaydesk" "Escritorio listo · Super+d lanzador · Super+i panel" >/dev/null 2>&1
    fi
fi

# --- Tus programas ---------------------------------------------------------
# quiet firefox
# quiet spotify
# quiet nm-applet
# quiet blueman-applet
