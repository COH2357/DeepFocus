/* DeepFocus desktop UI — vanilla JS, no build step. */
(function () {
  "use strict";

  const { translate } = window.FocusdI18N;
  const TOKEN = (document.querySelector('meta[name="focusd-token"]') || {}).content || "";
  const VIEWS = ["dashboard", "stats", "focus", "limits", "settings"];
  const MONTH_DAY = { month: "short", day: "numeric" };

  const state = {
    lang: pickLang(),
    theme: pickTheme(),
    view: "dashboard",
    settings: null,
    overview: null,
    limitsData: null,
    stats: null,
    pomodoro: null,
    platform: {},
    version: "",
    statsRange: "7",
    customFrom: isoDate(offsetDate(-6)),
    customTo: isoDate(new Date()),
    selectedPreset: null,
    loading: false,
    fatal: null,
    completionAnnounced: false,
    lastBootstrap: 0
  };

  const $ = (sel, root) => (root || document).querySelector(sel);
  const $$ = (sel, root) => Array.from((root || document).querySelectorAll(sel));
  const view = $("#view");
  const modalRoot = $("#modalRoot");
  const tooltip = $("#tooltip");

  /* ---------- utilities ---------- */

  function t(key, vars) {
    return translate(state.lang, key, vars);
  }

  function pickLang() {
    try {
      const saved = localStorage.getItem("focusd.lang");
      if (saved === "zh" || saved === "en") return saved;
    } catch (_) {}
    const langs = navigator.languages && navigator.languages.length ? navigator.languages : [navigator.language || "en"];
    for (const l of langs) {
      if (String(l).toLowerCase().indexOf("zh") === 0) return "zh";
    }
    return "en";
  }

  function pickTheme() {
    try {
      const saved = localStorage.getItem("focusd.theme");
      if (saved === "light" || saved === "dark") return saved;
    } catch (_) {}
    return window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  function isoDate(d) {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, "0");
    const day = String(d.getDate()).padStart(2, "0");
    return y + "-" + m + "-" + day;
  }

  function offsetDate(days) {
    const d = new Date();
    d.setDate(d.getDate() + days);
    return d;
  }

  function parseISO(value) {
    const parts = String(value).split("-").map(Number);
    return new Date(parts[0], (parts[1] || 1) - 1, parts[2] || 1);
  }

  function locale() {
    return state.lang === "zh" ? "zh-CN" : "en-US";
  }

  function fmtDateLong(value) {
    return new Intl.DateTimeFormat(locale(), { weekday: "long", month: "long", day: "numeric" }).format(parseISO(value));
  }

  function fmtDateShort(value) {
    return new Intl.DateTimeFormat(locale(), MONTH_DAY).format(parseISO(value));
  }

  function fmtTime(ts) {
    if (!ts) return "—";
    return new Intl.DateTimeFormat(locale(), { hour: "2-digit", minute: "2-digit" }).format(new Date(ts * 1000));
  }

  function fmtNumber(n) {
    return new Intl.NumberFormat(locale()).format(Math.round(n || 0));
  }

  function fmtPercent(n) {
    return Math.round(n || 0) + "%";
  }

  function fmtDuration(seconds) {
    const s = Math.max(0, Math.round(seconds || 0));
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    if (state.lang === "zh") {
      if (h > 0) return h + " 小时" + (m ? " " + m + " 分" : "");
      if (m > 0) return m + " 分钟";
      return s + " 秒";
    }
    if (h > 0) return h + "h" + (m ? " " + m + "m" : "");
    if (m > 0) return m + "m";
    return s + "s";
  }

  function fmtCompact(seconds) {
    const s = Math.max(0, Math.round(seconds || 0));
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    if (h > 0) return h + (state.lang === "zh" ? " 小时 " : "h ") + String(m).padStart(2, "0") + (state.lang === "zh" ? " 分" : "m");
    return m + (state.lang === "zh" ? " 分钟" : "m");
  }

  function fmtClock(seconds) {
    const s = Math.max(0, Math.round(seconds || 0));
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const sec = s % 60;
    if (h > 0) return h + ":" + String(m).padStart(2, "0") + ":" + String(sec).padStart(2, "0");
    return String(m).padStart(2, "0") + ":" + String(sec).padStart(2, "0");
  }

  function bigTimeHTML(seconds) {
    const s = Math.max(0, Math.round(seconds || 0));
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    if (h > 0) {
      return '<span class="big-value">' + h + '</span><span class="big-unit">' +
        (state.lang === "zh" ? "小时 " : "h ") + String(m).padStart(2, "0") +
        (state.lang === "zh" ? " 分" : "m") + "</span>";
    }
    return '<span class="big-value">' + m + '</span><span class="big-unit">' +
      (state.lang === "zh" ? "分钟" : "min") + "</span>";
  }

  function esc(value) {
    return String(value == null ? "" : value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function initial(name) {
    const text = String(name || "?").trim();
    return esc(text.charAt(0).toUpperCase() || "?");
  }

  function appIcon(app) {
    return '<span class="app-icon">' + initial(app.name || app.exe) + "</span>";
  }

  function deltaHTML(delta, suffix) {
    if (delta === null || delta === undefined || !isFinite(delta)) {
      return '<span class="muted" style="font-size:11.5px">' + t("dash.noComparison") + "</span>";
    }
    const dir = delta > 0 ? "up" : delta < 0 ? "down" : "";
    const arrow = delta > 0
      ? '<svg viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M6 10V2M2.8 5.2 6 2l3.2 3.2" stroke-linecap="round" stroke-linejoin="round"/></svg>'
      : delta < 0
        ? '<svg viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M6 2v8M2.8 6.8 6 10l3.2-3.2" stroke-linecap="round" stroke-linejoin="round"/></svg>'
        : "";
    return '<span class="delta ' + dir + '">' + arrow + Math.abs(Math.round(delta)) + "%" + (suffix ? " " + esc(suffix) : "") + "</span>";
  }

  const ICONS = {
    check: '<path d="M5 12.5 9.5 17 19 7" stroke-linecap="round" stroke-linejoin="round"/>',
    alert: '<path d="M12 4v9" stroke-linecap="round"/><circle cx="12" cy="17.5" r="1.1" fill="currentColor" stroke="none"/>',
    x: '<path d="m6 6 12 12M18 6 6 18" stroke-linecap="round"/>',
    plus: '<path d="M12 5v14M5 12h14" stroke-linecap="round"/>',
    trash: '<path d="M4 7h16M9 7V4h6v3M6.5 7l1 13h9l1-13" stroke-linecap="round" stroke-linejoin="round"/>',
    download: '<path d="M12 3v12m0 0 4-4m-4 4-4-4M4 20h16" stroke-linecap="round" stroke-linejoin="round"/>',
    folder: '<path d="M3 7.5A1.5 1.5 0 0 1 4.5 6h5l2 2.5h8A1.5 1.5 0 0 1 21 10v8a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18V7.5Z" stroke-linejoin="round"/>',
    play: '<path d="M8 5.5v13l10-6.5-10-6.5Z" fill="currentColor" stroke="none"/>',
    pause: '<path d="M9 5v14M15 5v14" stroke-linecap="round"/>',
    refresh: '<path d="M20 12a8 8 0 1 1-2.3-5.6M20 4v5h-5" stroke-linecap="round" stroke-linejoin="round"/>',
    external: '<path d="M14 4h6v6M20 4l-8 8M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" stroke-linecap="round" stroke-linejoin="round"/>',
    clock: '<circle cx="12" cy="12" r="8.5"/><path d="M12 7v5.2l3.4 2" stroke-linecap="round"/>',
    shield: '<path d="M12 3 4.5 6v6c0 4.4 3.1 7.6 7.5 9 4.4-1.4 7.5-4.6 7.5-9V6L12 3Z" stroke-linejoin="round"/>',
    spark: '<path d="M12 3v4M12 17v4M3 12h4M17 12h4M5.6 5.6l2.8 2.8M15.6 15.6l2.8 2.8M18.4 5.6l-2.8 2.8M8.4 15.6l-2.8 2.8" stroke-linecap="round"/>',
    globe: '<circle cx="12" cy="12" r="8.5"/><path d="M3.5 12h17M12 3.5c2.4 2.3 3.6 5.2 3.6 8.5s-1.2 6.2-3.6 8.5c-2.4-2.3-3.6-5.2-3.6-8.5S9.6 5.8 12 3.5Z"/>',
    layers: '<path d="m12 3 9 5-9 5-9-5 9-5ZM3 12l9 5 9-5M3 16l9 5 9-5" stroke-linejoin="round"/>',
    timer: '<circle cx="12" cy="13" r="8"/><path d="M12 9v4l2.5 2M9 2h6" stroke-linecap="round"/>',
    lock: '<rect x="4.5" y="10" width="15" height="10" rx="2"/><path d="M8 10V7.5a4 4 0 0 1 8 0V10"/>',
    info: '<circle cx="12" cy="12" r="8.5"/><path d="M12 11v5" stroke-linecap="round"/><circle cx="12" cy="8" r="1" fill="currentColor" stroke="none"/>',
    chart: '<path d="M4 19V5m0 14h16M8 16v-4m4 4V8m4 8v-6" stroke-linecap="round"/>',
    message: '<path d="M4 5.5h16v11H9l-5 4v-15Z" stroke-linejoin="round"/>'
  };

  function icon(name, cls) {
    return '<svg class="' + (cls || "") + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">' + (ICONS[name] || "") + "</svg>";
  }

  function switchHTML(checked, setting, extra) {
    return '<label class="switch"><input type="checkbox" data-setting="' + setting + '" ' +
      (checked ? "checked " : "") + (extra || "") + '><span class="track"></span></label>';
  }

  /* ---------- API ---------- */

  async function api(path, options) {
    const opts = Object.assign({}, options || {});
    opts.headers = Object.assign(
      { "X-Focusd-Token": TOKEN },
      opts.body ? { "Content-Type": "application/json" } : {},
      opts.headers || {}
    );
    const res = await fetch(path, opts);
    let data = null;
    try {
      data = await res.json();
    } catch (_) {}
    if (!res.ok) {
      const message = data && data.error ? data.error : res.status + " " + res.statusText;
      throw new Error(message);
    }
    return data;
  }

  function act(action, payload) {
    return api("/api/actions", {
      method: "POST",
      body: JSON.stringify(Object.assign({ action: action }, payload || {}))
    });
  }

  /* ---------- toasts / modal / tooltip ---------- */

  function toast(message, kind) {
    const root = $("#toasts");
    while (root.children.length >= 3) root.removeChild(root.firstElementChild);
    const node = document.createElement("div");
    node.className = "toast" + (kind === "err" ? " err" : "");
    node.innerHTML = icon(kind === "err" ? "alert" : "check") + "<span>" + esc(message) + "</span>";
    root.appendChild(node);
    setTimeout(function () {
      node.classList.add("out");
      setTimeout(function () {
        if (node.parentNode) node.parentNode.removeChild(node);
      }, 240);
    }, kind === "err" ? 5200 : 3200);
  }

  function toastError(err) {
    const message = err && err.message ? err.message : String(err || t("common.actionFailed"));
    toast(t("toast.error", { msg: message }), "err");
  }

  function closeModal() {
    modalRoot.innerHTML = "";
  }

  function openModal(html, options) {
    const opts = options || {};
    closeModal();
    modalRoot.innerHTML =
      '<div class="modal-backdrop" data-modal-backdrop><div class="modal' + (opts.wide ? " wide" : "") +
      '"><div class="modal-shell">' + html + "</div></div></div>";
    const backdrop = $("[data-modal-backdrop]");
    backdrop.addEventListener("click", function (event) {
      if (event.target === backdrop) closeModal();
    });
    if (opts.onMount) opts.onMount($(".modal", backdrop));
    const focusable = $("[autofocus]", backdrop) || $("input, button", backdrop);
    if (focusable) {
      setTimeout(function () {
        try { focusable.focus(); } catch (_) {}
      }, 30);
    }
  }

  function confirmModal(title, body, confirmText, danger, onConfirm) {
    openModal(
      '<div class="modal-head"><h3>' + esc(title) + "</h3><p>" + body + "</p></div>" +
      '<div class="modal-foot"><button class="btn" data-close> ' + t("common.cancel") + "</button>" +
      '<button class="btn ' + (danger ? "btn-danger" : "btn-primary") + '" id="confirmAction">' + esc(confirmText) + "</button></div>",
      {
        onMount: function (modal) {
          $("[data-close]", modal).addEventListener("click", closeModal);
          $("#confirmAction", modal).addEventListener("click", async function () {
            const button = this;
            button.disabled = true;
            try {
              await onConfirm();
              closeModal();
            } catch (err) {
              toastError(err);
              button.disabled = false;
            }
          });
        }
      }
    );
  }

  /* ---------- boot / routing / chrome ---------- */

  function boot() {
    applyTheme();
    applyLanguage();
    bindStatic();
    state.view = hashView();
    updateNav();
    render();
    loadEverything(true).then(function () {
      state.loading = false;
      render();
      tickPomodoro();
    }).catch(function (err) {
      state.loading = false;
      state.fatal = err;
      render();
    });
  }

  function hashView() {
    const raw = (location.hash || "").replace(/^#/, "");
    return VIEWS.indexOf(raw) >= 0 ? raw : "dashboard";
  }

  async function loadEverything(first) {
    const b = await api("/api/bootstrap");
    state.settings = b.settings;
    try {
      if (!localStorage.getItem("focusd.lang") && b.settings && (b.settings.language === "zh" || b.settings.language === "en")) {
        state.lang = b.settings.language;
      }
    } catch (_) {}
    state.overview = b.overview;
    state.pomodoro = b.pomodoro;
    state.limitsData = { limits: b.limits || [], suggestions: b.suggestions || [] };
    state.platform = b.platform || {};
    state.version = b.version || "";
    state.fatal = null;
    state.lastBootstrap = Date.now();
    if (state.selectedPreset == null && state.settings) {
      state.selectedPreset = state.settings.pomodoroMinutes;
    }
    if (first && state.view === "stats") await loadStats();
  }

  async function loadStats() {
    if (!state.settings) return;
    const range = computeRange(state.statsRange);
    state.stats = await api("/api/stats?from=" + encodeURIComponent(range.from) + "&to=" + encodeURIComponent(range.to));
  }

  async function loadLimits() {
    state.limitsData = await api("/api/limits");
  }

  async function loadSettings() {
    state.settings = await api("/api/settings");
  }

  async function loadPomodoro() {
    state.pomodoro = await api("/api/pomodoro");
  }

  function computeRange(kind) {
    const today = new Date();
    if (kind === "today") return { from: isoDate(today), to: isoDate(today) };
    if (kind === "30") return { from: isoDate(offsetDate(-29)), to: isoDate(today) };
    if (kind === "custom") return { from: state.customFrom, to: state.customTo };
    return { from: isoDate(offsetDate(-6)), to: isoDate(today) };
  }

  function applyTheme() {
    document.documentElement.dataset.theme = state.theme;
  }

  function applyLanguage() {
    document.documentElement.lang = state.lang === "zh" ? "zh-CN" : "en";
    $$("[data-i18n]").forEach(function (node) {
      const key = node.getAttribute("data-i18n");
      node.textContent = t(key);
    });
    updateChrome();
  }

  function updateNav() {
    $$(".nav-item").forEach(function (node) {
      node.classList.toggle("active", node.dataset.view === state.view);
    });
  }

  function updateChrome() {
    const settings = state.settings;
    const paused = settings && settings.paused;
    const running = settings && settings.daemonRunning;
    const pill = $("#trackerPill");
    const label = $("#trackerLabel");
    if (pill) {
      pill.classList.remove("on", "paused", "off");
      if (!settings) {
        label.textContent = t("chrome.connecting");
      } else if (!running) {
        pill.classList.add("off");
        label.textContent = t("chrome.trackerOff");
      } else if (paused) {
        pill.classList.add("paused");
        label.textContent = t("chrome.trackerPaused");
      } else {
        pill.classList.add("on");
        label.textContent = t("chrome.trackerOn");
      }
    }

    const pauseBtn = $("#pauseBtn");
    if (pauseBtn && settings) {
      pauseBtn.dataset.action = paused ? "resume" : "pause";
      pauseBtn.title = paused ? t("chrome.resume") : t("chrome.pause");
      pauseBtn.setAttribute("aria-label", pauseBtn.title);
      $("#pauseLabel").textContent = paused ? t("chrome.resume") : t("chrome.pause");
      $(".icon-pause", pauseBtn).classList.toggle("hidden", paused);
      $(".icon-play", pauseBtn).classList.toggle("hidden", !paused);
      pauseBtn.classList.toggle("on", paused);
    }

    const chip = $("#pomodoroChip");
    if (chip) {
      const active = state.pomodoro && state.pomodoro.active;
      chip.classList.toggle("hidden", !active);
      if (active) {
        $("#pomodoroChipTime").textContent = fmtClock(state.pomodoro.remainingSeconds);
        const labelNode = $(".pomo-label", chip);
        labelNode.textContent = state.pomodoro.remainingSeconds > 0 ? t("focus.running") : t("focus.done");
      }
    }

    const dateLabel = $("#crumbDate");
    if (dateLabel) dateLabel.textContent = fmtDateLong(isoDate(new Date()));
    $("#crumbSub").textContent = paused ? t("chrome.paused") : t("chrome.localOnly");
    $("#versionLabel").textContent = state.version ? "v" + state.version : "v…";
    $("#langLabel").textContent = state.lang === "zh" ? "EN" : "中";
    $("#langBtn").title = state.lang === "zh" ? "Switch to English" : "切换到中文";
    $("#langBtn").setAttribute("aria-label", $("#langBtn").title);
    $("#themeBtn").setAttribute("aria-label", t("shortcuts.theme"));
    $("#refreshBtn").setAttribute("aria-label", t("common.refresh"));
    $("#shortcutHint").setAttribute("aria-label", t("shortcuts.title"));
    document.title = "DeepFocus — " + t("nav." + state.view);
  }

  function bindStatic() {
    $("#themeBtn").addEventListener("click", toggleTheme);
    $("#langBtn").addEventListener("click", toggleLang);
    $("#refreshBtn").addEventListener("click", refreshAll);
    $("#shortcutHint").addEventListener("click", openShortcuts);
    $("#pomodoroChip").addEventListener("click", function () { go("focus"); });
    $("#pomodoroChip").addEventListener("keydown", function (event) {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        go("focus");
      }
    });

    document.addEventListener("click", function (event) {
      const nav = event.target.closest("[data-view]");
      if (nav && nav.classList.contains("nav-item")) {
        event.preventDefault();
        go(nav.dataset.view);
        return;
      }
      const actionNode = event.target.closest("[data-action]");
      if (actionNode) {
        event.preventDefault();
        handleAction(actionNode.dataset.action, actionNode).catch(toastError);
        return;
      }
      if (event.target.closest("[data-close]")) {
        closeModal();
      }
    });

    view.addEventListener("change", onViewChange);
    view.addEventListener("submit", onViewSubmit);
    view.addEventListener("input", onViewInput);
    modalRoot.addEventListener("submit", onModalSubmit);

    document.addEventListener("keydown", onKeydown);
    document.addEventListener("mousemove", onPointerTip);

    window.addEventListener("hashchange", function () {
      const next = hashView();
      if (next !== state.view) go(next);
    });
  }

  async function go(name) {
    if (VIEWS.indexOf(name) < 0) name = "dashboard";
    if (name === state.view && !state.fatal) {
      render();
      return;
    }
    state.view = name;
    if (location.hash !== "#" + name) history.replaceState(null, "", "#" + name);
    updateNav();
    updateChrome();
    try {
      if (name === "stats" && !state.stats) {
        state.loading = true;
        render();
        await loadStats();
        state.loading = false;
      } else if (name === "limits") {
        await loadLimits();
      } else if (name === "settings") {
        await loadSettings();
      } else if (name === "focus") {
        await loadPomodoro();
      } else if (name === "stats") {
        await loadStats();
      }
    } catch (err) {
      toastError(err);
      state.loading = false;
    }
    render();
    view.scrollTop = 0;
  }

  async function refreshAll() {
    const button = $("#refreshBtn");
    if (button) button.classList.add("spin");
    try {
      await loadEverything(false);
      if (state.view === "stats") await loadStats();
      if (state.view === "limits") await loadLimits();
      if (state.view === "settings") await loadSettings();
      if (state.view === "focus") await loadPomodoro();
      render();
    } catch (err) {
      toastError(err);
    } finally {
      setTimeout(function () {
        if (button) button.classList.remove("spin");
      }, 450);
    }
  }

  // A setting that changes which rows are counted (site grouping, excluded
  // apps) must refresh the visible numbers immediately, not on the next poll.
  async function refreshAfterTrackingChange() {
    await loadEverything(false);
    if (state.view === "stats") await loadStats();
    if (state.view === "limits") await loadLimits();
    render();
  }

  function toggleTheme() {
    state.theme = state.theme === "dark" ? "light" : "dark";
    try { localStorage.setItem("focusd.theme", state.theme); } catch (_) {}
    applyTheme();
    act("theme", { dark: state.theme === "dark" }).catch(function () {});
    updateChrome();
    render();
  }

  function toggleLang() {
    state.lang = state.lang === "zh" ? "en" : "zh";
    try { localStorage.setItem("focusd.lang", state.lang); } catch (_) {}
    act("language", { language: state.lang }).catch(function () {});
    applyLanguage();
    if (state.settings && state.selectedPreset == null) state.selectedPreset = state.settings.pomodoroMinutes;
    render();
    toast(t("toast.language"));
  }

  /* ---------- rendering ---------- */

  function render() {
    updateNav();
    updateChrome();
    if (state.fatal) {
      view.innerHTML = renderFatal(state.fatal);
      return;
    }
    if (state.loading) {
      view.innerHTML = skeletonPage();
      return;
    }
    switch (state.view) {
      case "stats":
        view.innerHTML = renderStats();
        break;
      case "focus":
        view.innerHTML = renderFocus();
        break;
      case "limits":
        view.innerHTML = renderLimits();
        break;
      case "settings":
        view.innerHTML = renderSettings();
        break;
      default:
        view.innerHTML = renderDashboard();
    }
  }

  function skeletonPage() {
    return '<div class="page"><div class="sk-page">' +
      '<div class="skeleton sk-line" style="width:180px"></div>' +
      '<div class="skeleton sk-line" style="width:320px;height:30px"></div>' +
      '<div class="grid grid-dash"><div class="skeleton sk-card" style="height:220px"></div><div class="skeleton sk-card" style="height:220px"></div></div>' +
      '<div class="grid grid-4"><div class="skeleton sk-card"></div><div class="skeleton sk-card"></div><div class="skeleton sk-card"></div><div class="skeleton sk-card"></div></div>' +
      "</div></div>";
  }

  function renderFatal(err) {
    return '<div class="page"><div class="card" style="max-width:560px;margin:8vh auto;text-align:center;padding:40px">' +
      '<div class="kpi-icon brick" style="margin:0 auto 14px;width:42px;height:42px">' + icon("alert") + "</div>" +
      "<h1 style=\"font-size:24px\">" + esc(t("common.actionFailed")) + "</h1>" +
      '<p class="muted" style="font-size:12.5px;word-break:break-word">' + esc(err && err.message ? err.message : String(err)) + "</p>" +
      '<button class="btn btn-primary" data-action="retry" style="margin-top:16px">' + esc(t("common.refresh")) + "</button>" +
      "</div></div>";
  }

  function pageHead(eyebrow, title, subtitle, actions) {
    return '<div class="page-head"><div><div class="eyebrow">' + esc(eyebrow) + "</div><h1>" + title + "</h1>" +
      (subtitle ? '<p class="page-sub">' + esc(subtitle) + "</p>" : "") + "</div>" +
      (actions ? '<div class="head-actions">' + actions + "</div>" : "") + "</div>";
  }

  function emptyState(iconName, title, sub) {
    return '<div class="empty">' + icon(iconName) + "<b>" + esc(title) + "</b>" +
      (sub ? "<span>" + esc(sub) + "</span>" : "") + "</div>";
  }

  /* ---------- dashboard ---------- */

  function renderDashboard() {
    const o = state.overview;
    if (!o) return skeletonPage();
    const paused = state.settings && state.settings.paused;
    const delta = o.yesterday > 0 ? ((o.total - o.yesterday) / o.yesterday) * 100 : null;
    const apps = o.apps || [];
    const maxApp = Math.max(1, apps.length ? apps[0].seconds : 1);
    const days = o.days || [];
    const weekMax = Math.max(1, ...days.map(function (d) { return d.seconds; }));
    const limitsHit = o.limitsHit || 0;

    const banner = paused
      ? '<div class="banner">' + icon("pause") + "<span>" + esc(t("dash.pausedBanner")) + "</span>" +
        '<button class="btn btn-sm" data-action="resume">' + esc(t("dash.resumeNow")) + "</button></div>"
      : "";

    const hero =
      '<div class="card hero">' +
        '<div class="hero-top"><div>' +
          '<div class="hero-label">' + esc(t("dash.tracked")) + "</div>" +
          '<div class="hero-time">' + bigTimeHTML(o.total) + "</div>" +
          '<div class="hero-foot">' + deltaHTML(delta) + "<span>" + esc(t("dash.vsYesterday")) + "</span>" +
            '<span class="faint">·</span><span>' + esc(new Intl.DateTimeFormat(locale(), { weekday: "long" }).format(new Date())) + "</span>" +
          "</div>" +
        "</div>" +
        '<div class="live-dot">' + esc(t("dash.live")) + "</div></div>" +
        sparkline(days) +
      "</div>";

    const weekBars = days.map(function (d) {
      const pct = d.seconds > 0 ? Math.max(6, (d.seconds / weekMax) * 100) : 3;
      const dayName = new Intl.DateTimeFormat(locale(), { weekday: "narrow" }).format(parseISO(d.date));
      return '<div class="wb' + (d.isToday ? " today" : "") + '" style="height:' + pct + '%" data-tip="' +
        esc(dayName + " · " + fmtDuration(d.seconds)) + '"></div>';
    }).join("");

    const side =
      '<div class="card hero-side">' +
        '<div class="between"><div><div class="card-title">' + esc(t("dash.weekly")) + "</div>" +
          '<div class="card-sub">' + esc(t("dash.weekTotal")) + "</div></div>" +
          '<div class="mono" style="font-size:15px">' + esc(fmtDuration(o.weekTotal)) + "</div></div>" +
        '<div class="week-bars">' + weekBars + "</div>" +
        '<div class="mini"><span class="mini-label">' + esc(t("dash.dailyAvg")) + '</span><span class="mini-value">' + esc(fmtDuration(o.dailyAvg)) + "</span></div>" +
        '<div class="mini"><span class="mini-label">' + esc(t("dash.peakDay")) + '</span><span class="mini-value">' +
          (o.peakDay ? esc(fmtDateShort(o.peakDay.date)) : "—") + "</span></div>" +
      "</div>";

    const kpis =
      '<div class="grid grid-4">' +
        kpi("layers", "", t("dash.appCount"), fmtNumber(o.appCount), t("stats.appsCount", { n: o.appCount })) +
        kpi("spark", "green", t("dash.launches"), fmtNumber(o.launches), t("dash.opens")) +
        kpi("clock", "amber", t("dash.weekly"), fmtDuration(o.weekTotal), t("dash.weekTotal")) +
        kpi("shield", limitsHit ? "brick" : "", t("dash.limitsHit"), fmtNumber(limitsHit), limitsHit ? t("limits.noteShort") : t("dash.allClear")) +
      "</div>";

    const active = o.active
      ? '<div class="card now-card"><div class="now-icon">' + initial(o.active.app) + "</div>" +
        '<div class="now-body"><div class="now-name">' + esc(o.active.app) + "</div>" +
        '<div class="now-title">' + esc(o.active.title || t("dash.activeNow")) + "</div></div>" +
        '<div style="text-align:right"><div class="now-time">' + esc(fmtDuration(o.active.seconds)) + "</div>" +
        '<div class="live-dot">' + esc(t("dash.activeNow")) + "</div></div></div>"
      : '<div class="card now-card"><div class="now-icon" style="background:color-mix(in srgb,var(--ink) 6%,transparent);color:var(--faint)">' + icon("clock") + "</div>" +
        '<div class="now-body"><div class="now-name">' + esc(t("dash.idleNow")) + "</div>" +
        '<div class="now-title">' + esc(t("dash.idleHint")) + "</div></div></div>";

    const appRows = apps.slice(0, 7).map(function (app) {
      const pct = Math.max(3, (app.seconds / maxApp) * 100);
      const share = o.total > 0 ? (app.seconds / o.total) * 100 : 0;
      const limitText = app.limitMinutes ? t("limits.limit") + " " + app.limitMinutes + "m" : "";
      return '<div class="app-row clickable" data-action="app-limit" data-key="' + esc(app.limitKey || app.appId || app.exe) + '" data-name="' + esc(app.name) + '">' +
        appIcon(app) +
        '<div style="min-width:0"><div class="app-name">' + esc(app.name) + "</div>" +
          '<div class="app-meta">' + fmtPercent(share) + " " + esc(t("dash.share")) +
          (app.opens ? " · " + fmtNumber(app.opens) + " " + esc(t("dash.opens")) : "") +
          (limitText ? ' · <span class="limit-note">' + esc(limitText) + "</span>" : "") + "</div>" +
          '<div class="app-bar"><i style="width:' + pct + '%"></i></div></div>' +
        '<div class="app-time">' + esc(fmtCompact(app.seconds)) + "</div></div>";
    }).join("");

    const appsCard =
      '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("layers") + esc(t("dash.topApps")) + "</div>" +
        '<div class="card-sub">' + esc(t("dash.tracked")) + "</div></div>" +
        '<button class="card-link" data-action="go-stats">' + esc(t("dash.seeStats")) + "</button></div>" +
        (apps.length ? '<div class="app-list">' + appRows + "</div>" : emptyState("layers", t("dash.noApps"), t("dash.noAppsSub"))) +
      "</div>";

    const sitesCard = renderSitesCard(o.sites || []);

    const hourly = renderHourly(o.hourly || [], o.date);
    const hourlyCard =
      '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("chart") + esc(t("dash.hourly")) + "</div>" +
        '<div class="card-sub">' + esc(fmtDateLong(o.date)) + "</div></div>" +
        '<span class="upper faint">' + esc(t("dash.live")) + "</span></div>" + hourly + "</div>";

    const recent = o.recent || [];
    const recentRows = recent.map(function (s) {
      return '<div class="session-row' + (s.active ? " active" : "") + '">' +
        '<div class="session-time">' + esc(fmtTime(s.start)) + "</div>" +
        '<div style="min-width:0"><div class="session-name">' + esc(s.app) + "</div>" +
        '<div class="session-title">' + esc(s.title || s.exe || "") + "</div></div>" +
        '<div class="session-dur">' + (s.active ? '<span class="live-dot">live</span>' : esc(fmtCompact(s.seconds))) + "</div></div>";
    }).join("");
    const recentCard =
      '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("clock") + esc(t("dash.recent")) + "</div></div>" +
        '<button class="card-link" data-action="go-stats">' + esc(t("dash.seeStats")) + "</button></div>" +
        (recent.length ? '<div class="session-list">' + recentRows + "</div>" : emptyState("clock", t("dash.noSessions"), "")) + "</div>";

    return '<div class="page">' +
      pageHead(fmtDateLong(o.date), t("dash.title"), t("dash.subtitle"),
        '<button class="btn btn-primary" data-action="go-focus">' + icon("timer") + esc(t("dash.focusCta")) + "</button>") +
      banner +
      '<div class="grid grid-dash" style="margin-bottom:16px">' + hero + side + "</div>" +
      kpis +
      '<div style="margin:16px 0">' + active + "</div>" +
      '<div class="grid grid-dash">' +
        '<div class="stack">' + appsCard + hourlyCard + "</div>" +
        '<div class="stack">' + sitesCard + recentCard + "</div>" +
      "</div></div>";
  }

  function kpi(iconName, tone, label, value, foot) {
    return '<div class="card kpi"><div class="kpi-top"><span class="kpi-label">' + esc(label) + "</span>" +
      '<span class="kpi-icon ' + tone + '">' + icon(iconName) + "</span></div>" +
      '<div class="kpi-value">' + esc(value) + "</div>" +
      (foot ? '<div class="kpi-foot">' + esc(foot) + "</div>" : "") + "</div>";
  }

  function sparkline(days) {
    const W = 640, H = 74, pad = 4;
    const max = Math.max(1, ...days.map(function (d) { return d.seconds; }));
    const step = days.length > 1 ? (W - pad * 2) / (days.length - 1) : 0;
    const pts = days.map(function (d, i) {
      return [pad + i * step, H - pad - (d.seconds / max) * (H - pad * 2)];
    });
    if (!pts.length) return "";
    const line = pts.map(function (p, i) { return (i ? "L" : "M") + p[0].toFixed(1) + "," + p[1].toFixed(1); }).join(" ");
    const area = line + " L" + (W - pad) + "," + (H - pad) + " L" + pad + "," + (H - pad) + " Z";
    const last = pts[pts.length - 1];
    return '<svg class="spark" viewBox="0 0 ' + W + " " + H + '" preserveAspectRatio="none">' +
      '<defs><linearGradient id="sparkFill" x1="0" y1="0" x2="0" y2="1">' +
      '<stop offset="0%" stop-color="var(--accent)" stop-opacity=".2"/>' +
      '<stop offset="100%" stop-color="var(--accent)" stop-opacity="0"/></linearGradient></defs>' +
      '<path class="area" d="' + area + '"/>' +
      '<path class="line" d="' + line + '" vector-effect="non-scaling-stroke"/>' +
      '<circle cx="' + last[0].toFixed(1) + '" cy="' + last[1].toFixed(1) + '" r="3.2" vector-effect="non-scaling-stroke"/></svg>';
  }

  function renderSitesCard(sites) {
    if (!sites.length) {
      return '<div class="card"><div class="card-head"><div class="card-title">' + icon("globe") + esc(t("dash.topSites")) + "</div></div>" +
        emptyState("globe", t("dash.noSites"), t("dash.noSitesSub")) + "</div>";
    }
    const colors = ["var(--accent)", "var(--brick)", "var(--green)", "var(--amber)", "var(--muted)"];
    const top = sites.slice(0, 4);
    const restSeconds = sites.slice(4).reduce(function (sum, s) { return sum + s.seconds; }, 0);
    const totalSites = sites.reduce(function (sum, s) { return sum + s.seconds; }, 0) || 1;
    let acc = 0;
    const stops = [];
    top.forEach(function (site, i) {
      const from = (acc / totalSites) * 100;
      acc += site.seconds;
      const to = (acc / totalSites) * 100;
      stops.push(colors[i] + " " + from.toFixed(2) + "% " + to.toFixed(2) + "%");
    });
    if (restSeconds > 0) {
      const from = (acc / totalSites) * 100;
      stops.push("var(--muted) " + from.toFixed(2) + "% 100%");
    }
    const gradient = stops.length ? "conic-gradient(" + stops.join(",") + ")" : "conic-gradient(var(--line) 0 100%)";
    const legend = top.map(function (site, i) {
      return '<div class="site-row"><span class="site-dot" style="background:' + colors[i] + '"></span>' +
        '<span class="site-name" title="' + esc(site.name) + '">' + esc(site.name) + "</span>" +
        '<span class="site-time">' + esc(fmtCompact(site.seconds)) + "</span></div>";
    }).join("") +
      (restSeconds > 0 ? '<div class="site-row"><span class="site-dot" style="background:var(--muted)"></span>' +
        '<span class="site-name">' + esc(t("dash.other")) + '</span><span class="site-time">' + esc(fmtCompact(restSeconds)) + "</span></div>" : "");

    return '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("globe") + esc(t("dash.topSites")) + "</div>" +
      '<div class="card-sub">' + esc(sites.length + " " + t("dash.totalSites")) + "</div></div></div>" +
      '<div class="donut-wrap"><div class="donut-holder"><div class="donut" style="background:' + gradient + '"></div>' +
      '<div class="donut-center"><b>' + esc(fmtCompact(totalSites)) + "</b><span>" + esc(t("dash.topSites")) + "</span></div></div>" +
      '<div class="site-list">' + legend + "</div></div></div>";
  }

  function renderHourly(hourly, date) {
    const max = Math.max(1, ...hourly);
    const nowHour = date === isoDate(new Date()) ? new Date().getHours() : -1;
    const cols = hourly.map(function (value, hour) {
      const pct = value > 0 ? Math.max(3, (value / max) * 100) : 1.5;
      return '<div class="hour-col' + (hour === nowHour ? " now" : "") + '" data-tip="' +
        String(hour).padStart(2, "0") + ":00 · " + esc(fmtDuration(value)) + '">' +
        '<div class="hour-bar" style="height:' + pct + '%"></div></div>';
    }).join("");
    return '<div class="hours">' + cols + "</div>" +
      '<div class="hour-axis"><span>00</span><span>06</span><span>12</span><span>18</span><span>23</span></div>';
  }

  /* ---------- stats ---------- */

  function renderStats() {
    if (state.loading) return skeletonPage();
    const d = state.stats;
    const rangeButtons = [
      ["today", t("stats.today")],
      ["7", t("stats.last7")],
      ["30", t("stats.last30")],
      ["custom", t("stats.custom")]
    ].map(function (pair) {
      return '<button class="' + (state.statsRange === pair[0] ? "active" : "") + '" data-action="range" data-range="' + pair[0] + '">' + esc(pair[1]) + "</button>";
    }).join("");

    const custom = state.statsRange === "custom"
      ? '<form data-form="custom-range" class="row" style="gap:8px;margin-top:10px">' +
        '<div class="field" style="gap:3px"><label>' + esc(t("stats.from")) + '</label><input class="input mono" type="date" name="from" value="' + esc(state.customFrom) + '" style="width:150px;height:34px"></div>' +
        '<div class="field" style="gap:3px"><label>' + esc(t("stats.to")) + '</label><input class="input mono" type="date" name="to" value="' + esc(state.customTo) + '" style="width:150px;height:34px"></div>' +
        '<button class="btn btn-sm" style="align-self:flex-end">' + esc(t("common.refresh")) + "</button></form>"
      : "";

    if (!d) {
      return '<div class="page">' + pageHead(fmtDateLong(isoDate(new Date())), t("stats.title"), t("stats.subtitle")) +
        '<div class="card"><div class="segmented">' + rangeButtons + "</div>" + custom + "</div></div>";
    }

    const delta = d.prevTotal > 0 ? ((d.total - d.prevTotal) / d.prevTotal) * 100 : null;
    const empty = d.total <= 0 && (!d.sessions || !d.sessions.length);
    const summary =
      '<div class="grid grid-4">' +
        kpi("clock", "", t("stats.totalTime"), fmtDuration(d.total), d.days ? t("stats.days", { n: d.days }) : "") +
        kpi("chart", "green", t("stats.dailyAvg"), fmtDuration(d.avgPerDay), "") +
        kpi("spark", "amber", t("stats.peakDay"), d.peakDay ? fmtDateShort(d.peakDay.date) : "—", d.peakDay ? fmtDuration(d.peakDay.seconds) : "") +
        '<div class="card kpi"><div class="kpi-top"><span class="kpi-label">' + esc(t("stats.prevPeriod")) + '</span><span class="kpi-icon">' + icon("refresh") + "</span></div>" +
          '<div class="kpi-value">' + (delta === null ? "—" : (delta > 0 ? "+" : "") + Math.round(delta) + "%") + "</div>" +
          '<div class="kpi-foot">' + esc(t("stats.prevTotal", { time: fmtDuration(d.prevTotal) })) + "</div></div>" +
      "</div>";

    const maxDay = Math.max(1, ...d.series.map(function (s) { return s.seconds; }));
    const bars = d.series.map(function (day, i) {
      const pct = day.seconds > 0 ? Math.max(2.5, (day.seconds / maxDay) * 100) : 1.5;
      const label = fmtDateShort(day.date);
      return '<div class="bar-col' + (day.isToday ? " today" : "") + (day.seconds ? "" : " empty") +
        '" data-tip="' + esc(label + " · " + fmtDuration(day.seconds)) + '">' +
        '<div class="fill" style="height:' + pct + '%"></div></div>';
    }).join("");
    const labels = (function () {
      if (!d.series.length) return "";
      const picks = [0, Math.floor((d.series.length - 1) / 2), d.series.length - 1];
      const seen = {};
      return picks.filter(function (i) { return !seen[i] && (seen[i] = true); }).map(function (i) {
        return "<span>" + esc(fmtDateShort(d.series[i].date)) + "</span>";
      }).join("");
    })();

    const chartCard = '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("chart") + esc(t("stats.byDay")) + "</div>" +
      '<div class="card-sub">' + esc(fmtDateShort(d.from) + " — " + fmtDateShort(d.to)) + "</div></div>" +
      '<div class="mono faint" style="font-size:11px">' + esc(fmtDuration(d.total)) + "</div></div>" +
      '<div class="bars">' + bars + "</div>" + '<div class="bar-x">' + labels + "</div></div>";

    const total = Math.max(1, d.total);
    const appRows = (d.apps || []).map(function (app) {
      const share = (app.seconds / total) * 100;
      return "<tr><td><div class=\"cell-app\">" + appIcon(app) + '<div style="min-width:0"><div class="name">' +
        esc(app.name) + '</div><div class="exe">' + esc(app.exe || "") + "</div></div></div></td>" +
        '<td class="num mono">' + esc(fmtCompact(app.seconds)) + "</td>" +
        '<td><span class="share-bar"><i style="width:' + Math.max(2, share) + '%"></i></span><span class="mono faint" style="font-size:10.5px">' + fmtPercent(share) + "</span></td>" +
        '<td class="num mono">' + fmtNumber(app.opens) + "</td>" +
        '<td class="num mono">' + (app.limitMinutes ? app.limitMinutes + "m" : t("stats.noLimit")) + "</td></tr>";
    }).join("");

    const appCard = '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("layers") + esc(t("stats.byApp")) + "</div>" +
      '<div class="card-sub">' + esc(t("stats.appsCount", { n: (d.apps || []).length })) + "</div></div></div>" +
      ((d.apps || []).length
        ? '<div class="table-wrap"><table class="app-table"><thead><tr><th>' + esc(t("stats.colApp")) + "</th><th class=\"num\">" + esc(t("stats.colTime")) +
          '</th><th>' + esc(t("stats.colShare")) + '</th><th class="num">' + esc(t("stats.colOpens")) + '</th><th class="num">' + esc(t("stats.colLimit")) + "</th></tr></thead><tbody>" + appRows + "</tbody></table></div>"
        : emptyState("layers", t("stats.empty"), t("stats.emptySub"))) + "</div>";

    const siteTotal = (d.sites || []).reduce(function (sum, s) { return sum + s.seconds; }, 0) || 1;
    const siteRows = (d.sites || []).slice(0, 12).map(function (site) {
      const share = (site.seconds / siteTotal) * 100;
      return '<div class="app-row"><span class="app-icon">' + initial(site.name) + "</span>" +
        '<div style="min-width:0"><div class="app-name">' + esc(site.name) + "</div>" +
        '<div class="app-bar"><i style="width:' + Math.max(3, share) + '%"></i></div></div>' +
        '<div class="app-time">' + esc(fmtCompact(site.seconds)) + "</div></div>";
    }).join("");
    const siteCard = '<div class="card"><div class="card-head"><div class="card-title">' + icon("globe") + esc(t("stats.bySite")) + "</div>" +
      '<span class="mono faint" style="font-size:11px">' + esc(fmtCompact(siteTotal)) + "</span></div>" +
      ((d.sites || []).length ? '<div class="app-list">' + siteRows + "</div>" : emptyState("globe", t("dash.noSites"), t("dash.noSitesSub"))) + "</div>";

    const sessionRows = (d.sessions || []).map(function (s) {
      return '<tr><td class="mono" style="white-space:nowrap;color:var(--faint);font-size:10.5px">' + esc(fmtDateShort(s.date)) + "<br>" + esc(fmtTime(s.start)) + "</td>" +
        '<td><div class="cell-app">' + appIcon({ name: s.app, exe: s.exe }) + '<div style="min-width:0"><div class="name">' + esc(s.app) + "</div>" +
        '<div class="exe">' + esc(s.title || s.exe || "") + "</div></div></div></td>" +
        '<td class="num mono">' + esc(s.active ? '<span class="live-dot">live</span>' : fmtCompact(s.seconds)) + "</td></tr>";
    }).join("");
    const sessionCard = '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("clock") + esc(t("stats.sessionsTitle")) + "</div>" +
      '<div class="card-sub">' + esc(t("stats.latest")) + "</div></div></div>" +
      ((d.sessions || []).length
        ? '<div class="table-wrap" style="max-height:380px;overflow-y:auto"><table class="app-table"><thead><tr><th>' + esc(t("common.start")) + "</th><th>" +
          esc(t("stats.colApp")) + '</th><th class="num">' + esc(t("common.duration")) + "</th></tr></thead><tbody>" + sessionRows + "</tbody></table></div>"
        : emptyState("clock", t("stats.noSessionsRange"), "")) + "</div>";

    return '<div class="page">' +
      pageHead(fmtDateShort(d.from) + " — " + fmtDateShort(d.to), t("stats.title"), t("stats.subtitle"),
        '<div class="segmented">' + rangeButtons + "</div>") +
      (custom ? '<div style="margin:-10px 0 16px">' + custom + "</div>" : "") +
      (empty ? '<div class="card" style="margin-bottom:16px">' + emptyState("chart", t("stats.empty"), t("stats.emptySub")) + "</div>" : "") +
      summary +
      '<div style="margin:16px 0">' + chartCard + "</div>" +
      '<div class="grid grid-dash">' +
        '<div class="stack">' + appCard + sessionCard + "</div>" +
        '<div class="stack">' + siteCard + "</div>" +
      "</div></div>";
  }

  /* ---------- focus ---------- */

  function renderFocus() {
    const settings = state.settings;
    if (!settings) return skeletonPage();
    const p = state.pomodoro || { active: false, remainingSeconds: 0, totalMinutes: 0 };
    const active = !!p.active;
    const finished = active && p.remainingSeconds <= 0;
    const configured = state.selectedPreset || settings.pomodoroMinutes || 25;
    const totalMinutes = active ? p.totalMinutes : configured;
    const totalSeconds = Math.max(1, totalMinutes * 60);
    const remaining = active ? p.remainingSeconds : configured * 60;
    const fraction = Math.max(0, Math.min(1, remaining / totalSeconds));
    const circumference = 2 * Math.PI * 118;
    const offset = circumference * (1 - fraction);
    const presets = [25, 45, 60, 90];
    if (presets.indexOf(settings.pomodoroMinutes) < 0) presets.unshift(settings.pomodoroMinutes);
    const presetHTML = presets.map(function (minutes) {
      return '<button class="preset' + (!active && configured === minutes ? " active" : "") + '" data-action="preset" data-minutes="' + minutes + '"' + (active ? " disabled" : "") + ">" + minutes + " " + esc(t("focus.minutesUnit")) + "</button>";
    }).join("");

    const timerCard = '<div class="card timer-card">' +
      '<div class="eyebrow" style="margin-bottom:4px">' + esc(t("focus.preset")) + "</div>" +
      '<div class="ring-holder"><svg class="ring-svg" viewBox="0 0 268 268">' +
        '<circle class="ring-bg" cx="134" cy="134" r="118"/>' +
        '<circle class="ring-fg" id="ringFg" cx="134" cy="134" r="118" stroke-dasharray="' + circumference.toFixed(2) +
        '" stroke-dashoffset="' + offset.toFixed(2) + '"/></svg>' +
        '<div class="ring-center"><div class="timer-value" id="timerValue">' + esc(active ? fmtClock(remaining) : String(configured).padStart(2, "0") + ":00") + "</div>" +
        '<div class="timer-state" id="timerState">' + esc(active ? (finished ? t("focus.done") : t("focus.running")) : t("focus.ready")) + "</div>" +
        '<div class="timer-sub">' + esc(active ? fmtDuration(totalSeconds) : t("focus.activeHint")) + "</div></div></div>" +
      '<div class="preset-row">' + presetHTML + "</div>" +
      '<div class="focus-actions">' +
        (active
          ? '<button class="btn btn-danger btn-lg" data-action="stop-pomodoro">' + icon("pause") + esc(t("focus.stop")) + "</button>"
          : '<button class="btn btn-primary btn-lg" data-action="start-pomodoro">' + icon("play") + esc(t("focus.start")) + "</button>") +
      "</div>" +
      '<div class="card-sub" style="margin-top:14px">' + esc(t("focus.notifyNote")) + "</div></div>";

    const breakCard = '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("spark") + esc(t("focus.breakReminder")) + "</div>" +
      '<div class="card-sub">' + esc(t("focus.breakDesc")) + "</div></div>" +
      switchHTML(settings.breakReminder.enabled, "break-enabled") + "</div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("focus.breakEvery")) + "</b></div>" +
      '<div class="setting-control"><input class="input mono" type="number" min="1" max="600" step="1" data-setting="break-minutes" value="' +
      settings.breakReminder.minutes + '" style="width:84px"><span class="faint" style="font-size:11px">' + esc(t("settings.minutes")) + "</span></div></div></div>";

    const defaultCard = '<div class="card"><div class="card-head"><div class="card-title">' + icon("timer") + esc(t("focus.default")) + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("focus.defaultLength")) + "</b><span>" + esc(t("focus.defaultHint")) + "</span></div>" +
      '<div class="setting-control"><select class="select" data-setting="pomodoro-minutes">' + pomodoroOptions(settings.pomodoroMinutes) + "</select></div></div></div>";

    const quoteCard = '<div class="card" style="background:linear-gradient(145deg,var(--accent-soft),transparent 70%)">' +
      '<div class="focus-quote">' + esc(t("focus.quote")) + "<small>" + esc(t("focus.quoteSub")) + "</small></div></div>";

    return '<div class="page">' +
      pageHead(t("nav.focus"), t("focus.title"), t("focus.subtitle")) +
      '<div class="focus-hero"><div class="stack">' + timerCard + "</div>" +
      '<div class="focus-side">' + breakCard + defaultCard + quoteCard + "</div></div></div>";
  }

  function pomodoroOptions(current) {
    const values = [15, 20, 25, 30, 45, 50, 60, 90];
    if (values.indexOf(current) < 0) values.push(current);
    values.sort(function (a, b) { return a - b; });
    return values.map(function (m) {
      return '<option value="' + m + '"' + (m === current ? " selected" : "") + ">" + m + " " + esc(t("settings.minutes")) + "</option>";
    }).join("");
  }

  /* ---------- limits ---------- */

  function renderLimits() {
    const data = state.limitsData || { limits: [], sites: [], events: [] };
    const limits = data.limits || [];
    const rows = limits.map(function (lim) {
      const pct = Math.min(100, Math.max(0, lim.percent));
      const opens = lim.mode === "opens";
      const remaining = opens ? Math.max(0, (lim.maxOpens || 0) - (lim.opensUsed || 0)) : lim.minutes * 60 - lim.usedSeconds;
      const foot = opens
        ? t("limits.opensLeft", { n: remaining })
        : (lim.status === "over"
          ? t("limits.overBy", { time: fmtDuration(Math.abs(remaining)) })
          : t("limits.remaining", { time: fmtDuration(Math.max(0, remaining)) }));
      const meta = opens
        ? t("limits.opensMeta", { used: lim.opensUsed || 0, max: lim.maxOpens || 0, min: lim.minutesPerOpen || 0 })
        : esc(fmtDuration(lim.usedSeconds)) + " / " + esc(fmtDuration(lim.minutes * 60)) + " · " + esc(t(lim.status === "over" ? "limits.statusOver" : "limits.limit"));
      return '<div class="limit-row ' + esc(lim.status) + '">' +
        '<span class="app-icon">' + initial(lim.name) + "</span>" +
        '<div style="min-width:0"><div class="between" style="gap:8px"><div class="limit-name">' + esc(lim.name) + "</div>" +
          '<span class="status-pill ' + esc(lim.status) + '">' + esc(t("limits.status" + cap(lim.status))) + "</span></div>" +
          '<div class="limit-meta">' + meta + "</div>" +
          '<div class="limit-progress"><i style="width:' + pct + '%"></i></div>' +
          '<div class="limit-meta" style="margin-top:5px">' + esc(foot) + "</div></div>" +
        '<div class="limit-time">' + pct + "%<br><small>" + esc(opens ? (lim.maxOpens || 0) + " " + t("limits.opensUnit") : fmtDuration(lim.minutes * 60)) + "</small></div>" +
        '<button class="icon-btn" title="' + esc(t("limits.delete")) + '" data-action="delete-limit" data-key="' + esc(lim.key) + '" data-name="' + esc(lim.name) + '">' + icon("trash") + "</button></div>";
    }).join("");

    const sites = data.sites || [];
    const siteRows = sites.map(function (lim) {
      const pct = Math.min(100, Math.max(0, lim.percent));
      const remaining = lim.minutes * 60 - lim.usedSeconds;
      const foot = remaining <= 0
        ? t("limits.overBy", { time: fmtDuration(Math.abs(remaining)) })
        : t("limits.remaining", { time: fmtDuration(remaining) });
      return '<div class="limit-row ' + esc(lim.status) + '">' +
        '<span class="app-icon">' + icon("globe") + "</span>" +
        '<div style="min-width:0"><div class="between" style="gap:8px"><div class="limit-name">' + esc(lim.label) + "</div>" +
          '<span class="status-pill ' + esc(lim.status) + '">' + esc(t("limits.status" + cap(lim.status))) + "</span></div>" +
          '<div class="limit-meta">' + esc(fmtDuration(lim.usedSeconds)) + " / " + esc(fmtDuration(lim.minutes * 60)) + " · " + esc(lim.key) + "</div>" +
          '<div class="limit-progress"><i style="width:' + pct + '%"></i></div>' +
          '<div class="limit-meta" style="margin-top:5px">' + esc(foot) + "</div></div>" +
        '<div class="limit-time">' + pct + "%<br><small>" + esc(fmtDuration(lim.minutes * 60)) + "</small></div>" +
        '<button class="icon-btn" title="' + esc(t("limits.delete")) + '" data-action="delete-site-limit" data-key="' + esc(lim.key) + '" data-name="' + esc(lim.label) + '">' + icon("trash") + "</button></div>";
    }).join("");

    const events = data.events || [];
    const eventRows = events.slice(0, 12).map(function (e) {
      const action = e.action === "bypassed" ? t("limits.eventBypassed") : t("limits.eventBlocked");
      return '<div class="event-row"><span class="event-time">' + esc(fmtDateShort(e.date)) + " " + esc(fmtTime(e.time)) + "</span>" +
        '<span class="event-action ' + esc(e.action) + '">' + esc(action) + "</span>" +
        "<b>" + esc(e.target) + "</b>" +
        '<span class="event-reason">' + esc(e.reason || e.kind) + "</span></div>";
    }).join("");

    const siteForm = '<form class="site-limit-form" data-form="site-limit">' +
      '<input class="input" name="site" placeholder="' + esc(t("limits.sitePlaceholder")) + '" autocomplete="off">' +
      '<input class="input mono" name="minutes" type="number" min="1" max="1440" step="1" value="30" style="width:86px">' +
      '<button class="btn btn-sm" type="submit">' + esc(t("limits.addSite")) + "</button></form>";

    const appCard = limits.length
      ? '<div class="card">' + rows + "</div>"
      : '<div class="card">' + emptyState("shield", t("limits.empty"), t("limits.emptySub")) +
        '<div style="text-align:center;margin-top:4px"><button class="btn btn-primary" data-action="open-limit">' + icon("plus") + esc(t("limits.add")) + "</button></div></div>";

    return '<div class="page">' +
      pageHead(t("nav.limits"), t("limits.title"), t("limits.subtitle"),
        '<button class="btn btn-primary" data-action="open-limit">' + icon("plus") + esc(t("limits.add")) + "</button>") +
      appCard +
      '<div class="card site-limits" style="margin-top:16px"><div class="card-head"><div><div class="card-title">' + icon("globe") + esc(t("limits.sitesTitle")) + "</div>" +
        '<div class="card-sub">' + esc(t("limits.sitesSub")) + "</div></div></div>" +
        (siteRows || '<div class="chips-empty" style="padding:4px 0 10px">' + esc(t("limits.sitesEmpty")) + "</div>") +
        siteForm + "</div>" +
      '<div class="card" style="margin-top:16px"><div class="card-head"><div><div class="card-title">' + icon("alert") + esc(t("limits.eventsTitle")) + "</div>" +
        '<div class="card-sub">' + esc(t("limits.eventsSub", { n: data.blockedToday || 0, m: data.bypassedToday || 0 })) + "</div></div></div>" +
        (eventRows || '<div class="chips-empty" style="padding:4px 0">' + esc(t("limits.eventsEmpty")) + "</div>") + "</div>" +
      '<div class="card" style="margin-top:16px;display:flex;gap:12px;align-items:flex-start">' +
        '<span class="kpi-icon" style="flex:none">' + icon("info") + "</span><div><b style=\"font-size:12.5px\">" + esc(t("limits.addTitle")) + "</b>" +
        '<p class="muted" style="margin:4px 0 0;font-size:12px">' + esc(t("limits.note")) + "</p></div></div>" +
      "</div>";
  }

  function cap(value) {
    return String(value).charAt(0).toUpperCase() + String(value).slice(1);
  }

  function openLimitModal(prefill) {
    const data = state.limitsData || { suggestions: [] };
    const suggestions = data.suggestions || [];
    const preset = prefill || {};
    const rows = suggestions.slice(0, 30).map(function (item) {
      return '<button type="button" class="suggest-item" data-suggest data-key="' + esc(item.key) + '" data-name="' + esc(item.name) + '">' +
        '<span class="app-icon">' + initial(item.name) + "</span>" +
        '<span class="meta"><b>' + esc(item.name) + "</b><span>" + esc(item.key) + "</span></span>" +
        (item.limited ? '<span class="status-pill">' + esc(t("limits.limit")) + "</span>" : '<span class="mono faint" style="font-size:10.5px">' + esc(fmtCompact(item.seconds)) + "</span>") +
        "</button>";
    }).join("");

    openModal(
      '<div class="modal-head"><h3>' + esc(t("limits.addTitle")) + "</h3><p>" + esc(t("limits.addSub")) + "</p></div>" +
      '<form id="limitForm"><div class="modal-body">' +
        '<div class="field"><label>' + esc(t("limits.suggestions")) + "</label>" +
          (rows ? '<div class="suggest-list">' + rows + "</div>" : '<div class="chips-empty">' + esc(t("dash.noApps")) + "</div>") + "</div>" +
        '<div class="field"><label for="limitKey">' + esc(t("limits.customExe")) + "</label>" +
          '<input class="input" id="limitKey" name="key" autocomplete="off" placeholder="steam.exe" value="' + esc(preset.key || "") + '">' +
          '<span class="hint">' + esc(t("limits.customHint")) + "</span></div>" +
        '<div class="field"><label>' + esc(t("limits.modeLabel")) + "</label>" +
          '<div style="display:flex;gap:8px"><button type="button" class="btn btn-sm mode-btn selected" data-limit-mode="time">' + esc(t("limits.modeTime")) + "</button>" +
          '<button type="button" class="btn btn-sm mode-btn" data-limit-mode="opens">' + esc(t("limits.modeOpens")) + "</button></div></div>" +
        '<div id="timeFields"><div class="field"><label for="limitMinutes">' + esc(t("limits.minutesLabel")) + "</label>" +
          '<input class="input mono" id="limitMinutes" name="minutes" type="number" min="1" max="1440" step="1" value="60"></div></div>' +
        '<div id="openFields" style="display:none">' +
          '<div class="field"><label for="maxOpens">' + esc(t("limits.maxOpensLabel")) + "</label>" +
            '<input class="input mono" id="maxOpens" name="maxOpens" type="number" min="1" max="100" step="1" value="3"></div>' +
          '<div class="field"><label for="minutesPerOpen">' + esc(t("limits.perOpenLabel")) + "</label>" +
            '<input class="input mono" id="minutesPerOpen" name="minutesPerOpen" type="number" min="1" max="1440" step="1" value="15"></div>' +
          '<span class="hint">' + esc(t("limits.openHint")) + "</span></div>" +
        '<div class="chips-empty" data-selected-name style="padding:0;color:var(--accent)">' +
          (preset.name ? esc(preset.name) : "") + "</div>" +
      "</div>" +
      '<div class="modal-foot"><button type="button" class="btn" data-close>' + esc(t("common.cancel")) + "</button>" +
        '<button class="btn btn-primary" type="submit">' + esc(t("limits.saveLimit")) + "</button></div></form>",
      {
        onMount: function (modal) {
          const keyInput = $("#limitKey", modal);
          const selectedName = $("[data-selected-name]", modal);
          let selectedKey = preset.key || "";
          let limitMode = "time";
          $$("[data-limit-mode]", modal).forEach(function (button) {
            button.addEventListener("click", function () {
              limitMode = button.dataset.limitMode;
              $$("[data-limit-mode]", modal).forEach(function (b) { b.classList.toggle("selected", b === button); });
              $("#timeFields", modal).style.display = limitMode === "time" ? "" : "none";
              $("#openFields", modal).style.display = limitMode === "opens" ? "" : "none";
            });
          });

          $$("[data-suggest]", modal).forEach(function (button) {
            button.addEventListener("click", function () {
              $$("[data-suggest]", modal).forEach(function (b) { b.classList.remove("selected"); });
              button.classList.add("selected");
              selectedKey = button.dataset.key;
              keyInput.value = selectedKey;
              selectedName.textContent = button.dataset.name;
            });
          });

          keyInput.addEventListener("input", function () {
            if (keyInput.value.trim()) {
              selectedKey = "";
              $$("[data-suggest]", modal).forEach(function (b) { b.classList.remove("selected"); });
              selectedName.textContent = keyInput.value.trim();
            }
          });

          $("#limitForm", modal).addEventListener("submit", async function (event) {
            event.preventDefault();
            const key = keyInput.value.trim() || selectedKey;
            if (!key) return toast(t("limits.selectApp"), "err");
            try {
              if (limitMode === "opens") {
                const maxOpens = parseInt($("#maxOpens", modal).value, 10);
                const minutesPerOpen = parseInt($("#minutesPerOpen", modal).value, 10);
                if (!maxOpens || maxOpens < 1 || maxOpens > 100) return toast(t("limits.opensInvalid"), "err");
                if (!minutesPerOpen || minutesPerOpen < 1 || minutesPerOpen > 1440) return toast(t("limits.minutesInvalid"), "err");
                await act("limit-set", { key: key, mode: "opens", maxOpens: maxOpens, minutesPerOpen: minutesPerOpen });
              } else {
                const minutes = parseInt($("#limitMinutes", modal).value, 10);
                if (!minutes || minutes < 1 || minutes > 1440) return toast(t("limits.minutesInvalid"), "err");
                await act("limit-set", { key: key, mode: "time", minutes: minutes });
              }
              closeModal();
              toast(t("limits.addedToast"));
              await loadLimits();
              render();
            } catch (err) {
              toastError(err);
            }
          });
        }
      }
    );
  }

  function deleteLimit(key, name) {
    confirmModal(
      t("limits.deleteTitle"),
      esc(t("limits.deleteBody", { name: name })),
      t("limits.removeConfirm"),
      true,
      async function () {
        await act("limit-delete", { key: key });
        toast(t("limits.removedToast"));
        await loadLimits();
        render();
      }
    );
  }

  /* ---------- settings ---------- */

  function renderSettings() {
    const s = state.settings;
    if (!s) return skeletonPage();
    const idle = s.idle || { thresholdMinutes: 5, trackWhileIdle: false, trackWhilePlaying: true };
    const media = s.mediaDetection !== false;

    const tracking = '<div class="card"><div class="card-head"><div class="card-title">' + icon("layers") + esc(t("settings.tracking")) + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.daemon")) + "</b><span>" + esc(t("settings.daemonDesc")) + "</span></div>" +
        '<div class="setting-control"><span class="status-pill' + (s.daemonRunning ? "" : " over") + '">' + esc(s.daemonRunning ? t("settings.running") : t("settings.stopped")) + "</span>" +
        '<button class="btn btn-sm" data-action="' + (s.daemonRunning ? "stop-tracker" : "start-tracker") + '">' + esc(s.daemonRunning ? t("settings.stopTracker") : t("settings.startTracker")) + "</button></div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.autostart")) + "</b><span>" + esc(t("settings.autostartDesc")) + "</span></div>" +
        '<div class="setting-control">' + switchHTML(s.autostart, "autostart") + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.pause")) + "</b><span>" + esc(t("settings.pauseDesc")) + "</span></div>" +
        '<div class="setting-control">' + switchHTML(s.paused, "pause") + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.retention")) + "</b><span>" + esc(t("settings.retentionDesc")) + "</span></div>" +
        '<div class="setting-control"><select class="select" data-setting="retention">' + retentionOptions(s.retentionDays) + "</select></div></div></div>";

    const idleCard = '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("clock") + esc(t("settings.idle")) + "</div>" +
      '<div class="card-sub">' + esc(t("settings.mouseHint")) + "</div></div></div>" +
      '<div class="setting-row" style="display:block"><div class="between"><div class="setting-text"><b>' + esc(t("settings.idleThreshold")) + '</b><span id="idleValue" class="mono accent">' + esc(t("settings.idleValue", { n: idle.thresholdMinutes })) + "</span></div></div>" +
        '<input class="slider" type="range" min="0" max="' + (s.maxIdleMinutes || 240) + '" step="1" value="' + idle.thresholdMinutes + '" data-setting="idle-threshold">' +
        '<div class="hint faint" style="font-size:10.5px;margin-top:2px">' + esc(t("settings.idleThresholdDesc")) + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.trackWhileIdle")) + "</b><span>" + esc(t("settings.trackWhileIdleDesc")) + "</span></div>" +
        '<div class="setting-control">' + switchHTML(idle.trackWhileIdle, "track-idle") + "</div></div>" +
      (media
        ? '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.trackWhilePlaying")) + "</b><span>" + esc(t("settings.trackWhilePlayingDesc")) + "</span></div>" +
          '<div class="setting-control">' + switchHTML(idle.trackWhilePlaying, "track-playing") + "</div></div>"
        : '<div class="setting-row"><div class="setting-text"><span>' + esc(t("settings.mediaUnavailable")) + "</span></div></div>") +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.closeToTray")) + "</b><span>" + esc(t("settings.closeToTrayDesc")) + "</span></div>" +
      '<div class="setting-control">' + switchHTML(!!s.closeToTray, "close-to-tray") + "</div></div>" +
      "</div>";

    const experimental = '<div class="card experimental-card"><div class="card-head"><div><div class="card-title">' + icon("globe") + esc(t("settings.experimental")) +
      ' <span class="badge">beta</span></div>' +
      '<div class="card-sub">' + esc(t("settings.domainGroupingHint")) + "</div></div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.domainGrouping")) + "</b><span>" + esc(t("settings.domainGroupingDesc")) + "</span></div>" +
      '<div class="setting-control">' + switchHTML(!!s.domainGrouping, "domain-grouping") + "</div></div></div>";

    const webGuard = '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("shield") + esc(t("settings.webGuard")) + "</div>" +
      '<div class="card-sub">' + esc(t("settings.webGuardDesc")) + "</div></div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.strongWeb")) + "</b><span>" + esc(t("settings.strongWebDesc")) + "</span></div>" +
        '<div class="setting-control">' + switchHTML(!!s.strongWebMode, "strong-web") + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.extensionStatus")) + "</b><span>" +
        esc(s.extensionActive ? t("settings.extensionConnected", { browsers: (s.extensionBrowsers || []).join(", ") }) : t("settings.extensionMissing")) + "</span></div>" +
        '<div class="setting-control"><span class="status-pill ' + (s.extensionActive ? "" : "warn") + '">' + esc(s.extensionActive ? t("settings.extensionOn") : t("settings.extensionOff")) + "</span></div></div>" +
      '<div class="setting-row" style="display:block"><div class="setting-text" style="margin-bottom:6px"><b>' + esc(t("settings.webAllowlist")) + "</b><span>" + esc(t("settings.webAllowlistDesc")) + "</span></div>" +
        '<div class="chips">' + chipsHTML(s.webAllowlist, "remove-web-allow", t("settings.webAllowlistEmpty")) + "</div>" +
        '<form class="chip-add" data-form="web-allowlist"><input class="input" name="value" placeholder="google.com" autocomplete="off"><button class="btn btn-sm" type="submit">' + esc(t("settings.addAllow")) + "</button></form>" +
        '<p class="hint faint" style="font-size:10.5px;margin:8px 0 0">' + esc(t("settings.extensionHint")) + "</p></div></div>";


      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.breakReminder")) + "</b><span>" + esc(t("focus.breakDesc")) + "</span></div>" +
        '<div class="setting-control">' + switchHTML(s.breakReminder.enabled, "break-enabled") + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.breakEvery")) + "</b></div>" +
        '<div class="setting-control"><input class="input mono" type="number" min="1" max="600" step="1" value="' + s.breakReminder.minutes + '" data-setting="break-minutes" style="width:86px"><span class="faint" style="font-size:11px">' + esc(t("settings.minutes")) + "</span></div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.pomodoroDefault")) + "</b></div>" +
        '<div class="setting-control"><select class="select" data-setting="pomodoro-minutes">' + pomodoroOptions(s.pomodoroMinutes) + "</select></div></div></div>";

    const breaks = '<div class="card"><div class="card-head"><div class="card-title">' + icon("spark") + esc(t("settings.breaks")) + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.breakReminder")) + "</b><span>" + esc(t("focus.breakDesc")) + "</span></div>" +
        '<div class="setting-control">' + switchHTML(s.breakReminder.enabled, "break-enabled") + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.breakEvery")) + "</b></div>" +
        '<div class="setting-control"><input class="input mono" type="number" min="1" max="600" step="1" value="' + s.breakReminder.minutes + '" data-setting="break-minutes" style="width:86px"><span class="faint" style="font-size:11px">' + esc(t("settings.minutes")) + "</span></div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.pomodoroDefault")) + "</b></div>" +
        '<div class="setting-control"><select class="select" data-setting="pomodoro-minutes">' + pomodoroOptions(s.pomodoroMinutes) + "</select></div></div></div>";

    const privacy = '<div class="card"><div class="card-head"><div class="card-title">' + icon("lock") + esc(t("settings.privacy")) + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.redact")) + "</b><span>" + esc(t("settings.redactDesc")) + "</span></div>" +
        '<div class="setting-control">' + switchHTML(s.redactTitles, "redact") + "</div></div>" +
      '<div class="setting-row" style="display:block"><div class="setting-text" style="margin-bottom:6px"><b>' + esc(t("settings.whitelist")) + "</b><span>" + esc(t("settings.whitelistDesc")) + "</span></div>" +
        '<div class="chips">' + chipsHTML(s.whitelist, "remove-whitelist", t("settings.whitelistEmpty")) + "</div>" +
        '<p class="faint" style="font-size:11px;margin:0 0 8px">' + esc(t("settings.whitelistBuiltin")) + "</p>" +
        '<form class="chip-add" data-form="whitelist"><input class="input" name="value" placeholder="' + esc(t("settings.addExe")) + '" autocomplete="off"><button class="btn btn-sm" type="submit">' + esc(t("settings.addApp")) + "</button></form></div>" +
      '<div class="setting-row" style="display:block"><div class="setting-text" style="margin-bottom:6px"><b>' + esc(t("settings.browsers")) + "</b><span>" + esc(t("settings.browsersDesc")) + "</span></div>" +
        '<div class="chips">' + chipsHTML(s.customBrowsers, "remove-browser", t("settings.browsersEmpty")) + "</div>" +
        '<form class="chip-add" data-form="browser"><input class="input" name="value" placeholder="' + esc(t("settings.addExe")) + '" autocomplete="off"><button class="btn btn-sm" type="submit">' + esc(t("settings.addApp")) + "</button></form></div></div>";

    const archive = '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("download") + esc(t("settings.keepHistory")) + "</div>" +
      '<div class="card-sub">' + esc(t("settings.keepHistoryDesc")) + "</div></div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.keepHistoryToggle")) + "</b><span>" + esc(t("settings.keepHistoryHint")) + "</span></div>" +
        '<div class="setting-control">' + switchHTML(!!s.keepHistory, "keep-history") + "</div></div>" +
      (s.keepHistory
        ? '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.archiveCount", { n: s.archiveCount || 0 })) + "</b><span>" +
          esc(s.archiveLatest ? t("settings.archiveLatest", { name: s.archiveLatest }) : t("settings.archiveEmpty")) + "</span></div>" +
          '<div class="setting-control"><button class="btn btn-sm" data-action="open-archive">' + esc(t("settings.openArchive")) + "</button></div></div>"
        : "") +
      "</div>";

    const quotes = '<div class="card"><div class="card-head"><div><div class="card-title">' + icon("message") + esc(t("settings.quotes")) + "</div>" +
      '<div class="card-sub">' + esc(t("settings.quotesDesc", { n: s.quoteCount || 0 })) + "</div></div></div>" +
      '<button class="btn btn-sm" data-action="quotes">' + esc(t("settings.manageQuotes")) + "</button></div>";

    const dataCard = '<div class="card"><div class="card-head"><div class="card-title">' + icon("download") + esc(t("settings.data")) + "</div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.export")) + "</b><span>" + esc(t("settings.exportDesc")) + "</span></div>" +
        '<div class="setting-control"><button class="btn btn-sm" data-action="export" data-format="csv">' + esc(t("settings.exportCsv")) + "</button>" +
        '<button class="btn btn-sm" data-action="export" data-format="json">' + esc(t("settings.exportJson")) + "</button></div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.openFolder")) + "</b><span>" + esc(t("settings.openFolderDesc")) + "</span></div>" +
        '<div class="setting-control"><button class="btn btn-sm" data-action="open-folder">' + icon("folder") + "</button></div></div>" +
      '<div class="setting-row"><div class="setting-text"><b>' + esc(t("settings.update")) + "</b><span>" + esc(t("settings.updateDesc")) + "</span></div>" +
        '<div class="setting-control"><button class="btn btn-sm" data-action="update">' + icon("external") + "</button></div></div></div>";

    const danger = '<div class="card danger-zone"><div class="card-head"><div><div class="card-title">' + icon("alert") + esc(t("settings.danger")) + "</div>" +
      '<div class="card-sub">' + esc(t("settings.wipeDesc")) + "</div></div></div>" +
      '<button class="btn btn-danger" data-action="wipe-modal">' + icon("trash") + esc(t("settings.wipe")) + "</button></div>";

    const about = '<div class="card"><div class="card-head"><div class="card-title">' + icon("info") + esc(t("settings.about")) + "</div></div>" +
      '<p class="muted" style="font-size:12px;margin:0 0 10px">' + esc(t("settings.aboutText")) + "</p>" +
      '<div class="setting-row" style="padding-top:0;border-top:0"><div class="setting-text"><b>' + esc(t("settings.notices")) + '</b><span>' + esc(t("settings.noticesDesc")) + "</span></div>" +
        '<div class="setting-control"><button class="btn btn-sm" data-action="notices">' + esc(t("settings.notices")) + "</button></div></div>" +
      '<div class="about-line"><span>' + esc(t("common.version")) + "</span><b>v" + esc(s.version || state.version) + "</b></div>" +
      '<div class="about-line"><span>' + esc(t("settings.database")) + '</span><b title="' + esc(s.dbPath) + '">' + esc(s.dbPath) + "</b></div>" +
      '<div class="about-line"><span>' + esc(t("settings.installDir")) + '</span><b title="' + esc(s.installDir) + '">' + esc(s.installDir) + "</b></div>" +
      '<p class="faint" style="font-size:11px;margin:12px 0 0">' + esc(t("settings.privacyNote")) + "</p></div>";

    return '<div class="page">' +
      pageHead(t("nav.settings"), t("settings.title"), t("settings.subtitle")) +
      '<div class="settings-grid">' + tracking + idleCard + experimental + webGuard + archive + quotes + breaks + privacy + dataCard + about + '<div class="full">' + danger + "</div></div></div>";
  }

  function retentionOptions(current) {
    const values = [7, 14, 30, 90, 180, 365];
    if (values.indexOf(current) < 0) values.push(current);
    values.sort(function (a, b) { return a - b; });
    return values.map(function (days) {
      return '<option value="' + days + '"' + (days === current ? " selected" : "") + ">" + days + " " + esc(t("settings.days")) + "</option>";
    }).join("");
  }

  function chipsHTML(items, action, emptyText) {
    if (!items || !items.length) return '<span class="chips-empty">' + esc(emptyText) + "</span>";
    return items.map(function (value) {
      return '<span class="chip">' + esc(value) + '<button class="x" data-action="' + action + '" data-value="' + esc(value) + '" title="' + esc(t("settings.remove")) + '">×</button></span>';
    }).join("");
  }

  function openWipeModal() {
    openModal(
      '<div class="modal-head"><h3>' + esc(t("settings.wipeTitle")) + "</h3><p>" + esc(t("settings.wipeSub")) + "</p></div>" +
      '<div class="modal-body"><div class="field"><input class="input mono" id="wipeInput" placeholder="' + esc(t("settings.wipePlaceholder")) + '" autocomplete="off"></div></div>' +
      '<div class="modal-foot"><button class="btn" data-close>' + esc(t("common.cancel")) + "</button>" +
        '<button class="btn btn-danger" id="wipeConfirm">' + esc(t("settings.wipeConfirm")) + "</button></div>",
      {
        onMount: function (modal) {
          $("[data-close]", modal).addEventListener("click", closeModal);
          $("#wipeConfirm", modal).addEventListener("click", async function () {
            const value = $("#wipeInput", modal).value.trim();
            if (value !== "DELETE") return toast(t("settings.wipeMismatch"), "err");
            this.disabled = true;
            try {
              await act("wipe", { confirm: "DELETE" });
              closeModal();
              toast(t("settings.wipeDone"));
              await loadEverything(false);
              render();
            } catch (err) {
              toastError(err);
              this.disabled = false;
            }
          });
        }
      }
    );
  }

  function quoteEditRow(id, text) {
    return '<div class="quote-edit" data-id="' + esc(id || "") + '">' +
      '<textarea class="input" rows="2" maxlength="240">' + esc(text || "") + "</textarea>" +
      '<button type="button" class="icon-btn" data-remove-quote title="' + esc(t("settings.quoteDelete")) + '">' + icon("trash") + "</button></div>";
  }

  async function openQuotesModal() {
    let data;
    try {
      data = await api("/api/quotes");
    } catch (err) {
      toastError(err);
      return;
    }
    const list = data.quotes || [];
    const body = '<div class="quote-edit-list">' + list.map(function (q) { return quoteEditRow(q.id, q.text); }).join("") + "</div>" +
      '<button type="button" class="btn btn-sm" id="addQuote" style="margin-top:10px">' + icon("plus") + " " + esc(t("settings.quoteAdd")) + "</button>";
    openModal(
      '<div class="modal-head"><h3>' + esc(t("settings.quotesTitle")) + "</h3><p>" + esc(t("settings.quotesSub")) + "</p></div>" +
      '<div class="modal-body">' + body + "</div>" +
      '<div class="modal-foot"><button class="btn" data-close>' + esc(t("common.cancel")) + "</button>" +
        '<button class="btn btn-primary" id="saveQuotes">' + esc(t("common.save")) + "</button></div>",
      {
        onMount: function (modal) {
          const listEl = $(".quote-edit-list", modal);
          $("[data-close]", modal).addEventListener("click", closeModal);
          $("#addQuote", modal).addEventListener("click", function () {
            listEl.insertAdjacentHTML("beforeend", quoteEditRow("", ""));
            const ta = listEl.lastElementChild.querySelector("textarea");
            if (ta) ta.focus();
          });
          listEl.addEventListener("click", async function (event) {
            const btn = event.target.closest("[data-remove-quote]");
            if (!btn) return;
            const row = btn.closest(".quote-edit");
            const id = parseInt(row.dataset.id, 10);
            if (!id) { row.remove(); return; }
            if (listEl.querySelectorAll(".quote-edit").length <= 1) return toast(t("settings.quoteLast"), "err");
            try {
              await act("quote-delete", { id: id });
              row.remove();
            } catch (err) {
              toastError(err);
            }
          });
          $("#saveQuotes", modal).addEventListener("click", async function () {
            const rows = $$(".quote-edit", listEl);
            if (!rows.length) return toast(t("settings.quoteLast"), "err");
            this.disabled = true;
            try {
              for (const row of rows) {
                const text = $("textarea", row).value.trim();
                if (!text) continue;
                const id = parseInt(row.dataset.id, 10);
                if (id) await act("quote-update", { id: id, text: text });
                else await act("quote-add", { text: text });
              }
              closeModal();
              await loadSettings();
              render();
              toast(t("settings.saved"));
            } catch (err) {
              toastError(err);
              this.disabled = false;
            }
          });
        }
      }
    );
  }

  function openNoticesModal() {
    const projects = [
      ["Focusd (original engine)", "MIT", "original project by 0xarchit — github.com/0xarchit/Focusd"],
      ["Go", "BSD-3-Clause", "runtime and standard library"],
      ["go-webview2", "MIT", "native WebView2 window (jchv)"],
      ["go-winloader", "MIT", "embedded WebView2Loader (jchv)"],
      ["Bubble Tea", "MIT", "terminal UI runtime (charmbracelet)"],
      ["Lip Gloss", "MIT", "terminal styling (charmbracelet)"],
      ["modernc.org/sqlite", "BSD-3-Clause", "pure-Go SQLite driver"],
      ["Bricolage Grotesque", "SIL OFL 1.1", "display typeface (Mathieu Triay)"]
    ];
    const rows = projects.map(function (item) {
      return '<div class="notice-item"><div><b>' + esc(item[0]) + '</b><span>' + esc(item[2]) + '</span></div>' +
        '<span class="notice-license">' + esc(item[1]) + "</span></div>";
    }).join("");
    openModal(
      '<div class="modal-head"><h3>' + esc(t("settings.noticesTitle")) + "</h3><p>" + esc(t("settings.noticesSub")) + "</p></div>" +
      '<div class="modal-body"><div class="notice-list">' + rows + "</div>" +
      '<p class="faint" style="font-size:11.5px;margin:14px 0 0">' + esc(t("settings.noticesLocal")) + "</p></div>" +
      '<div class="modal-foot"><button class="btn btn-primary" data-close>' + esc(t("settings.noticesClose")) + "</button></div>",
      {
        onMount: function (modal) {
          $("[data-close]", modal).addEventListener("click", closeModal);
        }
      }
    );
  }

  function openShortcuts() {
    const rows = [
      ["⌘ / Ctrl + 1…5", t("shortcuts.nav")],
      ["R", t("shortcuts.refresh")],
      ["T", t("shortcuts.theme")],
      ["L", t("shortcuts.lang")],
      ["Esc", t("shortcuts.esc")],
      ["?", t("shortcuts.help")]
    ];
    openModal(
      '<div class="modal-head"><h3>' + esc(t("shortcuts.title")) + "</h3></div>" +
      '<div class="modal-body">' + rows.map(function (row) {
        return '<div class="kbd-row"><span>' + esc(row[1]) + "</span><kbd>" + esc(row[0]) + "</kbd></div>";
      }).join("") + "</div>" +
      '<div class="modal-foot"><button class="btn" data-close>' + esc(t("common.close")) + "</button></div>",
      {
        onMount: function (modal) {
          $("[data-close]", modal).addEventListener("click", closeModal);
        }
      }
    );
  }

  /* ---------- actions ---------- */

  async function handleAction(action, node) {
    switch (action) {
      case "pause":
        await setPaused(true);
        break;
      case "resume":
        await setPaused(false);
        break;
      case "retry":
        state.fatal = null;
        state.loading = true;
        render();
        try {
          await loadEverything(true);
          render();
        } catch (err) {
          state.fatal = err;
          render();
        }
        break;
      case "refresh":
        await refreshAll();
        break;
      case "go-focus":
        await go("focus");
        break;
      case "go-stats":
        await go("stats");
        break;
      case "app-limit":
        openLimitModal({ key: node.dataset.key, name: node.dataset.name });
        break;
      case "open-limit":
        openLimitModal();
        break;
      case "delete-limit":
        deleteLimit(node.dataset.key, node.dataset.name);
        break;
      case "delete-site-limit":
        confirmModal(t("limits.deleteTitle"), esc(t("limits.deleteBody", { name: node.dataset.name })), t("limits.removeConfirm"), true, async function () {
          await act("site-limit-delete", { key: node.dataset.key });
          toast(t("limits.removedToast"));
          await loadLimits();
          render();
        });
        break;
      case "remove-web-allow": {
        const list = (state.settings.webAllowlist || []).filter(function (v) { return v !== node.dataset.value; });
        await act("web-allowlist", { allowlist: list });
        await loadSettings();
        render();
        break;
      }
      case "quotes":
        openQuotesModal();
        break;
      case "preset":
        state.selectedPreset = parseInt(node.dataset.minutes, 10) || 25;
        render();
        break;
      case "start-pomodoro":
        await startPomodoro();
        break;
      case "stop-pomodoro":
        await stopPomodoro();
        break;
      case "range":
        state.statsRange = node.dataset.range;
        if (state.statsRange !== "custom") {
          state.stats = null;
          state.loading = true;
          render();
          try {
            await loadStats();
          } catch (err) {
            toastError(err);
          }
          state.loading = false;
        }
        render();
        break;
      case "start-tracker":
        try {
          await act("daemon-start");
          toast(t("settings.daemonStarted"));
          await loadSettings();
          await loadEverything(false);
          render();
        } catch (err) {
          toastError(err);
        }
        break;
      case "stop-tracker":
        try {
          await act("daemon-stop");
          toast(t("settings.daemonStopped"));
          await loadSettings();
          updateChrome();
          render();
        } catch (err) {
          toastError(err);
        }
        break;
      case "export":
        try {
          const res = await act("export", { format: node.dataset.format || "csv" });
          toast(t("settings.exported", { path: res.path || "" }));
        } catch (err) {
          toastError(err);
        }
        break;
      case "open-folder":
        try {
          await act("open-data-folder");
          toast(t("settings.opened"));
        } catch (err) {
          toastError(err);
        }
        break;
      case "open-archive":
        try {
          await act("open-archive");
          toast(t("settings.opened"));
        } catch (err) {
          toastError(err);
        }
        break;
      case "update":
        try {
          await act("update");
          toast(t("settings.updater"));
        } catch (err) {
          toastError(err);
        }
        break;
      case "wipe-modal":
        openWipeModal();
        break;
      case "shortcuts":
        openShortcuts();
        break;
      case "notices":
        openNoticesModal();
        break;
      case "remove-whitelist":
        try {
          await act("whitelist-remove", { value: node.dataset.value });
          await refreshAfterTrackingChange();
        } catch (err) {
          toastError(err);
        }
        break;
      case "remove-browser":
        try {
          await act("browser-remove", { value: node.dataset.value });
          await loadSettings();
          render();
        } catch (err) {
          toastError(err);
        }
        break;
      default:
        break;
    }
  }

  async function setPaused(paused) {
    if (!state.settings) return;
    const previous = state.settings.paused;
    state.settings.paused = paused;
    updateChrome();
    if (state.view === "dashboard" || state.view === "settings") render();
    try {
      await act(paused ? "pause" : "resume");
      toast(paused ? t("toast.paused") : t("toast.resumed"));
      if (state.view === "dashboard") {
        await loadOverview();
      }
    } catch (err) {
      state.settings.paused = previous;
      updateChrome();
      render();
      toastError(err);
    }
  }

  async function loadOverview() {
    const overview = await api("/api/overview");
    state.overview = overview;
    if (state.view === "dashboard") render();
  }

  async function startPomodoro() {
    const minutes = state.selectedPreset || (state.settings && state.settings.pomodoroMinutes) || 25;
    try {
      await act("pomodoro-start", { minutes: minutes });
      state.pomodoro = { active: true, remainingSeconds: minutes * 60, totalMinutes: minutes, configuredMinutes: minutes };
      state.completionAnnounced = false;
      toast(t("focus.startedToast"));
      render();
    } catch (err) {
      toastError(err);
    }
  }

  async function stopPomodoro() {
    try {
      await act("pomodoro-stop");
      state.pomodoro = { active: false, remainingSeconds: 0, totalMinutes: 0, configuredMinutes: state.settings ? state.settings.pomodoroMinutes : 25 };
      toast(t("focus.stoppedToast"));
      render();
    } catch (err) {
      toastError(err);
    }
  }

  /* ---------- change / submit events ---------- */

  async function onViewChange(event) {
    const input = event.target.closest("[data-setting]");
    if (!input || !state.settings) return;
    const setting = input.dataset.setting;
    try {
      switch (setting) {
        case "autostart": {
          const enabled = input.checked;
          await act(enabled ? "autostart-enable" : "autostart-disable");
          state.settings.autostart = enabled;
          toast(t("settings.saved"));
          break;
        }
        case "pause": {
          await setPaused(input.checked);
          break;
        }
        case "strong-web": {
          await act("strong-web", { enabled: input.checked });
          state.settings.strongWebMode = input.checked;
          toast(t("settings.saved"));
          break;
        }
        case "redact": {
          await act("redact-titles", { redactTitles: input.checked });
          state.settings.redactTitles = input.checked;
          toast(t("settings.saved"));
          break;
        }
        case "retention": {
          await act("retention", { days: parseInt(input.value, 10) });
          state.settings.retentionDays = parseInt(input.value, 10);
          toast(t("settings.saved"));
          break;
        }
        case "idle-threshold": {
          const value = parseInt(input.value, 10) || 0;
          const label = $("#idleValue");
          if (label) label.textContent = t("settings.idleValue", { n: value });
          await saveIdle({ thresholdMinutes: value });
          break;
        }
        case "track-idle": {
          await saveIdle({ trackWhileIdle: input.checked });
          break;
        }
        case "track-playing": {
          await saveIdle({ trackWhilePlaying: input.checked });
          break;
        }
        case "domain-grouping": {
          await act("domain-grouping", { enabled: input.checked });
          state.settings.domainGrouping = input.checked;
          toast(t("settings.saved"));
          await refreshAfterTrackingChange();
          break;
        }
        case "keep-history": {
          const enabled = input.checked;
          await act("keep-history", { enabled: enabled });
          state.settings.keepHistory = enabled;
          toast(t("settings.saved"));
          await refreshAfterTrackingChange();
          break;
        }
        case "close-to-tray": {
          await act("close-to-tray", { enabled: input.checked });
          state.settings.closeToTray = input.checked;
          toast(t("settings.saved"));
          break;
        }
        case "break-enabled": {
          const minutes = state.settings.breakReminder.minutes;
          await act("break-reminder", { enabled: input.checked, minutes: minutes });
          state.settings.breakReminder.enabled = input.checked;
          toast(t("settings.saved"));
          break;
        }
        case "break-minutes": {
          const minutes = Math.max(1, Math.min(600, parseInt(input.value, 10) || 1));
          input.value = minutes;
          await act("break-reminder", { enabled: state.settings.breakReminder.enabled, minutes: minutes });
          state.settings.breakReminder.minutes = minutes;
          toast(t("settings.saved"));
          break;
        }
        case "pomodoro-minutes": {
          const minutes = parseInt(input.value, 10);
          await act("pomodoro-minutes", { minutes: minutes });
          state.settings.pomodoroMinutes = minutes;
          if (!state.pomodoro || !state.pomodoro.active) state.selectedPreset = minutes;
          toast(t("settings.saved"));
          render();
          break;
        }
        default:
          break;
      }
    } catch (err) {
      toastError(err);
      if (input.type === "checkbox") input.checked = !input.checked;
      try {
        await loadSettings();
      } catch (_) {}
      render();
    }
  }

  async function saveIdle(patch) {
    if (!state.settings) return;
    const current = Object.assign({}, state.settings.idle, patch);
    await act("idle", {
      thresholdMinutes: current.thresholdMinutes,
      trackWhileIdle: !!current.trackWhileIdle,
      trackWhilePlaying: !!current.trackWhilePlaying
    });
    state.settings.idle = current;
    toast(t("settings.saved"));
  }

  function onViewInput(event) {
    const input = event.target.closest('[data-setting="idle-threshold"]');
    if (!input) return;
    const label = $("#idleValue");
    if (label) label.textContent = t("settings.idleValue", { n: parseInt(input.value, 10) || 0 });
  }

  async function onViewSubmit(event) {
    const form = event.target.closest("[data-form]");
    if (!form) return;
    event.preventDefault();
    const kind = form.dataset.form;
    if (kind === "custom-range") {
      const from = form.elements.from.value;
      const to = form.elements.to.value;
      if (!from || !to) return;
      if (from > to) return toast(t("stats.invalidRange"), "err");
      state.customFrom = from;
      state.customTo = to;
      state.stats = null;
      state.loading = true;
      render();
      try {
        await loadStats();
      } catch (err) {
        toastError(err);
      }
      state.loading = false;
      render();
      return;
    }
    if (kind === "site-limit") {
      const site = form.elements.site.value.trim();
      const minutes = parseInt(form.elements.minutes.value, 10);
      if (!site) return toast(t("limits.sitePlaceholder"), "err");
      if (!minutes || minutes < 1 || minutes > 1440) return toast(t("limits.minutesInvalid"), "err");
      try {
        await act("site-limit-set", { key: site, minutes: minutes });
        form.reset();
        toast(t("limits.addedToast"));
        await loadLimits();
        render();
      } catch (err) {
        toastError(err);
      }
      return;
    }
    if (kind === "web-allowlist") {
      const value = form.elements.value.value.trim();
      if (!value) return;
      const list = (state.settings.webAllowlist || []).slice();
      if (list.indexOf(value) < 0) list.push(value);
      try {
        await act("web-allowlist", { allowlist: list });
        form.reset();
        await loadSettings();
        render();
        toast(t("settings.saved"));
      } catch (err) {
        toastError(err);
      }
      return;
    }
    const input = form.elements.value;
    const value = input ? input.value.trim() : "";
    if (!value) return;
    try {
      if (kind === "whitelist") await act("whitelist-add", { value: value });
      if (kind === "browser") await act("browser-add", { value: value });
      input.value = "";
      tokenToast();
      await refreshAfterTrackingChange();
    } catch (err) {
      toastError(err);
    }
  }

  function tokenToast() {
    toast(t("settings.saved"));
  }

  function onModalSubmit(event) {
    // Limit and other modal forms manage their own submit handlers.
    if (event.target.closest("#limitForm")) return;
  }

  /* ---------- ticker ---------- */

  function tickPomodoro() {
    setInterval(function () {
      const p = state.pomodoro;
      if (!p || !p.active) return;
      if (p.remainingSeconds > 0) {
        p.remainingSeconds = Math.max(0, p.remainingSeconds - 1);
        updatePomodoroDom();
        if (p.remainingSeconds === 0 && !state.completionAnnounced) {
          state.completionAnnounced = true;
          toast(t("focus.completedToast"));
          setTimeout(async function () {
            try {
              await loadPomodoro();
              render();
            } catch (_) {}
          }, 1600);
        }
      }
    }, 1000);

    setInterval(function () {
      if (document.visibilityState !== "visible") return;
      if (state.view !== "dashboard") return;
      if (modalRoot.firstChild) return;
      loadOverview().catch(function () {});
    }, 20000);

    setInterval(function () {
      if (document.visibilityState !== "visible" || state.view !== "settings") return;
      if (!state.settings || !state.settings.daemonRunning) return;
      if (modalRoot.firstChild) return;
      // Keep the tracker status chip current without disturbing focus.
      api("/api/settings").then(function (settings) {
        state.settings = settings;
        updateChrome();
      }).catch(function () {});
    }, 30000);
  }

  function updatePomodoroDom() {
    const p = state.pomodoro;
    if (!p) return;
    const timer = $("#timerValue");
    if (timer) timer.textContent = p.active ? fmtClock(p.remainingSeconds) : fmtClock((state.selectedPreset || 25) * 60);
    const stateLabel = $("#timerState");
    if (stateLabel && p.active) stateLabel.textContent = p.remainingSeconds > 0 ? t("focus.running") : t("focus.done");
    const ring = $("#ringFg");
    if (ring && p.active) {
      const total = Math.max(1, (p.totalMinutes || 25) * 60);
      const fraction = Math.max(0, Math.min(1, p.remainingSeconds / total));
      const circumference = 2 * Math.PI * 118;
      ring.setAttribute("stroke-dashoffset", (circumference * (1 - fraction)).toFixed(2));
    }
    const chip = $("#pomodoroChip");
    if (chip) {
      chip.classList.toggle("hidden", !p.active);
      if (p.active) $("#pomodoroChipTime").textContent = fmtClock(p.remainingSeconds);
    }
  }

  /* ---------- keyboard + tooltip ---------- */

  function onKeydown(event) {
    if (event.key === "Escape") {
      if (modalRoot.firstChild) {
        closeModal();
        event.preventDefault();
      }
      return;
    }
    const tag = (event.target && event.target.tagName) || "";
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || (event.target && event.target.isContentEditable)) return;

    if ((event.metaKey || event.ctrlKey) && /^[1-5]$/.test(event.key)) {
      event.preventDefault();
      go(VIEWS[parseInt(event.key, 10) - 1]);
      return;
    }
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (event.key === "r" || event.key === "R") {
      refreshAll();
    } else if (event.key === "t" || event.key === "T") {
      toggleTheme();
    } else if (event.key === "l" || event.key === "L") {
      toggleLang();
    } else if (event.key === "?" || event.key === "/") {
      openShortcuts();
    }
  }

  function onPointerTip(event) {
    if (!tooltip) return;
    const node = event.target.closest("[data-tip]");
    if (!node) {
      tooltip.classList.remove("show");
      return;
    }
    tooltip.textContent = node.dataset.tip;
    tooltip.style.left = event.clientX + "px";
    tooltip.style.top = event.clientY - 12 + "px";
    tooltip.classList.add("show");
  }

  /* ---------- go ---------- */

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
