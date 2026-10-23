// KitsuneDE Core Shell Controller (Waybar, Rofi, SwayNC, Particles, Live Go WS Sync)
(function () {
  function wsURL(path) {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${proto}//${window.location.host}${path}`;
  }

  function esc(str) {
    return String(str ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  class DesktopController {
    constructor() {
      this.wm = new window.KitsuneWM(document.getElementById('desktop-stage'));
      this.settings = {};
      this.activeTheme = {};
      this.themes = [];
      this.system = null;
      this.archPackages = [];
      this.telemetrySubs = new Set();
      this.stateSubs = new Set();
      this.notifications = [];
      this.particleMode = 'rain';
      this.particles = [];
    }

    async init() {
      this.bindShellUI();
      this.startClock();
      this.initParticles();

      const res = await fetch('/api/bootstrap');
      const data = await res.json();
      this.themes = data.themes || [];
      this.archPackages = data.archPackages || [];
      this.applyState(data.settings, data.activeTheme);
      this.updateTelemetry(data.system);

      this.connectEventsWS();

      // Launch default Hyprland Tiled pair on Workspace 1: Kitty Fastfetch + Rice Studio
      window.KitsuneApps.open('terminal');
      window.KitsuneApps.open('settings');
      this.wm.focus(this.wm.windows[0]?.id);

      document.body.classList.remove('booting');
      this.pushToast('狐 KitsuneDE // Sway & SwayFX', 'Escritorio Arch Linux Anime impulsado por Go');
    }

    onTelemetry(fn) {
      this.telemetrySubs.add(fn);
      return () => this.telemetrySubs.delete(fn);
    }

    onStateUpdate(fn) {
      this.stateSubs.add(fn);
      return () => this.stateSubs.delete(fn);
    }

    previewLocalSettings(next) {
      this.settings = next;
      this.applyCSSSettings(next);
      this.wm.applySettings(next);
    }

    async saveSettings(next) {
      const res = await fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(next),
      });
      const data = await res.json();
      if (data.settings && data.theme) {
        this.applyState(data.settings, data.theme);
      }
    }

    applyCSSSettings(s) {
      const root = document.documentElement.style;
      root.setProperty('--border-size', `${s.borderSize ?? 2}px`);
      root.setProperty('--border-radius', `${s.borderRadius ?? 12}px`);
      root.setProperty('--win-opacity', ((s.windowOpacity ?? 90) / 100).toFixed(2));
      root.setProperty('--glass-blur', s.blurEnabled ? '16px' : '0px');

      document.body.classList.toggle('no-anim', !s.animationsEnabled);
      document.body.classList.toggle('glow-off', !s.borderGlow);
      document.body.classList.toggle('comp-sway', s.compositor === 'sway');

      const compEl = document.getElementById('wb-comp-label');
      if (compEl) compEl.textContent = (s.compositor || 'swayfx').toUpperCase();
      document.getElementById('wb-layout-label').textContent = (s.layoutMode || 'tiling').toUpperCase();
      document.querySelectorAll('.ws-btn').forEach((btn) => {
        btn.classList.toggle('active', parseInt(btn.dataset.ws, 10) === s.activeWorkspace);
      });

      document.getElementById('nc-btn-tiling').classList.toggle('active', s.layoutMode === 'tiling');
      document.getElementById('nc-btn-particles').classList.toggle('active', Boolean(s.particlesEnabled));
      document.getElementById('nc-btn-blur').classList.toggle('active', Boolean(s.blurEnabled));
    }

    applyState(settings, theme) {
      if (settings) {
        this.settings = settings;
        this.applyCSSSettings(settings);
        this.wm.applySettings(settings);
      }
      if (theme && theme.id) {
        this.activeTheme = theme;
        const root = document.documentElement.style;
        root.setProperty('--primary', theme.primary);
        root.setProperty('--secondary', theme.secondary);
        root.setProperty('--accent', theme.accent);
        root.setProperty('--bg-base', theme.bgBase);
        root.setProperty('--bg-surface', theme.bgSurface);
        root.setProperty('--bg-overlay', theme.bgOverlay);
        root.setProperty('--text-main', theme.textMain);
        root.setProperty('--text-muted', theme.textMuted);
        root.setProperty('--success', theme.success);
        root.setProperty('--warning', theme.warning);
        root.setProperty('--danger', theme.danger);

        document.getElementById('wallpaper-layer').style.backgroundImage = `url('${theme.wallpaper}')`;
        document.getElementById('wb-theme-label').textContent = theme.name;
        document.getElementById('eww-jp').textContent = theme.japaneseName;
        document.getElementById('lock-jp').textContent = `狐 ${theme.japaneseName} // Arch Linux`;

        if (this.particleMode !== theme.particleMode) {
          this.particleMode = theme.particleMode || 'rain';
          this.resetParticles();
        }
      }
      this.stateSubs.forEach((fn) => fn(this.settings, this.activeTheme));
    }

    updateTelemetry(snap) {
      if (!snap) return;
      this.system = snap;
      document.getElementById('wb-cpu').textContent = `${Math.round(snap.cpuPercent)}%`;
      document.getElementById('wb-ram').textContent = `${Math.round(snap.memPercent)}%`;
      document.getElementById('nc-user').textContent = `${snap.username}@${snap.hostname}`;
      document.getElementById('nc-uptime').textContent = `${snap.goVersion} • Uptime ${snap.uptimeHuman}`;
      this.telemetrySubs.forEach((fn) => fn(snap));
    }

    connectEventsWS() {
      const ws = new WebSocket(wsURL('/ws/events'));
      ws.onmessage = (ev) => {
        try {
          const evt = JSON.parse(ev.data);
          if (evt.type === 'telemetry') {
            this.updateTelemetry(evt.payload);
          } else if (evt.type === 'state_updated' && evt.payload) {
            this.applyState(evt.payload.settings, evt.payload.theme);
          } else if (evt.type === 'notification' && evt.payload) {
            this.pushToast(evt.payload.title || 'KitsuneDE', evt.payload.body || '');
          } else if (evt.type === 'open_app' && evt.payload?.appId) {
            window.KitsuneApps.open(evt.payload.appId);
          }
        } catch (_) {}
      };
      ws.onclose = () => {
        setTimeout(() => this.connectEventsWS(), 2200);
      };
    }

    bindShellUI() {
      // Sync Waybar window title & workspace dots & dock running indicators
      this.wm.onStateChange = ({ focused, occupied, runningApps }) => {
        document.getElementById('wb-win-text').textContent = focused ? focused.title : 'Escritorio KitsuneDE';
        document.querySelectorAll('.ws-btn').forEach((btn) => {
          const wsNum = parseInt(btn.dataset.ws, 10);
          btn.classList.toggle('occupied', occupied.has(wsNum));
        });
        document.querySelectorAll('.dock-item').forEach((btn) => {
          btn.classList.toggle('running', runningApps.has(btn.dataset.app));
        });
      };

      // Workspaces
      document.querySelectorAll('.ws-btn').forEach((btn) => {
        btn.addEventListener('click', () => {
          const ws = parseInt(btn.dataset.ws, 10);
          this.saveSettings({ ...this.settings, activeWorkspace: ws });
        });
      });

      // Compositor profile cycle button (SwayFX -> Sway -> Hyprland)
      const compBtn = document.getElementById('btn-compositor');
      if (compBtn) {
        compBtn.addEventListener('click', () => {
          fetch('/api/ctl', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action: 'set_compositor', value: 'toggle' }),
          });
        });
      }

      // Layout toggle button
      document.getElementById('btn-layout').addEventListener('click', () => {
        fetch('/api/ctl', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ action: 'set_layout', value: 'toggle' }),
        });
      });

      // Quick Theme cycle button
      document.getElementById('btn-quick-theme').addEventListener('click', () => {
        fetch('/api/ctl', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ action: 'next_theme' }),
        });
      });

      // Waybar telemetry -> Btop++
      document.getElementById('btn-wb-sysmon').addEventListener('click', () => {
        window.KitsuneApps.open('sysmon');
      });

      // Mini Synth Radio in Waybar
      document.getElementById('btn-mini-radio').addEventListener('click', () => {
        window.KitsuneAudio.toggle();
      });
      window.KitsuneAudio.onChange((st) => {
        document.getElementById('wb-eq').classList.toggle('playing', st.playing);
        document.getElementById('wb-radio-label').textContent = st.playing ? st.track.jp : 'Synth Off';
        document.getElementById('nc-btn-radio').classList.toggle('active', st.playing);
      });

      // Dock App Launchers
      document.querySelectorAll('.dock-item').forEach((btn) => {
        btn.addEventListener('click', () => {
          const app = btn.dataset.app;
          window.KitsuneApps.open(app);
        });
      });

      // Rofi Launcher
      const rofiOverlay = document.getElementById('rofi-overlay');
      const rofiInput = document.getElementById('rofi-input');
      document.getElementById('btn-rofi').addEventListener('click', () => this.openRofi());
      rofiOverlay.addEventListener('click', (e) => {
        if (e.target === rofiOverlay) rofiOverlay.classList.add('hidden');
      });
      rofiInput.addEventListener('input', () => this.renderRofiItems(rofiInput.value));
      rofiInput.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') rofiOverlay.classList.add('hidden');
        if (e.key === 'Enter') {
          const first = document.querySelector('#rofi-list .rofi-item');
          if (first) first.click();
        }
      });

      // SwayNC Notification Drawer
      const drawer = document.getElementById('swaync-drawer');
      document.getElementById('btn-notif-drawer').addEventListener('click', () => {
        drawer.classList.toggle('hidden');
      });
      document.getElementById('btn-clear-notifs').addEventListener('click', () => {
        this.notifications = [];
        this.renderDrawerNotifs();
      });
      document.getElementById('nc-btn-tiling').addEventListener('click', () => {
        const nextMode = this.settings.layoutMode === 'tiling' ? 'floating' : 'tiling';
        this.saveSettings({ ...this.settings, layoutMode: nextMode });
      });
      document.getElementById('nc-btn-particles').addEventListener('click', () => {
        this.saveSettings({ ...this.settings, particlesEnabled: !this.settings.particlesEnabled });
      });
      document.getElementById('nc-btn-blur').addEventListener('click', () => {
        this.saveSettings({ ...this.settings, blurEnabled: !this.settings.blurEnabled });
      });
      document.getElementById('nc-btn-radio').addEventListener('click', () => {
        window.KitsuneAudio.toggle();
      });

      // Swaylock Screen
      const lockEl = document.getElementById('lockscreen');
      document.getElementById('btn-lock').addEventListener('click', () => {
        lockEl.classList.remove('hidden');
        document.getElementById('lock-pass').focus();
      });
      document.getElementById('lock-form').addEventListener('submit', (e) => {
        e.preventDefault();
        document.getElementById('lock-pass').value = '';
        lockEl.classList.add('hidden');
      });

      // Right-click Desktop Context Menu
      const ctxMenu = document.getElementById('ctx-menu');
      document.getElementById('desktop-stage').addEventListener('contextmenu', (e) => {
        if (e.target.closest('.kitsune-window')) return;
        e.preventDefault();
        ctxMenu.style.left = `${Math.min(window.innerWidth - 240, e.clientX)}px`;
        ctxMenu.style.top = `${Math.min(window.innerHeight - 240, e.clientY)}px`;
        ctxMenu.classList.remove('hidden');
      });
      window.addEventListener('click', () => ctxMenu.classList.add('hidden'));
      ctxMenu.querySelectorAll('[data-ctx]').forEach((btn) => {
        btn.addEventListener('click', () => {
          const val = btn.dataset.ctx;
          if (val.startsWith('open:')) {
            const app = val.split(':')[1];
            window.KitsuneApps.open(app, app === 'terminal' ? { multiInstance: true } : {});
          } else if (val === 'action:next-theme') {
            document.getElementById('btn-quick-theme').click();
          } else if (val === 'action:toggle-layout') {
            document.getElementById('btn-layout').click();
          } else if (val === 'action:lock') {
            lockEl.classList.remove('hidden');
          }
        });
      });

      // Global Keyboard Shortcuts (Alt+...)
      window.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
          rofiOverlay.classList.add('hidden');
          drawer.classList.add('hidden');
          ctxMenu.classList.add('hidden');
        }
        if (!e.altKey) return;
        const k = e.key.toLowerCase();
        if (e.code === 'Space') {
          e.preventDefault();
          this.openRofi();
        } else if (e.key === 'Enter') {
          e.preventDefault();
          window.KitsuneApps.open('terminal', { multiInstance: true });
        } else if (k === 'w') {
          e.preventDefault();
          const next = this.settings.layoutMode === 'tabbed' ? 'tiling' : 'tabbed';
          this.saveSettings({ ...this.settings, layoutMode: next });
        } else if (k === 't') {
          e.preventDefault();
          document.getElementById('btn-layout').click();
        } else if (k === 'v') {
          e.preventDefault();
          this.wm.toggleFocusedFloat();
        } else if (k === 'q') {
          e.preventDefault();
          this.wm.closeFocused();
        } else if (k === 'l') {
          e.preventDefault();
          lockEl.classList.remove('hidden');
        } else if (['1', '2', '3', '4', '5'].includes(k)) {
          e.preventDefault();
          this.saveSettings({ ...this.settings, activeWorkspace: parseInt(k, 10) });
        }
      });
    }

    openRofi() {
      const overlay = document.getElementById('rofi-overlay');
      const input = document.getElementById('rofi-input');
      overlay.classList.remove('hidden');
      input.value = '';
      this.renderRofiItems('');
      setTimeout(() => input.focus(), 30);
    }

    renderRofiItems(q) {
      const query = q.trim().toLowerCase();
      const entries = [
        { label: '>_  Kitty Terminal (Nueva ventana)', sub: 'Alt+Enter', run: () => window.KitsuneApps.open('terminal', { multiInstance: true }) },
        { label: '🎨  Kitsune Rice Studio (Sway / SwayFX)', sub: 'Personalizar WM', run: () => window.KitsuneApps.open('settings') },
        { label: '⚙️  Exportar Configs Sway & SwayFX', sub: 'Generador Go', run: () => window.KitsuneApps.open('settings', { tab: 'dotfiles' }) },
        { label: '🗂️  Activar Modo Sway Tabbed (Pestañas)', sub: 'Alt+W', run: () => this.saveSettings({ ...this.settings, layoutMode: 'tabbed' }) },
        { label: '✨  Perfil SwayFX (Blur + Corner Radius)', sub: 'Compositor', run: () => this.saveSettings({ ...this.settings, compositor: 'swayfx' }) },
        { label: '▣   Perfil Sway Puro (Bordes Clásicos)', sub: 'Compositor', run: () => this.saveSettings({ ...this.settings, compositor: 'sway' }) },
        { label: '📊  Btop++ Monitor de Recursos Linux', sub: '/proc Go', run: () => window.KitsuneApps.open('sysmon') },
        { label: '📁  Yazi Explorador de Archivos', sub: 'Gestor Arch', run: () => window.KitsuneApps.open('files') },
        { label: '📝  Neovim Go Code Studio', sub: 'Editor IDE', run: () => window.KitsuneApps.open('nvim') },
        { label: '🎵  Kitsune Synth Radio (Lo-Fi Anime)', sub: 'WebAudio', run: () => window.KitsuneApps.open('music') },
        { label: '📦  Pacman / AUR & Instalador Sway/SwayFX', sub: 'PKGBUILD', run: () => window.KitsuneApps.open('pacman') },
        ...this.themes.map((t) => ({
          label: `🌸  Tema: ${t.name}`,
          sub: t.japaneseName,
          run: () => this.saveSettings({ ...this.settings, activeTheme: t.id }),
        })),
      ].filter((it) => !query || it.label.toLowerCase().includes(query) || it.sub.toLowerCase().includes(query));

      const listEl = document.getElementById('rofi-list');
      listEl.innerHTML = entries.map((it, idx) => `
        <button class="rofi-item ${idx === 0 ? 'selected' : ''}" data-idx="${idx}">
          <span>${it.label}</span>
          <span class="rofi-item-sub">${it.sub}</span>
        </button>
      `).join('');

      listEl.querySelectorAll('.rofi-item').forEach((btn) => {
        btn.addEventListener('click', () => {
          const item = entries[parseInt(btn.dataset.idx, 10)];
          document.getElementById('rofi-overlay').classList.add('hidden');
          if (item) item.run();
        });
      });
    }

    pushToast(title, body) {
      const item = { title, body, time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) };
      this.notifications.unshift(item);
      if (this.notifications.length > 20) this.notifications.pop();
      this.renderDrawerNotifs();

      const stack = document.getElementById('toast-stack');
      const el = document.createElement('div');
      el.className = 'kitsune-toast';
      el.innerHTML = `
        <div class="toast-title">${esc(title)}</div>
        <div class="toast-body">${esc(body)}</div>
      `;
      stack.appendChild(el);
      setTimeout(() => {
        el.style.opacity = '0';
        el.style.transition = 'opacity 0.25s ease';
        setTimeout(() => el.remove(), 260);
      }, 3400);
    }

    renderDrawerNotifs() {
      const badge = document.getElementById('notif-badge');
      badge.textContent = String(this.notifications.length);
      badge.classList.toggle('hidden', this.notifications.length === 0);

      const listEl = document.getElementById('swaync-list');
      if (!this.notifications.length) {
        listEl.innerHTML = `<div style="font-size:12px;color:var(--text-muted);text-align:center;margin-top:20px">Sin notificaciones pendientes</div>`;
        return;
      }
      listEl.innerHTML = this.notifications.map((n) => `
        <div style="padding:9px 11px;border-radius:9px;background:var(--bg-surface);border:1px solid rgba(255,255,255,0.06)">
          <div style="display:flex;justify-content:space-between;font-family:var(--font-mono);font-size:11px;color:var(--primary)">
            <strong>${esc(n.title)}</strong><span>${esc(n.time)}</span>
          </div>
          <div style="font-size:11.5px;margin-top:3px">${esc(n.body)}</div>
        </div>
      `).join('');
    }

    startClock() {
      const tick = () => {
        const now = new Date();
        const hm = now.toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit', hour12: false });
        const dateShort = now.toLocaleDateString('es-AR', { day: '2-digit', month: 'short' });
        const dateLong = now.toLocaleDateString('es-AR', { weekday: 'long', day: 'numeric', month: 'long' });

        document.getElementById('wb-clock-text').textContent = `󰥔 ${hm} • ${dateShort}`;
        document.getElementById('eww-clock').textContent = hm;
        document.getElementById('lock-time').textContent = hm;
        document.getElementById('lock-date').textContent = dateLong;
      };
      tick();
      setInterval(tick, 5000);
    }

    // ==================== 4-MODE ANIME PARTICLE CANVAS ====================
    initParticles() {
      this.canvas = document.getElementById('particle-canvas');
      this.ctx2d = this.canvas.getContext('2d');
      const resize = () => {
        this.canvas.width = window.innerWidth;
        this.canvas.height = window.innerHeight;
      };
      resize();
      window.addEventListener('resize', resize);
      this.resetParticles();

      const loop = () => {
        this.drawParticles();
        requestAnimationFrame(loop);
      };
      requestAnimationFrame(loop);
    }

    resetParticles() {
      const count = 42;
      this.particles = Array.from({ length: count }, () => ({
        x: Math.random() * window.innerWidth,
        y: Math.random() * window.innerHeight,
        s: 2 + Math.random() * 4.5,
        vx: (Math.random() - 0.5) * 1.2,
        vy: 1 + Math.random() * 2.5,
        rot: Math.random() * Math.PI * 2,
        alpha: 0.25 + Math.random() * 0.55,
      }));
    }

    drawParticles() {
      const c = this.ctx2d;
      const w = this.canvas.width;
      const h = this.canvas.height;
      c.clearRect(0, 0, w, h);

      if (!this.settings.particlesEnabled) return;

      const primary = this.activeTheme.primary || '#00f0ff';
      const secondary = this.activeTheme.secondary || '#ff2a85';
      const mode = this.particleMode;

      for (const p of this.particles) {
        c.save();
        c.globalAlpha = p.alpha;

        if (mode === 'rain') {
          c.strokeStyle = p.s > 4.5 ? secondary : primary;
          c.lineWidth = 1.2;
          c.beginPath();
          c.moveTo(p.x, p.y);
          c.lineTo(p.x - 4, p.y + p.s * 4.2);
          c.stroke();
          p.x -= 0.9;
          p.y += p.vy * 3.8;
          if (p.y > h) { p.y = -20; p.x = Math.random() * w; }
          if (p.x < 0) p.x = w;
        } else if (mode === 'sakura') {
          c.translate(p.x, p.y);
          c.rotate(p.rot);
          c.fillStyle = p.s > 4.2 ? secondary : primary;
          c.beginPath();
          c.ellipse(0, 0, p.s * 1.4, p.s * 0.75, 0, 0, Math.PI * 2);
          c.fill();
          p.x += Math.sin(p.rot) * 0.7 + 0.45;
          p.y += p.vy * 0.65;
          p.rot += 0.02;
          if (p.y > h + 15) { p.y = -15; p.x = Math.random() * w; }
          if (p.x > w + 15) p.x = -15;
        } else if (mode === 'embers') {
          c.fillStyle = p.s > 4 ? secondary : primary;
          c.shadowBlur = 8;
          c.shadowColor = primary;
          c.beginPath();
          c.arc(p.x, p.y, p.s * 0.55, 0, Math.PI * 2);
          c.fill();
          p.x += Math.sin(p.y * 0.02) * 0.6;
          p.y -= p.vy * 0.85;
          if (p.y < -10) { p.y = h + 10; p.x = Math.random() * w; }
        } else {
          // 'hud' Evangelion tactical markers
          c.strokeStyle = p.s > 4.5 ? secondary : primary;
          c.lineWidth = 1;
          const sz = p.s * 1.6;
          c.strokeRect(p.x - sz / 2, p.y - sz / 2, sz, sz);
          p.y -= p.vy * 0.35;
          if (p.y < -15) { p.y = h + 15; p.x = Math.random() * w; }
        }
        c.restore();
      }
    }
  }

  window.addEventListener('DOMContentLoaded', () => {
    window.KitsuneDesktop = new DesktopController();
    window.KitsuneDesktop.init();
  });
})();
