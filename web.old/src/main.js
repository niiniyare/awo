/**
 * AWO Framework Frontend — thin SDUI client.
 *
 * Responsibilities:
 *   - Application shell (sidebar, topbar, content area)
 *   - History API routing (/ui/{module}/{resource}/...)
 *   - Dynamic navigation from GET /api/v1/ui/nav
 *   - SDUI page rendering via GET /api/v1/ui/{module}/{resource}[/create|/{id}|/{id}/edit]
 *   - AMIS embedding (amisRequire loaded by sdk.js script tag before this module)
 *   - Theme (light / dark / system)
 *   - Auth (401 redirect)
 *   - API response normalisation (AMIS pagination format)
 *
 * NOT responsible for:
 *   - Entity definitions, field layout, permission gating — all backend SDUI.
 *   - Manually-authored page schemas — every page comes from the engine.
 */

import './style.css';

(function () {
  'use strict';

  // ── Visible debug overlay (Termux: no DevTools) ────────────────────────────
  // Minimisable panel at bottom. Tap "−" to collapse, "+" to expand.
  // Remove dbg() calls once stable.
  var _dbgEl = null;
  var _dbgBody = null;
  var _dbgMin = false;

  function dbg(msg, ok) {
    if (!_dbgEl) {
      _dbgEl = document.createElement('div');
      _dbgEl.style.cssText = [
        'position:fixed', 'bottom:0', 'left:0', 'right:0', 'z-index:99999',
        'background:#1e1e2d', 'color:#e4e4e7', 'font:12px/1.5 monospace',
        'border-top:2px solid #363648',
      ].join(';');

      var hdr = document.createElement('div');
      hdr.style.cssText = 'display:flex;justify-content:space-between;align-items:center;padding:4px 10px;cursor:pointer;background:#262637;';
      hdr.innerHTML = '<span style="font-weight:700;letter-spacing:.05em">AWO DEBUG</span><span id="awo-dbg-btn" style="font-size:16px;line-height:1;padding:0 4px">−</span>';
      hdr.addEventListener('click', function () {
        _dbgMin = !_dbgMin;
        _dbgBody.style.display = _dbgMin ? 'none' : 'block';
        document.getElementById('awo-dbg-btn').textContent = _dbgMin ? '+' : '−';
        document.getElementById('content').style.paddingBottom = _dbgMin ? '24px' : '160px';
      });

      _dbgBody = document.createElement('div');
      _dbgBody.style.cssText = 'max-height:35vh;overflow-y:auto;padding:6px 10px;white-space:pre-wrap;';

      _dbgEl.appendChild(hdr);
      _dbgEl.appendChild(_dbgBody);
      document.body.appendChild(_dbgEl);

      // Push content up so debug bar doesn't overlap.
      var content = document.getElementById('content');
      if (content) content.style.paddingBottom = '160px';
    }

    var line = document.createElement('div');
    line.textContent = (ok === true ? '✓ ' : ok === false ? '✗ ' : '· ') + msg;
    line.style.color = ok === true ? '#4ade80' : ok === false ? '#f87171' : '#9ca3af';
    _dbgBody.appendChild(line);
    _dbgBody.scrollTop = _dbgBody.scrollHeight;
  }

  // AMIS SDK is loaded as a UMD <script> tag before this module.
  // amisRequire() is a global set by sdk.js.
  var amisRequireOk = typeof amisRequire === 'function';
  dbg('amisRequire = ' + typeof amisRequire, amisRequireOk);

  var amis = null;
  if (amisRequireOk) {
    try {
      amis = amisRequire('amis/embed');
      dbg('amis/embed = ' + typeof amis + (amis ? ' embed=' + typeof amis.embed : ''), !!(amis && amis.embed));
    } catch (e) {
      dbg('amisRequire("amis/embed") threw: ' + e.message, false);
    }

    // Ensure English is the active locale.
    // locale-en.js (loaded via <script> before this module) registers the full
    // en-US bundle and calls setDefaultLocale. This is a safety net in case
    // that file was not served (missing public/locales/en.js in production).
    try {
      var amisCore = amisRequire('amis-core');
      if (amisCore && amisCore.setDefaultLocale) {
        amisCore.setDefaultLocale('en-US');
        dbg('locale: en-US set', true);
      }
    } catch (localeErr) {
      dbg('locale set failed: ' + localeErr.message, false);
    }
  }

  var currentInstance = null;

  // ── Theme ──────────────────────────────────────────────────────────────────

  var THEME_KEY = 'awo-theme';
  var themeOrder = ['system', 'light', 'dark'];
  var themeIcons  = { system: 'fa-circle-half-stroke', light: 'fa-sun', dark: 'fa-moon' };
  var themeLabels = { system: 'System', light: 'Light', dark: 'Dark' };
  var osDark = window.matchMedia('(prefers-color-scheme: dark)');

  function getTheme() {
    try { return localStorage.getItem(THEME_KEY) || 'system'; } catch (_) { return 'system'; }
  }

  function isDark(theme) {
    return theme === 'dark' || (theme === 'system' && osDark.matches);
  }

  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    var dark = isDark(theme);
    document.documentElement.classList.toggle('dark', dark);
    document.body.classList.toggle('dark', dark);
    var icon = document.getElementById('themeIcon');
    var label = document.getElementById('themeLabel');
    if (icon) icon.className = 'fa ' + (themeIcons[theme] || 'fa-circle-half-stroke');
    if (label) label.textContent = themeLabels[theme] || theme;
    try { localStorage.setItem(THEME_KEY, theme); } catch (_) {}
  }

  // Exposed for onclick in HTML.
  window.cycleTheme = function () {
    var cur = getTheme();
    var next = themeOrder[(themeOrder.indexOf(cur) + 1) % themeOrder.length];
    applyTheme(next);
  };

  osDark.addEventListener('change', function () {
    if (getTheme() === 'system') applyTheme('system');
  });

  applyTheme(getTheme());

  // ── Navigation ─────────────────────────────────────────────────────────────
  // navModules: NavModule[] from GET /api/v1/ui/nav
  //   [{module, label, entries:[{module, label, entity, listUrl, icon}]}]
  var navModules = [];

  // Module → FontAwesome icon mapping (extend as modules are added).
  var MODULE_ICONS = {
    finance:   'fa-calculator',
    inventory: 'fa-boxes-stacked',
    crm:       'fa-users',
    hr:        'fa-id-badge',
    platform:  'fa-cogs',
    demo:      'fa-flask',
  };

  function moduleIcon(mod) {
    return MODULE_ICONS[mod] || 'fa-folder';
  }

  function esc(s) {
    return String(s || '')
      .replace(/&/g, '&amp;')
      .replace(/"/g, '&quot;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  async function loadNav() {
    try {
      dbg('GET /api/v1/ui/nav …');
      var res = await fetch('/api/v1/ui/nav', { headers: { Accept: 'application/json' } });
      dbg('nav HTTP ' + res.status, res.ok);
      if (!res.ok) return;
      navModules = await res.json();
      dbg('nav modules=' + navModules.length + ' entries=' + navModules.reduce(function(n,m){return n+m.entries.length;},0), true);
      renderNav();
      updateActiveNav(window.location.pathname);
    } catch (e) {
      dbg('nav error: ' + e.message, false);
    }
  }

  function renderNav() {
    var menu = document.getElementById('sidebarMenu');
    if (!menu) return;
    if (!navModules.length) {
      menu.innerHTML = '<div style="padding:16px;font-size:13px;color:var(--text-muted)">No entities registered.</div>';
      return;
    }
    var html = '';
    navModules.forEach(function (mod) {
      html += '<div class="menu-group" data-group="' + esc(mod.module) + '">'
        + '<div class="menu-group-header" data-tooltip="' + esc(mod.label) + '"'
        + ' onclick="toggleGroup(\'' + esc(mod.module) + '\')">'
        + '<span class="group-icon"><i class="fa ' + esc(moduleIcon(mod.module)) + '"></i></span>'
        + '<span class="group-label">' + esc(mod.label) + '</span>'
        + '<i class="fa fa-chevron-down group-chevron"></i>'
        + '</div>'
        + '<div class="menu-group-items">';

      mod.entries.forEach(function (entry) {
        var icon = entry.icon ? entry.icon : 'fa-table';
        html += '<a class="menu-item"'
          + ' href="' + esc(entry.listUrl) + '"'
          + ' data-url="' + esc(entry.listUrl) + '"'
          + ' data-tooltip="' + esc(entry.label) + '"'
          + ' onclick="handleNavClick(event,\'' + esc(entry.listUrl) + '\')">'
          + '<span class="item-icon"><i class="fa ' + esc(icon) + '"></i></span>'
          + '<span class="item-label">' + esc(entry.label) + '</span>'
          + '</a>';
      });

      html += '</div></div>';
    });
    menu.innerHTML = html;
  }

  function updateActiveNav(pathname) {
    document.querySelectorAll('.menu-item[data-url]').forEach(function (el) {
      var url = el.getAttribute('data-url') || '';
      // Active when exact match or pathname is a sub-path (e.g. /ui/finance/invoices/create)
      var active = (pathname === url) || (url.length > 4 && pathname.startsWith(url + '/'));
      el.classList.toggle('active', active);
    });

    // Auto-expand the group containing the active entry; collapse others.
    document.querySelectorAll('.menu-group').forEach(function (group) {
      var hasActive = group.querySelector('.menu-item.active') !== null;
      group.classList.toggle('expanded', hasActive);
    });

    updateBreadcrumb(pathname);
  }

  function updateBreadcrumb(pathname) {
    var bc = document.getElementById('breadcrumb');
    if (!bc) return;
    var groupLabel = '', pageLabel = '';

    navModules.forEach(function (mod) {
      mod.entries.forEach(function (entry) {
        if (pathname === entry.listUrl || pathname.startsWith(entry.listUrl + '/')) {
          groupLabel = mod.label;
          pageLabel = entry.label;
        }
      });
    });

    // Fallback label from URL segments.
    if (!pageLabel) {
      var seg = pathname.replace(/^\/ui\/?/, '').split('/').filter(Boolean);
      pageLabel = seg.length ? seg[seg.length - 1].replace(/-/g, ' ') : 'Home';
      pageLabel = pageLabel.charAt(0).toUpperCase() + pageLabel.slice(1);
    }

    bc.textContent = '';
    if (groupLabel) {
      var g = document.createElement('span');
      g.textContent = groupLabel;
      var sep = document.createElement('i');
      sep.className = 'fa fa-chevron-right bc-separator';
      var p = document.createElement('span');
      p.className = 'bc-current';
      p.textContent = pageLabel;
      bc.appendChild(g);
      bc.appendChild(sep);
      bc.appendChild(p);
    } else {
      var span = document.createElement('span');
      span.className = 'bc-current';
      span.textContent = pageLabel;
      bc.appendChild(span);
    }
  }

  // Exposed for onclick in rendered nav HTML.
  window.toggleGroup = function (groupId) {
    var el = document.querySelector('[data-group="' + groupId + '"]');
    if (!el) return;
    var wasExpanded = el.classList.contains('expanded');
    // Collapse all groups.
    document.querySelectorAll('.menu-group').forEach(function (g) { g.classList.remove('expanded'); });
    // Re-expand only if it was closed.
    if (!wasExpanded) el.classList.add('expanded');
  };

  window.handleNavClick = function (evt, url) {
    if (evt) evt.preventDefault();
    history.pushState(null, '', url);
    navigate(url);
    closeMobileSidebar();
    collapseSidebarOnNav();
  };

  // ── Sidebar ────────────────────────────────────────────────────────────────

  window.toggleSidebar = function () {
    document.getElementById('sidebar').classList.toggle('collapsed');
  };

  window.openMobileSidebar = function () {
    document.getElementById('sidebar').classList.add('mobile-open');
    document.getElementById('sidebar-backdrop').classList.add('visible');
  };

  window.closeMobileSidebar = function () {
    document.getElementById('sidebar').classList.remove('mobile-open');
    document.getElementById('sidebar-backdrop').classList.remove('visible');
  };

  function collapseSidebarOnNav() {
    // On desktop: collapse sidebar after navigation (same UX as erp/web).
    if (window.innerWidth >= 768) {
      document.getElementById('sidebar').classList.add('collapsed');
    }
  }

  // ── Sidebar Search ─────────────────────────────────────────────────────────

  function filterNav(query) {
    var q = query.trim().toLowerCase();
    if (!q) {
      renderNav();
      updateActiveNav(window.location.pathname);
      return;
    }
    document.querySelectorAll('.menu-group').forEach(function (group) {
      var hasMatch = false;
      group.querySelectorAll('.menu-item').forEach(function (item) {
        var label = (item.getAttribute('data-tooltip') || '').toLowerCase();
        var match = label.indexOf(q) >= 0;
        item.style.display = match ? '' : 'none';
        if (match) hasMatch = true;
      });
      group.style.display = hasMatch ? '' : 'none';
      if (hasMatch) group.classList.add('expanded');
    });
  }

  // ── Routing ────────────────────────────────────────────────────────────────
  //
  // URL format:    /ui/{module}/{resource}[/create | /{id} | /{id}/edit]
  // SDUI endpoint: /api/v1/ui/{module}/{resource}[/create | /{id} | /{id}/edit]
  //
  // The mapping is intentionally 1:1 so frontend routing and API routing
  // share the same URL structure.

  function parseRoute(pathname) {
    // Strip /ui prefix.
    var path = pathname.replace(/^\/ui\/?/, '') || '';
    var parts = path.split('/').filter(Boolean);

    if (parts.length === 0) return { type: 'home' };
    if (parts.length === 1) return { type: 'module', module: parts[0] };

    var module = parts[0];
    var resource = parts[1];
    var base = '/api/v1/ui/' + module + '/' + resource;

    if (parts.length === 2) {
      return { type: 'list', module: module, resource: resource, apiUrl: base };
    }
    if (parts[2] === 'create') {
      return { type: 'create', module: module, resource: resource, apiUrl: base + '/create' };
    }
    if (parts.length >= 4 && parts[3] === 'edit') {
      return { type: 'edit', module: module, resource: resource, id: parts[2], apiUrl: base + '/' + parts[2] + '/edit' };
    }
    // Detail view.
    return { type: 'detail', module: module, resource: resource, id: parts[2], apiUrl: base + '/' + parts[2] };
  }

  async function navigate(pathname) {
    pathname = pathname || window.location.pathname;
    updateActiveNav(pathname);

    var route = parseRoute(pathname);
    var contentEl = document.getElementById('content');

    // Unmount previous AMIS instance before replacing content.
    if (currentInstance && typeof currentInstance.unmount === 'function') {
      try { currentInstance.unmount(); } catch (_) {}
      currentInstance = null;
    }
    contentEl.innerHTML = '<div class="loading"><i class="fa fa-spinner fa-spin"></i>&nbsp; Loading…</div>';

    // Home or module-only — redirect to the first registered entity's list view.
    if (route.type === 'home' || route.type === 'module') {
      // Wait until nav is loaded before redirecting (loadNav may still be in-flight).
      if (navModules.length > 0 && navModules[0].entries.length > 0) {
        var first = navModules[0].entries[0].listUrl;
        history.replaceState(null, '', first);
        return navigate(first);
      }
      contentEl.innerHTML = '<div class="loading">No entities registered. Check server logs.</div>';
      return;
    }

    if (!route.apiUrl) {
      showError(contentEl, 'Unknown route', pathname);
      return;
    }

    try {
      dbg('GET ' + route.apiUrl + ' …');
      var res = await fetch(route.apiUrl, { headers: { Accept: 'application/json' } });
      dbg('page HTTP ' + res.status, res.ok);

      if (res.status === 401 || res.status === 403) {
        window.location.href = '/ui/login?next=' + encodeURIComponent(pathname);
        return;
      }

      if (!res.ok) {
        var errBody = {};
        try { errBody = await res.json(); } catch (_) {}
        throw new Error(errBody.message || ('HTTP ' + res.status + ': ' + route.apiUrl));
      }

      var schema = await res.json();
      contentEl.innerHTML = '';

      if (!amis) {
        showError(contentEl, 'AMIS SDK not loaded',
          'sdk.js did not initialise. Check browser console for 404 errors.');
        return;
      }

      // Inject locale into the schema root so AMIS picks it up even when the
      // global default locale wasn't set (e.g. locale file 404'd in production).
      if (schema && typeof schema === 'object' && !schema.locale) {
        schema.locale = 'en-US';
      }
      currentInstance = amis.embed(contentEl, schema, { locale: 'en-US' }, amisEnv);
      dbg('amis.embed fired, instance=' + typeof currentInstance, !!currentInstance);
    } catch (err) {
      showError(contentEl, 'Page error', err.message);
    }
  }

  function showError(el, title, detail) {
    el.innerHTML = '<div class="error-page">'
      + '<h2><i class="fa fa-exclamation-triangle"></i> ' + esc(title) + '</h2>'
      + '<p>' + esc(detail) + '</p>'
      + '<a onclick="history.back()">← Go back</a>'
      + '</div>';
  }

  // ── AMIS Environment ───────────────────────────────────────────────────────
  //
  // fetcher: normalises backend response envelopes to the AMIS format.
  //   Backend list: { data:[], meta:{ total:N } }  or  { data:[], meta:{ pagination:{ total_records:N } } }
  //   AMIS expects: { status:0, data:{ items:[], count:N } }
  //
  // jumpTo: intercepts AMIS internal navigation so link-type actions use
  //   History API instead of full-page reloads.

  var amisEnv = {
    locale: 'en-US',

    fetcher: function (api) {
      var url = api.url;

      // Convert AMIS pagination params to backend offset/limit.
      try {
        var u = new URL(url, window.location.origin);
        var page = u.searchParams.get('page');
        var perPage = u.searchParams.get('perPage');
        if (page !== null) {
          var limit = perPage ? parseInt(perPage, 10) : 20;
          u.searchParams.delete('page');
          u.searchParams.set('offset', String((parseInt(page, 10) - 1) * limit));
        }
        if (perPage !== null) {
          u.searchParams.delete('perPage');
          u.searchParams.set('limit', perPage);
        }
        url = u.toString();
      } catch (_) {}

      var method = (api.method || 'GET').toUpperCase();
      var headers = Object.assign({ 'Content-Type': 'application/json' }, api.headers || {});

      // Double-submit CSRF cookie for state-changing requests.
      if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
        var csrf = document.cookie.split('; ').reduce(function (v, pair) {
          var kv = pair.split('=');
          return kv[0] === 'csrf_token' ? decodeURIComponent(kv[1]) : v;
        }, '');
        if (csrf) headers['X-Csrf-Token'] = csrf;
      }

      var opts = { method: method, headers: headers };
      if (method !== 'GET' && method !== 'HEAD' && api.data) {
        opts.body = JSON.stringify(api.data);
      }

      return fetch(url, opts)
        .then(function (res) {
          if (res.status === 204) return { status: 0, msg: '' };
          if (res.status === 401) {
            window.location.href = '/ui/login?next=' + encodeURIComponent(window.location.pathname);
            return Promise.reject(new Error('401'));
          }
          if (!res.ok) {
            return res.json().catch(function () { return {}; }).then(function (b) {
              return { status: res.status, msg: b.message || b.msg || res.statusText };
            });
          }
          return res.json();
        })
        .then(function (json) {
          if (!json) return json;

          // Already in AMIS format (has status, no success).
          if (json.status !== undefined && json.success === undefined) return json;

          // Backend success/error envelope.
          if (json.success !== undefined) {
            if (!json.success) return { status: 1, msg: json.message || 'Request failed' };
            // List with nested pagination.
            if (Array.isArray(json.data) && json.meta && json.meta.pagination) {
              return { status: 0, data: { items: json.data, count: json.meta.pagination.total_records } };
            }
            return { status: 0, data: json.data, msg: '' };
          }

          // Direct backend list: { data:[], meta:{ total:N } }
          if (Array.isArray(json.data) && json.meta && json.meta.total !== undefined) {
            return { status: 0, data: { items: json.data, count: json.meta.total } };
          }

          return json;
        })
        .catch(function (err) {
          console.error('[awo:fetcher]', err.message, url);
          return { status: 1, msg: 'Network error: ' + err.message };
        });
    },

    // Intercept AMIS link navigation (e.g. "view detail" buttons in list schemas).
    jumpTo: function (to) {
      var url = (to || '').replace(/^#/, '');
      if (!url) return;
      if (!url.startsWith('/')) url = '/ui/' + url;
      history.pushState(null, '', url);
      navigate(url);
    },

    isCancel: function () { return false; },

    notify: function (type, msg) {
      console.log('[amis:' + type + ']', msg);
    },

    alert: function (msg) {
      amisEnv.notify('error', msg);
    },

    copy: function (text) {
      if (navigator.clipboard) navigator.clipboard.writeText(text);
    },
  };

  // ── Init ───────────────────────────────────────────────────────────────────

  // Browser back/forward.
  window.addEventListener('popstate', function () {
    navigate(window.location.pathname);
  });

  // Sidebar search.
  var searchInput = document.getElementById('sidebarSearch');
  if (searchInput) {
    searchInput.addEventListener('input', function () { filterNav(this.value); });
  }

  // Global click delegation for <a href="/ui/..."> links inside AMIS-rendered content.
  document.addEventListener('click', function (e) {
    var a = e.target.closest('a[href]');
    if (!a) return;
    var href = a.getAttribute('href');
    if (!href) return;
    // Only intercept internal /ui/* links; let external links and API links pass through.
    if (href.startsWith('/ui/') || href === '/ui') {
      e.preventDefault();
      history.pushState(null, '', href);
      navigate(href);
    }
  });

  // Bootstrap: load navigation, then navigate to current URL.
  loadNav().then(function () {
    var path = window.location.pathname;
    // Redirect root or bare /ui to the first registered entity.
    if (!path.startsWith('/ui') || path === '/ui' || path === '/ui/') {
      if (navModules.length > 0 && navModules[0].entries.length > 0) {
        var first = navModules[0].entries[0].listUrl;
        history.replaceState(null, '', first);
        navigate(first);
      } else {
        navigate('/ui');
      }
    } else {
      navigate(path);
    }
  });
})();
