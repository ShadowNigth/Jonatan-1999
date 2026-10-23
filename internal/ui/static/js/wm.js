// KitsuneDE Hybrid Window Manager (Hyprland Tiling + Floating + Monocle)
(function () {
  class WindowManager {
    constructor(stageEl) {
      this.stage = stageEl;
      this.windows = [];
      this.nextId = 1;
      this.zCounter = 30;
      this.focusedId = null;

      this.layoutMode = 'tiling'; // 'tiling' | 'floating' | 'monocle'
      this.activeWorkspace = 1;
      this.gapsIn = 8;
      this.gapsOut = 14;

      this.onStateChange = null;
      window.addEventListener('resize', () => this.relayout());
    }

    applySettings(s) {
      if (!s) return;
      this.layoutMode = s.layoutMode || 'tiling';
      this.activeWorkspace = s.activeWorkspace || 1;
      this.gapsIn = typeof s.gapsIn === 'number' ? s.gapsIn : 8;
      this.gapsOut = typeof s.gapsOut === 'number' ? s.gapsOut : 14;
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
          existing.minimized = false;
          this.focus(existing.id);
          this.relayout();
          if (opts.onReopen) opts.onReopen(existing);
          return existing;
        }
      }

      const id = 'win-' + this.nextId++;
      const stageRect = this.stage.getBoundingClientRect();
      const defW = Math.min(opts.width || 720, Math.max(360, stageRect.width - 40));
      const defH = Math.min(opts.height || 470, Math.max(260, stageRect.height - 40));
      const offset = ((this.windows.length % 5) * 26);

      const el = document.createElement('section');
      el.className = 'kitsune-window';
      el.dataset.winId = id;

      el.innerHTML = `
        <header class="win-titlebar">
          <div class="win-title-left">
            <span class="win-app-icon">${opts.icon || '🦊'}</span>
            <span class="win-title-text">${opts.title || 'Ventana'}</span>
            <span class="win-mode-tag">TILE</span>
          </div>
          <div class="win-controls">
            <button class="win-ctl-btn btn-master" title="Promover a ventana principal Master">⇄</button>
            <button class="win-ctl-btn btn-float" title="Alternar Flotante / Mosaico (Super+V)">⧉</button>
            <button class="win-ctl-btn btn-min" title="Minimizar al Dock">─</button>
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
        minimized: false,
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

      this.focus(id);
      this.relayout();
      return winObj;
    }

    bindWindowEvents(win) {
      const titlebar = win.el.querySelector('.win-titlebar');
      const btnClose = win.el.querySelector('.btn-close');
      const btnMin = win.el.querySelector('.btn-min');
      const btnMax = win.el.querySelector('.btn-max');
      const btnFloat = win.el.querySelector('.btn-float');
      const btnMaster = win.el.querySelector('.btn-master');
      const resizeHandle = win.el.querySelector('.win-resize-handle');

      win.el.addEventListener('pointerdown', () => {
        this.focus(win.id);
      });

      btnClose.addEventListener('click', (e) => {
        e.stopPropagation();
        this.close(win.id);
      });

      btnMin.addEventListener('click', (e) => {
        e.stopPropagation();
        win.minimized = true;
        this.relayout();
        this.notifyChange();
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

      // Pointer drag (supports floating drag + auto-pop-out if dragged far in tiling mode)
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

          if (!detached && Math.hypot(dx, dy) > 20) {
            detached = true;
            win.maximized = false;
            win.isFloatingOverride = true;
            const st = this.stage.getBoundingClientRect();
            const w = Math.min(700, st.width * 0.62);
            const h = Math.min(460, st.height * 0.68);
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
      if (this.layoutMode === 'tabbed') {
        this.renderSwayTabbar();
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
      return this.windows.filter((w) => w.workspace === this.activeWorkspace && !w.minimized);
    }

    relayout() {
      const st = this.stage.getBoundingClientRect();
      const stageW = Math.max(320, st.width);
      const stageH = Math.max(240, st.height);
      const go = this.gapsOut;
      const gi = this.gapsIn;

      // Hide windows on other workspaces or minimized
      this.windows.forEach((w) => {
        const onWs = w.workspace === this.activeWorkspace;
        if (!onWs) {
          w.el.style.display = 'none';
          return;
        }
        w.el.style.display = 'flex';
        w.el.classList.toggle('minimized', w.minimized);
      });

      const vis = this.visibleWindows();
      const tiled = [];
      const tabbed = [];
      const floating = [];
      const tabbar = document.getElementById('sway-tabbar');

      vis.forEach((w) => {
        const btnFloat = w.el.querySelector('.btn-float');
        if (w.maximized) {
          w.modeTagEl.textContent = 'MAX';
          this.applyRect(w, go, go, stageW - go * 2, stageH - go * 2);
          w.el.style.zIndex = '220';
          return;
        }
        if (this.layoutMode === 'monocle' && !w.isFloatingOverride) {
          w.modeTagEl.textContent = 'MONO';
          this.applyRect(w, go, go, stageW - go * 2, stageH - go * 2);
          return;
        }
        if (this.layoutMode === 'tabbed' && !w.isFloatingOverride) {
          w.modeTagEl.textContent = 'SWAY TAB';
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
        if (this.layoutMode === 'tabbed' && tabbed.length > 0) {
          tabbar.classList.remove('hidden');
          tabbar.style.left = `${go}px`;
          tabbar.style.top = `${go}px`;
          tabbar.style.width = `${Math.max(260, stageW - go * 2)}px`;
          this.tabbedWindows = tabbed;
          this.renderSwayTabbar();
          tabbed.forEach((w) => {
            this.applyRect(w, go, go + 34, Math.max(260, stageW - go * 2), Math.max(180, stageH - go * 2 - 34));
          });
        } else {
          tabbar.classList.add('hidden');
          this.tabbedWindows = [];
        }
      }

      // Layout Hyprland Tiled Windows
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

    renderSwayTabbar() {
      const tabbar = document.getElementById('sway-tabbar');
      if (!tabbar || !this.tabbedWindows || !this.tabbedWindows.length) return;
      tabbar.innerHTML = this.tabbedWindows.map((w) => `
        <button class="sway-tab ${w.id === this.focusedId ? 'active' : ''}" data-tab-win="${w.id}">
          ${w.icon} ${w.title}
        </button>
      `).join('');
      tabbar.querySelectorAll('.sway-tab').forEach((btn) => {
        btn.addEventListener('click', () => {
          this.focus(btn.dataset.tabWin);
        });
      });
    }

    applyRect(win, x, y, w, h) {
      win.el.style.left = `${Math.round(x)}px`;
      win.el.style.top = `${Math.round(y)}px`;
      win.el.style.width = `${Math.round(w)}px`;
      win.el.style.height = `${Math.round(h)}px`;
    }

    notifyChange() {
      if (typeof this.onStateChange === 'function') {
        const focused = this.windows.find((w) => w.id === this.focusedId && w.workspace === this.activeWorkspace && !w.minimized);
        const occupied = new Set(this.windows.map((w) => w.workspace));
        const runningApps = new Set(this.windows.map((w) => w.appId));
        this.onStateChange({ focused, occupied, runningApps });
      }
    }
  }

  window.KitsuneWM = WindowManager;
})();
