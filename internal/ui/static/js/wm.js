// Kitsune Go Wayland Window Manager (Tabbed First • Tiling • Floating • Monocle — No Bottom Dock)
(function () {
  function esc(str) {
    return String(str ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  class WindowManager {
    constructor(stageEl) {
      this.stage = stageEl;
      this.windows = [];
      this.tabbedWindows = [];
      this.nextId = 1;
      this.zCounter = 30;
      this.focusedId = null;

      this.layoutMode = 'tabbed'; // 'tabbed' | 'tiling' | 'floating' | 'monocle'
      this.activeWorkspace = 1;
      this.gapsIn = 8;
      this.gapsOut = 12;

      this.onStateChange = null;
      this.onRequestLauncher = null;
      this.onQuickOpenApp = null;
      window.addEventListener('resize', () => this.relayout());
    }

    applySettings(s) {
      if (!s) return;
      this.layoutMode = s.layoutMode || 'tabbed';
      this.activeWorkspace = s.activeWorkspace || 1;
      this.gapsIn = typeof s.gapsIn === 'number' ? s.gapsIn : 8;
      this.gapsOut = typeof s.gapsOut === 'number' ? s.gapsOut : 12;
      this.relayout();
      this.notifyChange();
    }

    createWindow(opts) {
      // Reuse existing singleton window unless multiInstance is true
      if (!opts.multiInstance) {
        const existing = this.windows.find((w) => w.appId === opts.appId);
        if (existing) {
          if (existing.workspace !== this.activeWorkspace && opts.switchWorkspace !== false) {
            existing.workspace = this.activeWorkspace;
          }
          if (opts.focus === false) {
            this.relayout();
            return existing;
          }
          this.focus(existing.id);
          this.relayout();
          if (opts.onReopen) opts.onReopen(existing);
          return existing;
        }
      }

      const id = 'win-' + this.nextId++;
      const stageRect = this.stage.getBoundingClientRect();
      const defW = Math.min(opts.width || 760, Math.max(360, stageRect.width - 40));
      const defH = Math.min(opts.height || 500, Math.max(260, stageRect.height - 40));
      const offset = ((this.windows.length % 5) * 26);

      const el = document.createElement('section');
      el.className = 'kitsune-window';
      el.dataset.winId = id;

      el.innerHTML = `
        <header class="win-titlebar">
          <div class="win-title-left">
            <span class="win-app-icon">${opts.icon || '🦊'}</span>
            <span class="win-title-text">${esc(opts.title || 'Ventana')}</span>
            <span class="win-mode-tag">TABBED</span>
          </div>
          <div class="win-controls">
            <button class="win-ctl-btn btn-master" title="Mover al inicio de las pestañas / Master">⇄</button>
            <button class="win-ctl-btn btn-float" title="Alternar Ventana Flotante / Pestaña (Alt+V)">⧉</button>
            <button class="win-ctl-btn btn-max" title="Maximizar">□</button>
            <button class="win-ctl-btn btn-close" title="Cerrar ventana (Alt+Q)">✕</button>
          </div>
        </header>
        <div class="win-body"></div>
        <div class="win-resize-handle" title="Redimensionar"></div>
      `;

      this.stage.appendChild(el);

      const winObj = {
        id,
        appId: opts.appId,
        title: opts.title || 'Ventana',
        icon: opts.icon || '🦊',
        workspace: opts.workspace || this.activeWorkspace,
        isFloatingOverride: Boolean(opts.floating),
        maximized: false,
        floatRect: {
          x: Math.max(20, (stageRect.width - defW) / 2 + offset - 30),
          y: Math.max(16, (stageRect.height - defH) / 2 + offset - 20),
          w: defW,
          h: defH,
        },
        el,
        bodyEl: el.querySelector('.win-body'),
        modeTagEl: el.querySelector('.win-mode-tag'),
        floatBtnEl: el.querySelector('.btn-float'),
        onDestroy: opts.onDestroy || null,
      };

      this.bindWindowEvents(winObj);
      this.windows.push(winObj);

      if (typeof opts.mount === 'function') {
        opts.mount(winObj.bodyEl, winObj);
      }

      if (opts.focus !== false) {
        this.focus(id);
      } else if (!this.focusedId) {
        this.focusedId = id;
      }
      this.relayout();
      return winObj;
    }

    bindWindowEvents(win) {
      const titlebar = win.el.querySelector('.win-titlebar');
      const btnClose = win.el.querySelector('.btn-close');
      const btnMax = win.el.querySelector('.btn-max');
      const btnFloat = win.el.querySelector('.btn-float');
      const btnMaster = win.el.querySelector('.btn-master');
      const resizeHandle = win.el.querySelector('.win-resize-handle');

      win.el.addEventListener('pointerdown', () => {
        if (this.focusedId !== win.id) {
          this.focus(win.id);
        }
      });

      btnClose.addEventListener('click', (e) => {
        e.stopPropagation();
        this.close(win.id);
      });

      btnMax.addEventListener('click', (e) => {
        e.stopPropagation();
        win.maximized = !win.maximized;
        this.relayout();
      });

      btnFloat.addEventListener('click', (e) => {
        e.stopPropagation();
        win.maximized = false;
        win.isFloatingOverride = !win.isFloatingOverride;
        this.focus(win.id);
        this.relayout();
      });

      btnMaster.addEventListener('click', (e) => {
        e.stopPropagation();
        const idx = this.windows.indexOf(win);
        if (idx > 0) {
          this.windows.splice(idx, 1);
          this.windows.unshift(win);
          win.isFloatingOverride = false;
          this.relayout();
        }
      });

      titlebar.addEventListener('dblclick', (e) => {
        if (e.target.closest('.win-controls')) return;
        win.maximized = !win.maximized;
        this.relayout();
      });

      // Pointer drag (supports floating drag + pop-out if dragged far)
      titlebar.addEventListener('pointerdown', (e) => {
        if (e.button !== 0 || e.target.closest('.win-controls')) return;
        this.focus(win.id);
        const startX = e.clientX;
        const startY = e.clientY;
        let detached = this.isEffectivelyFloating(win);
        let baseRect = { ...win.floatRect };

        if (detached) {
          const cur = win.el.getBoundingClientRect();
          const st = this.stage.getBoundingClientRect();
          baseRect = { x: cur.left - st.left, y: cur.top - st.top, w: cur.width, h: cur.height };
          win.floatRect = { ...baseRect };
        }

        const onMove = (ev) => {
          const dx = ev.clientX - startX;
          const dy = ev.clientY - startY;

          if (!detached && Math.hypot(dx, dy) > 26) {
            detached = true;
            win.maximized = false;
            win.isFloatingOverride = true;
            const st = this.stage.getBoundingClientRect();
            const w = Math.min(720, st.width * 0.64);
            const h = Math.min(480, st.height * 0.70);
            baseRect = {
              x: ev.clientX - st.left - w / 2,
              y: Math.max(6, ev.clientY - st.top - 18),
              w,
              h,
            };
            this.relayout();
          }

          if (detached && !win.maximized) {
            win.el.classList.add('dragging');
            const st = this.stage.getBoundingClientRect();
            win.floatRect.x = Math.max(-baseRect.w + 100, Math.min(st.width - 80, baseRect.x + dx));
            win.floatRect.y = Math.max(0, Math.min(st.height - 38, baseRect.y + dy));
            this.applyRect(win, win.floatRect.x, win.floatRect.y, win.floatRect.w, win.floatRect.h);
          }
        };

        const onUp = () => {
          win.el.classList.remove('dragging');
          window.removeEventListener('pointermove', onMove);
          window.removeEventListener('pointerup', onUp);
        };

        window.addEventListener('pointermove', onMove);
        window.addEventListener('pointerup', onUp);
      });

      // Corner Resize
      resizeHandle.addEventListener('pointerdown', (e) => {
        e.stopPropagation();
        if (e.button !== 0) return;
        this.focus(win.id);
        if (!this.isEffectivelyFloating(win)) {
          const cur = win.el.getBoundingClientRect();
          const st = this.stage.getBoundingClientRect();
          win.isFloatingOverride = true;
          win.floatRect = { x: cur.left - st.left, y: cur.top - st.top, w: cur.width, h: cur.height };
          this.relayout();
        }
        const startX = e.clientX;
        const startY = e.clientY;
        const startW = win.floatRect.w;
        const startH = win.floatRect.h;
        win.el.classList.add('resizing');

        const onMove = (ev) => {
          win.floatRect.w = Math.max(320, startW + (ev.clientX - startX));
          win.floatRect.h = Math.max(220, startH + (ev.clientY - startY));
          this.applyRect(win, win.floatRect.x, win.floatRect.y, win.floatRect.w, win.floatRect.h);
        };
        const onUp = () => {
          win.el.classList.remove('resizing');
          window.removeEventListener('pointermove', onMove);
          window.removeEventListener('pointerup', onUp);
        };
        window.addEventListener('pointermove', onMove);
        window.addEventListener('pointerup', onUp);
      });
    }

    isEffectivelyFloating(win) {
      return this.layoutMode === 'floating' || win.isFloatingOverride;
    }

    setTitle(win, newTitle) {
      win.title = newTitle;
      const el = win.el.querySelector('.win-title-text');
      if (el) el.textContent = newTitle;
      if (this.layoutMode === 'tabbed') {
        this.renderTabbar();
      }
      this.notifyChange();
    }

    cycleTab(delta = 1) {
      const list = this.visibleWindows();
      if (list.length < 2) return;
      const idx = list.findIndex((w) => w.id === this.focusedId);
      const next = list[(idx + delta + list.length) % list.length];
      if (next) this.focus(next.id);
    }

    focus(id) {
      this.focusedId = id;
      this.windows.forEach((w) => {
        if (w.id === id) {
          w.el.classList.add('focused');
          const baseZ = this.isEffectivelyFloating(w) ? 120 : 30;
          w.el.style.zIndex = String(baseZ + (++this.zCounter % 80));
        } else {
          w.el.classList.remove('focused');
        }
      });
      if (this.layoutMode === 'tabbed' || this.layoutMode === 'monocle') {
        this.relayout();
      }
      this.notifyChange();
    }

    close(id) {
      const idx = this.windows.findIndex((w) => w.id === id);
      if (idx < 0) return;
      const [win] = this.windows.splice(idx, 1);
      if (typeof win.onDestroy === 'function') {
        try { win.onDestroy(); } catch (_) {}
      }
      win.el.remove();
      if (this.focusedId === id) {
        const visible = this.visibleWindows();
        this.focusedId = visible.length ? visible[visible.length - 1].id : null;
        if (this.focusedId) this.focus(this.focusedId);
      }
      this.relayout();
      this.notifyChange();
    }

    closeFocused() {
      if (this.focusedId) this.close(this.focusedId);
    }

    toggleFocusedFloat() {
      const win = this.windows.find((w) => w.id === this.focusedId);
      if (!win) return;
      win.isFloatingOverride = !win.isFloatingOverride;
      this.relayout();
    }

    visibleWindows() {
      return this.windows.filter((w) => w.workspace === this.activeWorkspace);
    }

    relayout() {
      const st = this.stage.getBoundingClientRect();
      const stageW = Math.max(320, st.width);
      const stageH = Math.max(240, st.height);
      const go = this.gapsOut;
      const gi = this.gapsIn;

      // Hide windows on other workspaces
      this.windows.forEach((w) => {
        w.el.classList.remove('tab-hidden');
        if (w.workspace !== this.activeWorkspace) {
          w.el.style.display = 'none';
          return;
        }
        w.el.style.display = 'flex';
      });

      const vis = this.visibleWindows();
      // Ensure focusedId belongs to active workspace if any window exists
      if (vis.length > 0 && !vis.some((w) => w.id === this.focusedId)) {
        this.focusedId = vis[vis.length - 1].id;
        vis.forEach((w) => w.el.classList.toggle('focused', w.id === this.focusedId));
      }

      const tiled = [];
      const tabbed = [];
      const floating = [];
      const tabbar = document.getElementById('kitsune-tabbar');

      vis.forEach((w) => {
        const btnFloat = w.el.querySelector('.btn-float');
        if (w.maximized) {
          w.modeTagEl.textContent = 'MAX';
          this.applyRect(w, go, go, stageW - go * 2, stageH - go * 2);
          w.el.style.zIndex = '220';
          return;
        }
        if (this.layoutMode === 'monocle' && !w.isFloatingOverride) {
          w.modeTagEl.textContent = 'MONOCLE';
          this.applyRect(w, go, go, stageW - go * 2, stageH - go * 2);
          if (w.id !== this.focusedId) w.el.classList.add('tab-hidden');
          return;
        }
        if (this.layoutMode === 'tabbed' && !w.isFloatingOverride) {
          w.modeTagEl.textContent = 'TABBED';
          btnFloat.classList.remove('active-float');
          tabbed.push(w);
          return;
        }
        if (this.isEffectivelyFloating(w)) {
          w.modeTagEl.textContent = 'FLOAT';
          btnFloat.classList.add('active-float');
          floating.push(w);
        } else {
          w.modeTagEl.textContent = 'TILE';
          btnFloat.classList.remove('active-float');
          tiled.push(w);
        }
      });

      if (tabbar) {
        if (this.layoutMode === 'tabbed') {
          tabbar.classList.remove('hidden');
          tabbar.style.left = `${go}px`;
          tabbar.style.top = `${go}px`;
          tabbar.style.width = `${Math.max(260, stageW - go * 2)}px`;
          this.tabbedWindows = tabbed;

          let activeTabId = this.focusedId;
          if (tabbed.length > 0 && !tabbed.some((w) => w.id === activeTabId)) {
            activeTabId = tabbed[tabbed.length - 1].id;
            this.focusedId = activeTabId;
          }

          this.renderTabbar(activeTabId);

          const tabBodyY = go + 42;
          const tabBodyH = Math.max(180, stageH - go * 2 - 42);
          const tabBodyW = Math.max(260, stageW - go * 2);

          tabbed.forEach((w) => {
            this.applyRect(w, go, tabBodyY, tabBodyW, tabBodyH);
            if (w.id === activeTabId) {
              w.el.classList.remove('tab-hidden');
              w.el.classList.add('focused');
            } else {
              w.el.classList.add('tab-hidden');
              w.el.classList.remove('focused');
            }
          });
        } else {
          tabbar.classList.add('hidden');
          this.tabbedWindows = [];
        }
      }

      // Layout Tiled Windows
      const areaX = go;
      const areaY = go;
      const areaW = Math.max(260, stageW - go * 2);
      const areaH = Math.max(200, stageH - go * 2);

      const n = tiled.length;
      if (n === 1) {
        this.applyRect(tiled[0], areaX, areaY, areaW, areaH);
      } else if (n === 2) {
        const masterW = Math.floor((areaW - gi) * 0.54);
        const stackW = areaW - gi - masterW;
        this.applyRect(tiled[0], areaX, areaY, masterW, areaH);
        this.applyRect(tiled[1], areaX + masterW + gi, areaY, stackW, areaH);
      } else if (n === 3) {
        const masterW = Math.floor((areaW - gi) * 0.52);
        const stackW = areaW - gi - masterW;
        const halfH = Math.floor((areaH - gi) / 2);
        this.applyRect(tiled[0], areaX, areaY, masterW, areaH);
        this.applyRect(tiled[1], areaX + masterW + gi, areaY, stackW, halfH);
        this.applyRect(tiled[2], areaX + masterW + gi, areaY + halfH + gi, stackW, areaH - gi - halfH);
      } else if (n >= 4) {
        const cols = Math.ceil(Math.sqrt(n));
        const rows = Math.ceil(n / cols);
        const cellW = Math.floor((areaW - gi * (cols - 1)) / cols);
        const cellH = Math.floor((areaH - gi * (rows - 1)) / rows);
        tiled.forEach((w, idx) => {
          const r = Math.floor(idx / cols);
          const c = idx % cols;
          this.applyRect(
            w,
            areaX + c * (cellW + gi),
            areaY + r * (cellH + gi),
            cellW,
            cellH
          );
        });
      }

      // Position Floating Windows
      floating.forEach((w) => {
        const fw = Math.min(w.floatRect.w, stageW - 24);
        const fh = Math.min(w.floatRect.h, stageH - 20);
        const fx = Math.max(8, Math.min(stageW - fw - 8, w.floatRect.x));
        const fy = Math.max(6, Math.min(stageH - fh - 6, w.floatRect.y));
        this.applyRect(w, fx, fy, fw, fh);
      });
    }

    renderTabbar(activeTabId = this.focusedId) {
      const tabbar = document.getElementById('kitsune-tabbar');
      if (!tabbar) return;

      if (!this.tabbedWindows || this.tabbedWindows.length === 0) {
        tabbar.innerHTML = `
          <span style="font-family:var(--font-mono);font-size:11px;color:var(--text-muted);padding:0 8px">Workspace vacío — Abrir pestaña:</span>
          <button class="kitsune-tab-add" data-qapp="settings">🎨 Rice Studio</button>
          <button class="kitsune-tab-add" data-qapp="sysmon">📊 Btop++ Monitor</button>
          <button class="kitsune-tab-add" data-qapp="files">📁 Yazi Files</button>
          <button class="kitsune-tab-add" data-qapp="nvim">📝 Neovim Go</button>
          <button class="kitsune-tab-add" data-qapp="terminal">&gt;_ Kitty</button>
          <button class="kitsune-tab-add" id="btn-tab-rofi">󰣇 Rofi (Alt+Space)</button>
        `;
      } else {
        tabbar.innerHTML =
          this.tabbedWindows.map((w) => `
            <div class="kitsune-tab ${w.id === activeTabId ? 'active' : ''}" data-tab-win="${w.id}" role="tab">
              <span class="kitsune-tab-label">
                <span>${w.icon}</span>
                <span>${esc(w.title)}</span>
              </span>
              <span class="kitsune-tab-close" data-close-win="${w.id}" title="Cerrar pestaña">✕</span>
            </div>
          `).join('') +
          `<button class="kitsune-tab-add" id="btn-tab-rofi" title="Abrir nueva pestaña o aplicación con Rofi (Alt+Space)">＋ Rofi</button>`;
      }

      tabbar.querySelectorAll('[data-tab-win]').forEach((tabEl) => {
        tabEl.addEventListener('click', (e) => {
          if (e.target.closest('[data-close-win]')) return;
          this.focus(tabEl.dataset.tabWin);
        });
      });

      tabbar.querySelectorAll('[data-close-win]').forEach((closeEl) => {
        closeEl.addEventListener('click', (e) => {
          e.stopPropagation();
          this.close(closeEl.dataset.closeWin);
        });
      });

      tabbar.querySelectorAll('[data-qapp]').forEach((btn) => {
        btn.addEventListener('click', () => {
          if (typeof this.onQuickOpenApp === 'function') {
            this.onQuickOpenApp(btn.dataset.qapp);
          }
        });
      });

      const rofiBtn = tabbar.querySelector('#btn-tab-rofi');
      if (rofiBtn) {
        rofiBtn.addEventListener('click', () => {
          if (typeof this.onRequestLauncher === 'function') {
            this.onRequestLauncher();
          }
        });
      }
    }

    applyRect(win, x, y, w, h) {
      win.el.style.left = `${Math.round(x)}px`;
      win.el.style.top = `${Math.round(y)}px`;
      win.el.style.width = `${Math.round(w)}px`;
      win.el.style.height = `${Math.round(h)}px`;
    }

    notifyChange() {
      if (typeof this.onStateChange === 'function') {
        const focused = this.windows.find((w) => w.id === this.focusedId && w.workspace === this.activeWorkspace);
        const occupied = new Set(this.windows.map((w) => w.workspace));
        const runningApps = new Set(this.windows.map((w) => w.appId));
        this.onStateChange({ focused, occupied, runningApps });
      }
    }
  }

  window.KitsuneWM = WindowManager;
})();
