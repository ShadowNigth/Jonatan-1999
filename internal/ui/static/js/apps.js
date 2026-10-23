// Kitsune Go Wayland — Flagship Applications Suite (Rice Studio, Btop++, Yazi, Neovim Go Studio, Kitty)
(function () {
  function wsURL(path) {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${proto}//${window.location.host}${path}`;
  }

  function escapeHTML(str) {
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  const ANSI_FG = {
    30: '#1e1e2e', 31: '#ff4d79', 32: '#2ee59d', 33: '#ffd166',
    34: '#5ccfe6', 35: '#cba6f7', 36: '#00f0ff', 37: '#eef6ff',
    90: '#6c7086', 91: '#ff6e91', 92: '#5af7b0', 93: '#ffe082',
    94: '#89b4fa', 95: '#f5c2e7', 96: '#73fff8', 97: '#ffffff',
  };
  const ANSI_BG = {
    40: '#161622', 41: '#ff2a85', 42: '#00ffa3', 43: '#ffd166',
    44: '#00b4d8', 45: '#bd00ff', 46: '#00f0ff', 47: '#eef6ff',
    100: '#45475a', 101: '#f38ba8', 102: '#a6e3a1', 103: '#f9e2af',
    104: '#89b4fa', 105: '#f5c2e7', 106: '#94e2d5', 107: '#ffffff',
  };

  function ansiToHTML(raw) {
    const parts = String(raw).split(/\x1b\[([0-9;]*)m/);
    let html = escapeHTML(parts[0] || '');
    let fg = '';
    let bg = '';
    let bold = false;

    for (let i = 1; i < parts.length; i += 2) {
      const codes = (parts[i] || '0').split(';').map((n) => parseInt(n || '0', 10));
      for (let cIdx = 0; cIdx < codes.length; cIdx++) {
        const c = codes[cIdx];
        if (c === 0) {
          fg = ''; bg = ''; bold = false;
        } else if (c === 1) {
          bold = true;
        } else if (ANSI_FG[c]) {
          fg = ANSI_FG[c];
        } else if (ANSI_BG[c]) {
          bg = ANSI_BG[c];
        } else if (c === 38 && codes[cIdx + 1] === 5) {
          const n256 = codes[cIdx + 2] || 245;
          fg = n256 >= 232 ? '#8f9bb8' : 'var(--primary)';
          cIdx += 2;
        }
      }
      const chunk = escapeHTML(parts[i + 1] || '');
      if (!chunk) continue;
      if (!fg && !bg && !bold) {
        html += chunk;
      } else {
        const styles = [];
        if (fg) styles.push(`color:${fg}`);
        if (bg) styles.push(`background:${bg}`);
        if (bold) styles.push('font-weight:700');
        html += `<span style="${styles.join(';')}">${chunk}</span>`;
      }
    }
    return html;
  }

  const Apps = {
    open(appId, extra = {}) {
      const ctx = window.KitsuneDesktop;
      if (!ctx) return;

      switch (appId) {
        case 'settings':
          return this.openSettings(ctx, extra);
        case 'sysmon':
          return this.openSysmon(ctx, extra);
        case 'files':
          return this.openFiles(ctx, extra);
        case 'nvim':
          return this.openNvim(ctx, extra);
        case 'terminal':
          return this.openTerminal(ctx, extra);
        case 'music':
          return this.openMusic(ctx, extra);
        case 'pacman':
          return this.openPacman(ctx, extra);
        default:
          return this.openSettings(ctx, extra);
      }
    },

    // ==================== 1. KITSUNE RICE STUDIO ====================
    openSettings(ctx, extra = {}) {
      let activeTab = extra.tab || 'themes';
      let selectedDotfile = 'kitsune-wayland/config.toml';
      let cachedBundle = null;
      let unsubState = null;

      return ctx.wm.createWindow({
        appId: 'settings',
        title: 'Kitsune Rice Studio',
        icon: '🎨',
        width: 820,
        height: 540,
        focus: extra.focus !== false,
        onDestroy: () => {
          if (unsubState) unsubState();
        },
        mount: (body) => {
          const render = async () => {
            const s = ctx.settings;
            const themes = ctx.themes || [];

            body.innerHTML = `
              <div class="studio-wrap">
                <div class="studio-tabs">
                  <button class="studio-tab ${activeTab === 'themes' ? 'active' : ''}" data-tab="themes">🌸 Temas Anime, Blur &amp; Bordes Luminosos</button>
                  <button class="studio-tab ${activeTab === 'dotfiles' ? 'active' : ''}" data-tab="dotfiles">⚙️ Configs Kitsune Go Wayland</button>
                </div>
                <div class="studio-content" id="studio-pane"></div>
              </div>
            `;

            body.querySelectorAll('.studio-tab').forEach((btn) => {
              btn.addEventListener('click', () => {
                activeTab = btn.dataset.tab;
                render();
              });
            });

            const pane = body.querySelector('#studio-pane');

            if (activeTab === 'themes') {
              pane.innerHTML = `
                <div class="theme-grid">
                  ${themes.map((t) => `
                    <div class="theme-card ${t.id === s.activeTheme ? 'active' : ''}" data-theme="${t.id}">
                      <div class="theme-thumb" style="background-image:url('${t.wallpaper}')">
                        <span class="theme-jp-badge">${escapeHTML(t.japaneseName)}</span>
                      </div>
                      <div class="theme-card-body">
                        <div class="theme-card-title">${escapeHTML(t.name)} <span style="font-weight:400;font-size:11px;color:var(--text-muted)">(${escapeHTML(t.id)})</span></div>
                        <div class="theme-card-sub">${escapeHTML(t.subtitle)}</div>
                        <div class="theme-swatches">
                          <span style="background:${t.primary}" title="Primary"></span>
                          <span style="background:${t.secondary}" title="Secondary"></span>
                          <span style="background:${t.accent}" title="Accent"></span>
                          <span style="background:${t.bgSurface}" title="Surface"></span>
                        </div>
                      </div>
                    </div>
                  `).join('')}
                </div>

                <div class="controls-grid">
                  <div class="ctl-box">
                    <div class="ctl-label"><span>💧 Desenfoque Cristal Blur</span><strong id="lbl-blur">${s.blurRadius ?? 18}px</strong></div>
                    <input type="range" min="0" max="36" value="${s.blurRadius ?? 18}" id="rng-blur" />
                  </div>
                  <div class="ctl-box">
                    <div class="ctl-label"><span>🌟 Brillo Bordes Luminosos</span><strong id="lbl-glow">${s.glowIntensity ?? 72}%</strong></div>
                    <input type="range" min="0" max="100" value="${s.glowIntensity ?? 72}" id="rng-glow" />
                  </div>
                  <div class="ctl-box">
                    <div class="ctl-label"><span>▣ Grosor del Borde Neón</span><strong id="lbl-bsz">${s.borderSize ?? 2}px</strong></div>
                    <input type="range" min="1" max="6" value="${s.borderSize ?? 2}" id="rng-bsz" />
                  </div>
                  <div class="ctl-box">
                    <div class="ctl-label"><span>◜ Redondeo de Ventanas</span><strong id="lbl-rad">${s.borderRadius ?? 12}px</strong></div>
                    <input type="range" min="0" max="28" value="${s.borderRadius ?? 12}" id="rng-rad" />
                  </div>
                  <div class="ctl-box">
                    <div class="ctl-label"><span>🪟 Opacidad de Cristal</span><strong id="lbl-op">${s.windowOpacity ?? 92}%</strong></div>
                    <input type="range" min="65" max="100" value="${s.windowOpacity ?? 92}" id="rng-op" />
                  </div>
                  <div class="ctl-box">
                    <div class="ctl-label"><span>⇔ Margen Exterior (gaps)</span><strong id="lbl-go">${s.gapsOut ?? 12}px</strong></div>
                    <input type="range" min="0" max="36" value="${s.gapsOut ?? 12}" id="rng-go" />
                  </div>
                </div>

                <div class="toggle-row">
                  <button class="pill-toggle ${s.layoutMode === 'tabbed' ? 'on' : ''}" data-layout="tabbed">🗂️ Modo Tabbed (Sin Dock)</button>
                  <button class="pill-toggle ${s.layoutMode === 'tiling' ? 'on' : ''}" data-layout="tiling">⊞ Tiling Mosaico</button>
                  <button class="pill-toggle ${s.layoutMode === 'floating' ? 'on' : ''}" data-layout="floating">⧉ Flotante</button>
                  <button class="pill-toggle ${s.layoutMode === 'monocle' ? 'on' : ''}" data-layout="monocle">▣ Monocle</button>
                  <button class="pill-toggle ${s.particlesEnabled ? 'on' : ''}" id="tog-part">✨ Partículas Anime</button>
                  <button class="pill-toggle ${s.blurEnabled ? 'on' : ''}" id="tog-blur">💧 Cristal Blur</button>
                  <button class="pill-toggle ${s.borderGlow ? 'on' : ''}" id="tog-glow">🌟 Bordes Luminosos</button>
                </div>
              `;

              pane.querySelectorAll('.theme-card').forEach((card) => {
                card.addEventListener('click', async () => {
                  await ctx.saveSettings({ ...ctx.settings, activeTheme: card.dataset.theme });
                  cachedBundle = null;
                  render();
                });
              });

              const bindRange = (id, lblId, key, suffix, extraMutator) => {
                const el = pane.querySelector(id);
                const lbl = pane.querySelector(lblId);
                el.addEventListener('input', () => {
                  const val = parseInt(el.value, 10);
                  lbl.textContent = val + suffix;
                  const next = { ...ctx.settings, [key]: val };
                  if (extraMutator) extraMutator(next, val);
                  ctx.previewLocalSettings(next);
                });
                el.addEventListener('change', () => {
                  const val = parseInt(el.value, 10);
                  const next = { ...ctx.settings, [key]: val };
                  if (extraMutator) extraMutator(next, val);
                  ctx.saveSettings(next);
                });
              };

              bindRange('#rng-blur', '#lbl-blur', 'blurRadius', 'px', (n, v) => { n.blurEnabled = v > 0; });
              bindRange('#rng-glow', '#lbl-glow', 'glowIntensity', '%', (n, v) => { n.borderGlow = v > 0; });
              bindRange('#rng-bsz', '#lbl-bsz', 'borderSize', 'px');
              bindRange('#rng-rad', '#lbl-rad', 'borderRadius', 'px');
              bindRange('#rng-op', '#lbl-op', 'windowOpacity', '%');
              bindRange('#rng-go', '#lbl-go', 'gapsOut', 'px');

              pane.querySelectorAll('[data-layout]').forEach((btn) => {
                btn.addEventListener('click', async () => {
                  await ctx.saveSettings({ ...ctx.settings, layoutMode: btn.dataset.layout });
                  render();
                });
              });

              pane.querySelector('#tog-part').addEventListener('click', async () => {
                await ctx.saveSettings({ ...ctx.settings, particlesEnabled: !ctx.settings.particlesEnabled });
                render();
              });
              pane.querySelector('#tog-blur').addEventListener('click', async () => {
                await ctx.saveSettings({ ...ctx.settings, blurEnabled: !ctx.settings.blurEnabled });
                render();
              });
              pane.querySelector('#tog-glow').addEventListener('click', async () => {
                await ctx.saveSettings({ ...ctx.settings, borderGlow: !ctx.settings.borderGlow });
                render();
              });
            } else {
              pane.innerHTML = `<div style="font-family:var(--font-mono);font-size:12px;color:var(--text-muted)">Generando configuraciones nativas Kitsune Go Wayland...</div>`;
              const res = await fetch(`/api/ricer/preview?theme=${encodeURIComponent(s.activeTheme)}`);
              cachedBundle = await res.json();
              const files = cachedBundle.files || {};
              const preferredOrder = [
                'kitsune-wayland/config.toml',
                'kitsune-wayland/waybar.jsonc',
                'kitsune-wayland/waybar.css',
                'rofi/config.rasi',
                'btop/themes/kitsune.theme',
                'yazi/theme.toml',
                'nvim/init.lua',
                'kitty/kitty.conf',
                'fastfetch/config.jsonc',
                'gtk-4.0/gtk.css',
              ];
              const keys = preferredOrder.filter((k) => files[k]);
              if (!files[selectedDotfile] && keys.length) selectedDotfile = keys[0];

              pane.innerHTML = `
                <div class="dotfiles-bar">
                  <div class="dotfile-select">
                    ${keys.map((k) => `
                      <button class="dot-btn ${k === selectedDotfile ? 'active' : ''}" data-file="${k}">${k}</button>
                    `).join('')}
                  </div>
                  <div style="display:flex;gap:6px">
                    <button class="dot-btn" id="btn-copy-dot">📋 Copiar</button>
                    <button class="action-primary" id="btn-export-dot">⚡ Exportar Configs</button>
                  </div>
                </div>
                <pre class="code-preview" id="dot-code">${escapeHTML(files[selectedDotfile] || '')}</pre>
              `;

              pane.querySelectorAll('[data-file]').forEach((b) => {
                b.addEventListener('click', () => {
                  selectedDotfile = b.dataset.file;
                  render();
                });
              });

              pane.querySelector('#btn-copy-dot').addEventListener('click', () => {
                navigator.clipboard?.writeText(files[selectedDotfile] || '');
                ctx.pushToast('Copiado al portapapeles', selectedDotfile);
              });

              pane.querySelector('#btn-export-dot').addEventListener('click', async () => {
                await fetch('/api/ricer/export', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ themeId: s.activeTheme, outDir: 'dotfiles-kitsune-wayland' }),
                });
              });
            }
          };

          render();
          let lastTheme = ctx.settings.activeTheme;
          let lastLayout = ctx.settings.layoutMode;
          unsubState = ctx.onStateUpdate((nextS) => {
            if (nextS.activeTheme !== lastTheme || nextS.layoutMode !== lastLayout) {
              lastTheme = nextS.activeTheme;
              lastLayout = nextS.layoutMode;
              render();
            }
          });
        },
      });
    },

    // ==================== 2. BTOP++ MONITOR ====================
    openSysmon(ctx, extra = {}) {
      let unsub = null;
      const cpuHistory = [];

      return ctx.wm.createWindow({
        appId: 'sysmon',
        title: 'Btop++ Monitor',
        icon: '📊',
        width: 780,
        height: 520,
        focus: extra.focus !== false,
        onDestroy: () => {
          if (unsub) unsub();
        },
        mount: (body) => {
          const drawSparkline = () => {
            const canvas = body.querySelector('#btop-spark');
            if (!canvas) return;
            const c = canvas.getContext('2d');
            const w = canvas.width;
            const h = canvas.height;
            c.clearRect(0, 0, w, h);

            const primary = getComputedStyle(document.documentElement).getPropertyValue('--primary').trim() || '#00f0ff';
            const secondary = getComputedStyle(document.documentElement).getPropertyValue('--secondary').trim() || '#ff2a85';

            // Subtle grid lines
            c.strokeStyle = 'rgba(255,255,255,0.06)';
            c.lineWidth = 1;
            for (let y = 16; y < h; y += 18) {
              c.beginPath();
              c.moveTo(0, y);
              c.lineTo(w, y);
              c.stroke();
            }

            if (cpuHistory.length < 2) return;
            const step = w / Math.max(1, 39);
            const grad = c.createLinearGradient(0, 0, 0, h);
            grad.addColorStop(0, primary + '66');
            grad.addColorStop(1, secondary + '08');

            c.beginPath();
            cpuHistory.forEach((val, idx) => {
              const x = idx * step;
              const y = h - 6 - (Math.min(100, Math.max(0, val)) / 100) * (h - 14);
              if (idx === 0) c.moveTo(x, y);
              else c.lineTo(x, y);
            });
            c.strokeStyle = primary;
            c.lineWidth = 2;
            c.stroke();

            c.lineTo((cpuHistory.length - 1) * step, h);
            c.lineTo(0, h);
            c.closePath();
            c.fillStyle = grad;
            c.fill();
          };

          const updateView = (snap) => {
            if (!snap) return;
            cpuHistory.push(snap.cpuPercent || 0);
            if (cpuHistory.length > 40) cpuHistory.shift();

            body.innerHTML = `
              <div class="sysmon-wrap">
                <div class="btop-graph-box">
                  <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:6px;font-size:11.5px">
                    <span style="color:var(--primary);font-weight:700">󰻠 Btop++ Historial CPU en Vivo — ${escapeHTML(snap.cpuModel)}</span>
                    <span style="color:var(--secondary);font-weight:800">${snap.cpuPercent.toFixed(1)}% • LoadAvg ${escapeHTML(snap.loadAvg)}</span>
                  </div>
                  <canvas id="btop-spark" width="720" height="68" style="width:100%;height:68px;display:block"></canvas>
                </div>

                <div class="sysmon-cards">
                  <div class="metric-card">
                    <div class="metric-title">󰻠 CPU Total (${snap.cpuCores} núcleos)</div>
                    <div class="metric-val">${snap.cpuPercent.toFixed(1)}%</div>
                    <div class="metric-sub">Kernel ${escapeHTML(snap.kernel)}</div>
                    <div class="bar-track"><div class="bar-fill" style="width:${Math.min(100, snap.cpuPercent)}%"></div></div>
                  </div>
                  <div class="metric-card">
                    <div class="metric-title">󰍛 Memoria RAM</div>
                    <div class="metric-val">${snap.memPercent.toFixed(1)}%</div>
                    <div class="metric-sub">${snap.memUsedMB} MiB / ${snap.memTotalMB} MiB</div>
                    <div class="bar-track"><div class="bar-fill" style="width:${Math.min(100, snap.memPercent)}%"></div></div>
                  </div>
                  <div class="metric-card">
                    <div class="metric-title">󰋊 Disco Raíz (/)</div>
                    <div class="metric-val">${snap.diskPercent.toFixed(1)}%</div>
                    <div class="metric-sub">${snap.diskUsedGB} GiB / ${snap.diskTotalGB} GiB</div>
                    <div class="bar-track"><div class="bar-fill" style="width:${Math.min(100, snap.diskPercent)}%"></div></div>
                  </div>
                  <div class="metric-card">
                    <div class="metric-title">🐹 Kitsune Go &amp; Red</div>
                    <div class="metric-val">${snap.goroutines} <span style="font-size:12px">goroutines</span></div>
                    <div class="metric-sub">↓ ${snap.netRxKBps} KB/s • ↑ ${snap.netTxKBps} KB/s</div>
                    <div class="metric-sub">${escapeHTML(snap.goVersion)} • Uptime ${escapeHTML(snap.uptimeHuman)}</div>
                  </div>
                </div>

                <div class="cores-grid">
                  ${(snap.perCore || []).slice(0, 8).map((c) => `
                    <div class="core-pill">
                      <div style="display:flex;justify-content:space-between">
                        <span>${escapeHTML(c.name)}</span>
                        <strong style="color:var(--primary)">${c.percent.toFixed(0)}%</strong>
                      </div>
                      <div class="bar-track"><div class="bar-fill" style="width:${Math.min(100, c.percent)}%"></div></div>
                    </div>
                  `).join('')}
                </div>

                <div>
                  <div style="font-size:11.5px;color:var(--text-muted);margin-bottom:6px">
                    Procesos Activos en Linux (/proc)
                  </div>
                  <table class="proc-table">
                    <thead>
                      <tr>
                        <th>PID</th>
                        <th>PROCESO</th>
                        <th>ESTADO</th>
                        <th>HILOS</th>
                        <th>MEMORIA RSS</th>
                      </tr>
                    </thead>
                    <tbody>
                      ${(snap.processes || []).map((p) => `
                        <tr>
                          <td>${p.pid}</td>
                          <td style="color:var(--primary);font-weight:700">${escapeHTML(p.name)}</td>
                          <td>${escapeHTML(p.state)}</td>
                          <td>${p.threads}</td>
                          <td>${p.memMB.toFixed(1)} MB</td>
                        </tr>
                      `).join('')}
                    </tbody>
                  </table>
                </div>
              </div>
            `;
            drawSparkline();
          };

          updateView(ctx.system);
          unsub = ctx.onTelemetry((snap) => updateView(snap));
        },
      });
    },

    // ==================== 3. YAZI FILE MANAGER ====================
    openFiles(ctx, extra = {}) {
      let currentPath = extra.path || '';
      let selectedRel = '';

      return ctx.wm.createWindow({
        appId: 'files',
        title: 'Yazi File Manager',
        icon: '📁',
        width: 780,
        height: 500,
        focus: extra.focus !== false,
        mount: (body) => {
          const iconFor = (e) => {
            if (e.isDir) return '📂';
            if (e.ext === 'go') return '🐹';
            if (e.ext === 'jpg' || e.ext === 'png') return '🖼️';
            if (e.ext === 'sh' || e.name === 'PKGBUILD') return '⚙️';
            if (e.ext === 'md' || e.ext === 'toml') return '📘';
            return '📄';
          };

          const previewEntry = async (entry) => {
            const headEl = body.querySelector('#yazi-prev-title');
            const bodyPre = body.querySelector('#yazi-prev-body');
            const openBtn = body.querySelector('#btn-yazi-nvim');
            if (!headEl || !bodyPre || !openBtn) return;

            headEl.textContent = entry.relPath;
            if (entry.isDir) {
              openBtn.classList.add('hidden');
              bodyPre.textContent = `📂 Directorio: ${entry.relPath}\nHaz doble clic o pulsa Enter para entrar.`;
              return;
            }
            if (entry.ext === 'jpg' || entry.ext === 'png') {
              openBtn.classList.add('hidden');
              const baseName = entry.name;
              bodyPre.innerHTML = `<div style="text-align:center"><img src="/wallpapers/${escapeHTML(baseName)}" style="max-width:100%;max-height:260px;border-radius:10px;border:1px solid var(--primary)" alt="${escapeHTML(baseName)}" /><div style="margin-top:8px;color:var(--text-muted)">${escapeHTML(entry.relPath)} (${(entry.size / 1024).toFixed(1)} KB)</div></div>`;
              return;
            }

            openBtn.classList.remove('hidden');
            openBtn.onclick = () => Apps.open('nvim', { filePath: entry.relPath });

            const res = await fetch(`/api/fs/read?path=${encodeURIComponent(entry.relPath)}`);
            const data = await res.json();
            bodyPre.textContent = (data.content || data.error || '').slice(0, 6000);
          };

          const loadDir = async (rel) => {
            const res = await fetch(`/api/fs/list?path=${encodeURIComponent(rel)}`);
            const data = await res.json();
            currentPath = data.path || '';
            const entries = data.entries || [];

            body.innerHTML = `
              <div class="files-wrap">
                <div class="files-toolbar">
                  <button class="dot-btn" id="btn-fs-up">⬆ Subir</button>
                  <button class="dot-btn" id="btn-fs-root">🏠 Raíz</button>
                  <div class="files-path">󰉋 ~/${escapeHTML(data.root)}${currentPath ? '/' + escapeHTML(currentPath) : ''}</div>
                </div>
                <div class="yazi-split">
                  <div class="yazi-list-pane" id="yazi-list">
                    ${entries.map((e, idx) => `
                      <div class="yazi-row ${idx === 0 ? 'selected' : ''}" data-idx="${idx}">
                        <span style="display:flex;align-items:center;gap:8px;min-width:0;overflow:hidden">
                          <span>${iconFor(e)}</span>
                          <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${escapeHTML(e.name)}</span>
                        </span>
                        <span style="font-size:10.5px;color:var(--text-muted);flex-shrink:0">${e.isDir ? 'DIR ❯' : `${(e.size / 1024).toFixed(1)} KB`}</span>
                      </div>
                    `).join('')}
                  </div>
                  <div class="yazi-preview-pane">
                    <div class="yazi-preview-head">
                      <span id="yazi-prev-title" style="color:var(--primary)">Vista previa Yazi</span>
                      <button class="action-primary hidden" id="btn-yazi-nvim" style="padding:3px 9px;font-size:11px">📝 Abrir en Neovim Go</button>
                    </div>
                    <div class="yazi-preview-body" id="yazi-prev-body">Selecciona un archivo para previsualizar su código...</div>
                  </div>
                </div>
              </div>
            `;

            body.querySelector('#btn-fs-up').addEventListener('click', () => {
              if (!currentPath) return;
              const parts = currentPath.split('/');
              parts.pop();
              loadDir(parts.join('/'));
            });
            body.querySelector('#btn-fs-root').addEventListener('click', () => loadDir(''));

            body.querySelectorAll('.yazi-row').forEach((row) => {
              const entry = entries[parseInt(row.dataset.idx, 10)];
              row.addEventListener('click', () => {
                body.querySelectorAll('.yazi-row').forEach((r) => r.classList.remove('selected'));
                row.classList.add('selected');
                selectedRel = entry.relPath;
                if (entry.isDir) {
                  loadDir(entry.relPath);
                } else {
                  previewEntry(entry);
                }
              });
              row.addEventListener('dblclick', () => {
                if (!entry.isDir && entry.ext !== 'jpg' && entry.ext !== 'png') {
                  Apps.open('nvim', { filePath: entry.relPath });
                }
              });
            });

            const firstFile = entries.find((e) => !e.isDir) || entries[0];
            if (firstFile && selectedRel !== firstFile.relPath) {
              previewEntry(firstFile);
            } else if (firstFile) {
              previewEntry(firstFile);
            }
          };

          loadDir(currentPath);
        },
      });
    },

    // ==================== 4. NEOVIM GO STUDIO ====================
    openNvim(ctx, extra = {}) {
      let currentFile = extra.filePath || 'cmd/kitsune-de/main.go';

      const loadFileIntoWin = async (win, path) => {
        currentFile = path;
        const res = await fetch(`/api/fs/read?path=${encodeURIComponent(path)}`);
        const data = await res.json();
        const content = data.content || `// Error abriendo ${path}: ${data.error || ''}`;
        const lines = content.split('\n').length;

        win.bodyEl.innerHTML = `
          <div class="nvim-wrap">
            <div class="nvim-topbar">
              <div style="display:flex;gap:5px;flex-wrap:wrap">
                <button class="dot-btn ${path === 'cmd/kitsune-de/main.go' ? 'active' : ''}" data-open="cmd/kitsune-de/main.go">cmd/kitsune-de/main.go</button>
                <button class="dot-btn ${path === 'internal/config/state.go' ? 'active' : ''}" data-open="internal/config/state.go">config/state.go</button>
                <button class="dot-btn ${path === 'internal/ricer/generator.go' ? 'active' : ''}" data-open="internal/ricer/generator.go">ricer/generator.go</button>
                <button class="dot-btn ${path === 'INSTALAR_EN_ARCH.sh' ? 'active' : ''}" data-open="INSTALAR_EN_ARCH.sh">INSTALAR_EN_ARCH.sh</button>
                <button class="dot-btn ${path === 'arch/PKGBUILD' ? 'active' : ''}" data-open="arch/PKGBUILD">PKGBUILD</button>
              </div>
              <div style="display:flex;gap:6px">
                <button class="dot-btn" id="btn-nvim-test">▶ go test ./...</button>
                <button class="action-primary" id="btn-nvim-save">💾 Guardar (:w)</button>
              </div>
            </div>
            <div class="nvim-editor-area">
              <div class="nvim-gutter" id="nvim-gutter">${Array.from({ length: lines }, (_, i) => i + 1).join('<br>')}</div>
              <textarea class="nvim-textarea" id="nvim-ta" spellcheck="false">${escapeHTML(content)}</textarea>
            </div>
            <div class="nvim-statusline">
              <div><span class="nvim-mode" id="nvim-mode-badge">NORMAL</span> &nbsp; 🐹 ${escapeHTML(path)}</div>
              <div style="color:var(--text-muted)">Neovim Go Studio • UTF-8 • <span id="nvim-line-count">${lines}</span> líneas</div>
            </div>
          </div>
        `;

        ctx.wm.setTitle(win, `Neovim Go Studio — ${path}`);
        const ta = win.bodyEl.querySelector('#nvim-ta');
        const gutter = win.bodyEl.querySelector('#nvim-gutter');
        const modeBadge = win.bodyEl.querySelector('#nvim-mode-badge');
        const lineCount = win.bodyEl.querySelector('#nvim-line-count');

        const saveCurrent = async () => {
          const saveRes = await fetch('/api/fs/write', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: currentFile, content: ta.value }),
          });
          const out = await saveRes.json();
          if (out.ok) {
            ctx.pushToast('Neovim Go Studio (:w)', `${currentFile} guardado en disco`);
          } else {
            ctx.pushToast('Error al guardar', out.error || 'Error desconocido');
          }
        };

        ta.addEventListener('focus', () => {
          modeBadge.textContent = 'INSERT';
          modeBadge.style.background = 'var(--secondary)';
        });
        ta.addEventListener('blur', () => {
          modeBadge.textContent = 'NORMAL';
          modeBadge.style.background = 'var(--primary)';
        });
        ta.addEventListener('keydown', (e) => {
          if (e.key === 'Escape') {
            ta.blur();
          } else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
            e.preventDefault();
            saveCurrent();
          }
        });
        ta.addEventListener('scroll', () => {
          gutter.scrollTop = ta.scrollTop;
        });
        ta.addEventListener('input', () => {
          const count = ta.value.split('\n').length;
          lineCount.textContent = String(count);
          gutter.innerHTML = Array.from({ length: count }, (_, i) => i + 1).join('<br>');
        });

        win.bodyEl.querySelectorAll('[data-open]').forEach((b) => {
          b.addEventListener('click', () => loadFileIntoWin(win, b.dataset.open));
        });

        win.bodyEl.querySelector('#btn-nvim-save').addEventListener('click', saveCurrent);
        win.bodyEl.querySelector('#btn-nvim-test').addEventListener('click', () => {
          Apps.open('terminal', { initialCommand: 'go test -v ./...' });
        });
      };

      return ctx.wm.createWindow({
        appId: 'nvim',
        title: `Neovim Go Studio — ${currentFile}`,
        icon: '📝',
        width: 780,
        height: 500,
        focus: extra.focus !== false,
        onReopen: (existingWin) => {
          if (extra.filePath) loadFileIntoWin(existingWin, extra.filePath);
        },
        mount: (_, winObj) => {
          loadFileIntoWin(winObj, currentFile);
        },
      });
    },

    // ==================== 5. KITTY TERMINAL ====================
    openTerminal(ctx, extra = {}) {
      let ws = null;
      const history = [];
      let histIdx = -1;

      return ctx.wm.createWindow({
        appId: 'terminal',
        title: 'Kitty Terminal — kitsune@arch ~',
        icon: '>_',
        width: 760,
        height: 490,
        focus: extra.focus !== false,
        multiInstance: Boolean(extra.multiInstance),
        onDestroy: () => {
          if (ws) try { ws.close(); } catch (_) {}
        },
        mount: (body, winObj) => {
          body.innerHTML = `
            <div class="term-wrap">
              <div class="term-quickbar">
                <button class="term-chip" data-cmd="fastfetch">fastfetch</button>
                <button class="term-chip" data-cmd="kitsune-ctl status">kitsune-ctl status</button>
                <button class="term-chip" data-cmd="kitsune-ctl theme next">theme next</button>
                <button class="term-chip" data-cmd="kitsune-ctl blur 24">blur 24px</button>
                <button class="term-chip" data-cmd="kitsune-ctl glow 90">glow 90%</button>
                <button class="term-chip" data-cmd="kitsune-ctl layout tabbed">layout tabbed</button>
                <button class="term-chip" data-cmd="go test ./...">go test ./...</button>
              </div>
              <div class="term-output"></div>
              <form class="term-input-row">
                <span class="term-prompt">󰣇 kitsune <span class="tp-cwd">~</span> ❯</span>
                <input class="term-cmd-input" type="text" placeholder="Escribe un comando Linux o kitsune-ctl..." autocomplete="off" spellcheck="false" />
              </form>
            </div>
          `;

          const outEl = body.querySelector('.term-output');
          const formEl = body.querySelector('.term-input-row');
          const inputEl = body.querySelector('.term-cmd-input');
          const cwdEl = body.querySelector('.tp-cwd');

          const appendAnsi = (txt) => {
            const div = document.createElement('div');
            div.innerHTML = ansiToHTML(txt);
            outEl.appendChild(div);
            outEl.scrollTop = outEl.scrollHeight;
          };

          const sendCmd = (cmd) => {
            if (!cmd.trim()) return;
            history.push(cmd);
            histIdx = history.length;
            appendAnsi(`\x1b[1;36m❯\x1b[0m \x1b[1m${cmd}\x1b[0m`);
            if (ws && ws.readyState === WebSocket.OPEN) {
              ws.send(JSON.stringify({ type: 'exec', command: cmd }));
            } else {
              appendAnsi('\x1b[33m[reconectando sesión Go WebSocket...]\x1b[0m');
              connectWS(() => ws.send(JSON.stringify({ type: 'exec', command: cmd })));
            }
          };

          const connectWS = (onOpenCb) => {
            ws = new WebSocket(wsURL('/ws/terminal'));
            ws.onopen = () => {
              if (onOpenCb) onOpenCb();
              if (extra.initialCommand) {
                const c = extra.initialCommand;
                extra.initialCommand = '';
                setTimeout(() => sendCmd(c), 180);
              }
            };
            ws.onmessage = (ev) => {
              try {
                const msg = JSON.parse(ev.data);
                if (msg.cwd) {
                  cwdEl.textContent = msg.cwd;
                  ctx.wm.setTitle(winObj, `Kitty Terminal — ${msg.cwd}`);
                }
                if (msg.clear) {
                  outEl.innerHTML = '';
                  return;
                }
                if (msg.output) {
                  appendAnsi(msg.output);
                }
              } catch (_) {}
            };
          };

          connectWS();

          formEl.addEventListener('submit', (e) => {
            e.preventDefault();
            const val = inputEl.value;
            inputEl.value = '';
            sendCmd(val);
          });

          inputEl.addEventListener('keydown', (e) => {
            if (e.key === 'ArrowUp' && history.length) {
              e.preventDefault();
              histIdx = Math.max(0, histIdx - 1);
              inputEl.value = history[histIdx] || '';
            } else if (e.key === 'ArrowDown' && history.length) {
              e.preventDefault();
              histIdx = Math.min(history.length, histIdx + 1);
              inputEl.value = history[histIdx] || '';
            }
          });

          body.querySelectorAll('.term-chip').forEach((btn) => {
            btn.addEventListener('click', () => {
              sendCmd(btn.dataset.cmd);
              inputEl.focus();
            });
          });

          body.addEventListener('click', () => inputEl.focus());
          if (extra.focus !== false) {
            setTimeout(() => inputEl.focus(), 60);
          }
        },
      });
    },

    // ==================== 6. KITSUNE LO-FI & SYNTH PLAYER ====================
    openMusic(ctx, extra = {}) {
      let animId = null;
      let unsub = null;

      return ctx.wm.createWindow({
        appId: 'music',
        title: 'Kitsune Synth Radio',
        icon: '🎵',
        width: 520,
        height: 440,
        focus: extra.focus !== false,
        onDestroy: () => {
          if (animId) cancelAnimationFrame(animId);
          if (unsub) unsub();
        },
        mount: (body) => {
          const audio = window.KitsuneAudio;
          const renderUI = () => {
            const cur = audio.currentTrack();
            body.innerHTML = `
              <div class="music-wrap">
                <div class="music-hero">
                  <img class="music-cover" src="/wallpapers/kitsune-avatar.jpg" alt="Cover" />
                  <div style="flex:1">
                    <div style="font-family:var(--font-mono);font-size:11px;color:var(--primary)">${escapeHTML(cur.jp)} • ${cur.bpm} BPM</div>
                    <div style="font-weight:800;font-size:15px;margin:2px 0">${escapeHTML(cur.title)}</div>
                    <div style="font-size:11.5px;color:var(--text-muted)">Sintetizador WebAudio Procedural</div>
                  </div>
                  <button class="action-primary" id="btn-music-play">${audio.playing ? '⏸ Pausar' : '▶ Reproducir'}</button>
                </div>

                <canvas class="music-canvas" id="music-spec" width="440" height="84"></canvas>

                <div style="display:flex;align-items:center;gap:10px;font-family:var(--font-mono);font-size:11.5px">
                  <span>󰕾 Volumen</span>
                  <input type="range" min="0" max="100" value="${Math.round(audio.volume * 100)}" id="rng-vol" style="flex:1;accent-color:var(--primary)" />
                </div>

                <div class="track-list">
                  ${audio.tracks().map((t, idx) => `
                    <button class="track-btn ${idx === audio.trackIndex ? 'active' : ''}" data-idx="${idx}">
                      <span>${idx === audio.trackIndex && audio.playing ? '🔊 ' : '♪ '}${escapeHTML(t.title)}</span>
                      <span style="color:var(--text-muted)">${escapeHTML(t.jp)}</span>
                    </button>
                  `).join('')}
                </div>
              </div>
            `;

            body.querySelector('#btn-music-play').addEventListener('click', () => audio.toggle());
            body.querySelector('#rng-vol').addEventListener('input', (e) => {
              audio.setVolume(parseInt(e.target.value, 10) / 100);
            });
            body.querySelectorAll('.track-btn').forEach((btn) => {
              btn.addEventListener('click', () => {
                audio.selectTrack(parseInt(btn.dataset.idx, 10));
                if (!audio.playing) audio.play();
              });
            });

            const canvas = body.querySelector('#music-spec');
            const c2d = canvas.getContext('2d');
            const bins = new Uint8Array(32);

            if (animId) cancelAnimationFrame(animId);
            const draw = () => {
              audio.readSpectrum(bins);
              c2d.clearRect(0, 0, canvas.width, canvas.height);
              const primary = getComputedStyle(document.documentElement).getPropertyValue('--primary').trim() || '#00f0ff';
              const secondary = getComputedStyle(document.documentElement).getPropertyValue('--secondary').trim() || '#ff2a85';
              const grad = c2d.createLinearGradient(0, canvas.height, 0, 0);
              grad.addColorStop(0, primary);
              grad.addColorStop(1, secondary);
              c2d.fillStyle = grad;

              const barW = (canvas.width - 24) / 24;
              for (let i = 0; i < 24; i++) {
                const v = bins[i] || 4;
                const h = Math.max(4, (v / 255) * (canvas.height - 14));
                c2d.fillRect(12 + i * barW, canvas.height - h - 6, barW - 4, h);
              }
              animId = requestAnimationFrame(draw);
            };
            draw();
          };

          renderUI();
          unsub = audio.onChange(() => renderUI());
        },
      });
    },

    // ==================== 7. PACMAN & ARCH LINUX INSTALLER ====================
    openPacman(ctx, extra = {}) {
      let query = '';

      return ctx.wm.createWindow({
        appId: 'pacman',
        title: 'Pacman & Instalador Kitsune Go Wayland',
        icon: '📦',
        width: 740,
        height: 490,
        focus: extra.focus !== false,
        mount: (body) => {
          const pkgs = ctx.archPackages || [];

          const render = () => {
            const filtered = pkgs.filter((p) =>
              !query ||
              p.name.toLowerCase().includes(query) ||
              p.description.toLowerCase().includes(query) ||
              p.category.toLowerCase().includes(query)
            );

            body.innerHTML = `
              <div class="pacman-wrap">
                <div class="arch-banner-box">
                  <div style="font-family:var(--font-mono);font-size:12.5px;font-weight:800;color:var(--primary);margin-bottom:4px">
                    󰣇 Instalar Kitsune Go Wayland (Tabbed • Sin Sway • Sin Dock) en Arch Linux
                  </div>
                  <div style="font-size:12px;color:var(--text-main);line-height:1.45;margin-bottom:8px">
                    Escritorio independiente <strong>Kitsune Go Wayland</strong> con barra superior Waybar Kanji, Rofi, Blur y bordes luminosos en vivo:
                  </div>
                  <div style="display:flex;gap:6px;flex-wrap:wrap">
                    <a class="action-primary" href="/download/kitsune-go-wayland-arch-linux.zip" download="kitsune-go-wayland-arch-linux.zip" style="text-decoration:none;display:inline-flex;align-items:center">⬇️ Descargar ZIP Kitsune Go Wayland</a>
                    <button class="dot-btn" id="btn-view-install">📜 Ver INSTALAR_EN_ARCH.sh</button>
                    <button class="dot-btn" id="btn-view-pkgbuild">📦 Ver arch/PKGBUILD</button>
                    <button class="dot-btn" id="btn-run-syu">⚡ pacman -Syu</button>
                  </div>
                </div>

                <input class="pkg-search" id="pkg-q" type="text" value="${escapeHTML(query)}" placeholder="Buscar paquete de Kitsune Go Wayland..." />

                <div class="pkg-list">
                  ${filtered.map((p) => `
                    <div class="pkg-card">
                      <div>
                        <div style="display:flex;align-items:center;gap:7px;margin-bottom:3px">
                          <span class="pkg-repo">${escapeHTML(p.repo)}</span>
                          <strong style="font-family:var(--font-mono);font-size:13px">${escapeHTML(p.name)}</strong>
                          <span style="font-family:var(--font-mono);font-size:11px;color:var(--success)">${escapeHTML(p.version)}</span>
                        </div>
                        <div style="font-size:11.5px;color:var(--text-muted)">${escapeHTML(p.description)}</div>
                      </div>
                      <button class="dot-btn" data-install="${escapeHTML(p.name)}">${p.installed ? '✓ Instalado' : 'Instalar'}</button>
                    </div>
                  `).join('')}
                </div>
              </div>
            `;

            const input = body.querySelector('#pkg-q');
            input.addEventListener('input', () => {
              query = input.value.toLowerCase();
              render();
              const nextInput = body.querySelector('#pkg-q');
              nextInput.focus();
              nextInput.setSelectionRange(nextInput.value.length, nextInput.value.length);
            });

            body.querySelector('#btn-view-install').addEventListener('click', () => {
              Apps.open('nvim', { filePath: 'INSTALAR_EN_ARCH.sh' });
            });
            body.querySelector('#btn-view-pkgbuild').addEventListener('click', () => {
              Apps.open('nvim', { filePath: 'arch/PKGBUILD' });
            });
            body.querySelector('#btn-run-syu').addEventListener('click', () => {
              Apps.open('terminal', { initialCommand: 'pacman -Syu' });
            });

            body.querySelectorAll('[data-install]').forEach((btn) => {
              btn.addEventListener('click', () => {
                const pkgName = btn.dataset.install;
                Apps.open('terminal', { initialCommand: `pacman -S ${pkgName}` });
              });
            });
          };

          render();
        },
      });
    },
  };

  window.KitsuneApps = Apps;
})();
