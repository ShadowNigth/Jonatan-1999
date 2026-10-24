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

# sway no hereda el PATH de tu shell: aquí se añaden las carpetas habituales
# de binarios de usuario, para que este script (y lo que lanza) encuentre
# swaydesk, swaybar o swaynotify aunque no estén en el PATH de la sesión.
PATH="$HOME/.local/bin:$HOME/bin:$PATH"
export PATH

# Waybar no siempre descubre config.json automáticamente (puede caer en
# /etc/xdg/waybar/config.jsonc); usa explícitamente la plantilla de swaydesk.
WAYBAR_CONFIG="${XDG_CONFIG_HOME:-$HOME/.config}/waybar/config.json"

LOG_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/swaydesk"
LOG_FILE=/dev/null
if mkdir -p "$LOG_DIR" 2>/dev/null; then
    LOG_FILE="$LOG_DIR/autostart.log"
fi
log() {
    [ "$LOG_FILE" = /dev/null ] && return 0
    printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >>"$LOG_FILE" 2>/dev/null
}
have() { command -v "$1" >/dev/null 2>&1; }
quiet() {
    log "iniciando: $*"
    "$@" >>"$LOG_FILE" 2>&1 &
}
DBUS_STATUS=ausente
[ -n "${DBUS_SESSION_BUS_ADDRESS:-}" ] && DBUS_STATUS=presente
log "--- autostart; WAYLAND_DISPLAY=${WAYLAND_DISPLAY:-vacío}; SWAYSOCK=${SWAYSOCK:-vacío}; D-Bus=$DBUS_STATUS ---"

# alive NOMBRE — ¿hay un proceso con ese nombre exacto?
alive() {
    if ! have pgrep; then return 1; fi
    pgrep -x "$1" >/dev/null 2>&1
}

# once NOMBRE [args…] — arranca solo si no está ya en marcha. Si el programa
# se cerró (o nunca llegó a arrancar), el siguiente reload lo recupera.
once() {
    _nombre="$1"; shift
    if alive "$_nombre"; then
        log "ya está activo: $_nombre"
        return 0
    fi
    quiet "$_nombre" "$@"
}

# start_bar NOMBRE [args…] — la barra es crítica para la sesión: además de
# lanzarla, comprueba que sobreviva al inicio y reintenta fallos tempranos.
start_bar() {
    _nombre="$1"; shift
    if alive "$_nombre"; then
        log "barra ya activa: $_nombre"
        return 0
    fi
    if ! have pgrep; then
        log "no se puede verificar $_nombre (falta pgrep); se intenta una vez"
        quiet "$_nombre" "$@"
        return 0
    fi
    _intento=1
    while [ "$_intento" -le 3 ]; do
        quiet "$_nombre" "$@"
        _espera=0
        while [ "$_espera" -lt 12 ]; do
            sleep 0.1
            if alive "$_nombre"; then
                # Captura salidas inmediatas por Wayland, CSS o módulos.
                sleep 0.4
                if alive "$_nombre"; then
                    log "barra $_nombre quedó activa (intento $_intento/3)"
                    return 0
                fi
                break
            fi
            _espera=$((_espera + 1))
        done
        log "barra $_nombre no quedó activa (intento $_intento/3)"
        _intento=$((_intento + 1))
        if [ "$_intento" -le 3 ]; then sleep 0.3; fi
    done
    log "ERROR: $_nombre falló en los 3 intentos; revisa la salida anterior en este registro"
    return 1
}

# waybar_uses_generated_config distingue la instancia correcta de una Waybar
# antigua que arrancó con el config del sistema.
waybar_uses_generated_config() {
    [ -f "$WAYBAR_CONFIG" ] || return 1
    have pgrep && have grep || return 1
    pgrep -a -x waybar 2>/dev/null | grep -F -- "$WAYBAR_CONFIG" >/dev/null 2>&1
}

start_waybar() {
    if [ -f "$WAYBAR_CONFIG" ]; then
        log "Waybar usará la configuración de swaydesk: $WAYBAR_CONFIG"
        start_bar waybar --config "$WAYBAR_CONFIG"
    else
        log "no se encontró $WAYBAR_CONFIG; se usa la configuración por defecto de Waybar"
        start_bar waybar
    fi
}

# stop_exact NOMBRE — detiene un proveedor alternativo y espera brevemente
# para no arrancar dos barras/daemons que compiten por la misma superficie o
# el mismo nombre D-Bus. Devuelve error si no pudo detenerlo.
stop_exact() {
    _nombre="$1"
    if ! alive "$_nombre"; then return 0; fi
    if ! have pkill; then
        log "no se puede detener $_nombre: falta pkill"
        return 1
    fi
    log "deteniendo proveedor alternativo: $_nombre"
    pkill -TERM -x "$_nombre" >/dev/null 2>&1 || true
    _intento=0
    while alive "$_nombre" && [ "$_intento" -lt 10 ]; do
        sleep 0.1
        _intento=$((_intento + 1))
    done
    if alive "$_nombre"; then
        log "$_nombre sigue activo; no se iniciará otro proveedor"
        return 1
    fi
    return 0
}

# start_swaynotify arranca el proveedor propio y espera a que acepte peticiones.
# Así el resto de la sesión no queda sin avisos si Wayland/D-Bus tarda en iniciar.
start_swaynotify() {
    _noti_intento=1
    while [ "$_noti_intento" -le 3 ]; do
        quiet swaynotify daemon
        _noti_espera=0
        while [ "$_noti_espera" -lt 40 ]; do
            if swaynotify status >/dev/null 2>&1; then
                log "swaynotify quedó activo (intento $_noti_intento/3)"
                return 0
            fi
            sleep 0.1
            _noti_espera=$((_noti_espera + 1))
        done
        log "swaynotify no respondió (intento $_noti_intento/3)"
        if ! stop_exact swaynotify; then
            log "no se pudo reiniciar swaynotify; comprueba el autostart"
            return 1
        fi
        _noti_intento=$((_noti_intento + 1))
        if [ "$_noti_intento" -le 3 ]; then sleep 0.3; fi
    done
    log "ERROR: swaynotify no pudo registrarse; revisa D-Bus, Wayland y el registro anterior"
    return 1
}

# start_events_watcher asegura una sola instancia y la renueva en cada reload,
# para que use el binario nuevo y las variables de la sesión actual.
start_events_watcher() {
    _events_pattern='[s]waydesk events watch'
    if have pgrep && pgrep -f "$_events_pattern" >/dev/null 2>&1; then
        if ! have pkill; then
            log "watcher de dispositivos ya activo; falta pkill para reiniciarlo"
            return 0
        fi
        log "reiniciando watcher de dispositivos para la sesión actual"
        pkill -TERM -f "$_events_pattern" >/dev/null 2>&1 || true
        _events_wait=0
        while pgrep -f "$_events_pattern" >/dev/null 2>&1 && [ "$_events_wait" -lt 20 ]; do
            sleep 0.1
            _events_wait=$((_events_wait + 1))
        done
        if pgrep -f "$_events_pattern" >/dev/null 2>&1; then
            log "ERROR: el watcher anterior no terminó; no se inicia otro"
            return 1
        fi
    fi

    quiet swaydesk events watch
    if have pgrep; then
        _events_wait=0
        while [ "$_events_wait" -lt 20 ]; do
            if pgrep -f "$_events_pattern" >/dev/null 2>&1; then
                log "watcher de dispositivos activo"
                return 0
            fi
            sleep 0.1
            _events_wait=$((_events_wait + 1))
        done
        log "ERROR: el watcher no quedó activo; revisa su salida en este registro"
        return 1
    fi
    return 0
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
# Los proveedores son excluyentes: si `bar.enabled = true`, swaydesk usa su
# swaybar y detiene Waybar; si está apagado (o falta swaybar), hace lo inverso.
if have swaydesk && have swaybar && swaydesk bar --enabled >/dev/null 2>&1; then
    if stop_exact waybar; then
        start_bar swaybar
    fi
else
    if stop_exact swaybar; then
        if have waybar; then
            if alive waybar; then
                if [ -f "$WAYBAR_CONFIG" ] && ! waybar_uses_generated_config; then
                    log "Waybar estaba usando otra configuración; se reinicia con la de swaydesk"
                    if stop_exact waybar; then
                        start_waybar
                    fi
                else
                    # Ya corre con el config elegido: solo releer CSS y módulos.
                    if have pkill && pkill -SIGUSR2 -x waybar >/dev/null 2>&1; then
                        log "recarga de CSS/config enviada a waybar"
                    else
                        log "no se pudo recargar waybar; queda activo con la configuración actual"
                    fi
                fi
            else
                start_waybar
            fi
        else
            log "barra omitida: no se encontró swaybar habilitada ni waybar instalada"
        fi
    fi
fi

# --- Notificaciones --------------------------------------------------------
# swaynotify (propio) dibuja las tarjetas con el tema activo. Solo un proceso
# puede poseer org.freedesktop.Notifications: al seleccionar swaynotify se
# detienen mako/dunst; al desactivarlo, se conserva dunst si ya estaba activo
# y se usa mako como alternativa.
if have swaydesk && have swaynotify && swaydesk notify-enabled >/dev/null 2>&1; then
    _puede_iniciar=1
    stop_exact mako || _puede_iniciar=0
    stop_exact dunst || _puede_iniciar=0
    if [ "$_puede_iniciar" -eq 1 ]; then
        if swaynotify status >/dev/null 2>&1; then
            log "swaynotify ya responde en D-Bus"
        elif stop_exact swaynotify; then
            if ! start_swaynotify; then
                if have mako; then
                    log "swaynotify no quedó activo; se inicia mako como alternativa"
                    if [ -f "$HOME/.config/mako/config" ]; then
                        once mako -c "$HOME/.config/mako/config"
                    else
                        once mako
                    fi
                elif have dunst; then
                    log "swaynotify no quedó activo; se inicia dunst como alternativa"
                    once dunst
                else
                    log "ERROR: no hay un proveedor de notificaciones disponible"
                fi
            fi
        else
            log "no se pudo liberar el proveedor de notificaciones; swaynotify no se inicia"
        fi
    fi
else
    if stop_exact swaynotify; then
        if alive dunst; then
            stop_exact mako || true
            log "dunst ya está activo; no se inicia mako"
        elif have mako; then
            if [ -f "$HOME/.config/mako/config" ]; then
                once mako -c "$HOME/.config/mako/config"
            else
                once mako
            fi
        fi
    fi
fi

# --- Avisos en pantalla (volumen y reproducción) ----------------------------
# swayosd watch vigila el volumen y las señales MPRIS; cada cambio de pista
# muestra una tarjeta con título, artista y carátula. Se controlan por separado
# con osd.enabled y osd.media_enabled en config.toml.
if have swayosd && swaydesk osd-enabled >/dev/null 2>&1; then
    once swayosd watch
fi

# --- Dispositivos conectados ------------------------------------------------
# El watcher recibe entradas y monitores desde Sway, y discos USB desde udev.
# exec_always vuelve a ejecutar este archivo en cada reload: no lo dupliques.
if have swaydesk; then
    start_events_watcher
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

# --- Fondo animado o fondos por workspace -------------------------------
# El demonio dibuja con wlr-layer-shell. `--if-needed` evita reiniciarlo en
# cada reload y solo lo arranca si hay animación o reglas por workspace.
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
