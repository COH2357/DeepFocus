(() => {
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const token = $('meta[name="focusd-token"]').content;
  const state = { info: null, quoteIndex: 0, timer: null, busy: false };

  const api = async (path, opts = {}) => {
    const res = await fetch(path, {
      ...opts,
      headers: { "Content-Type": "application/json", "X-Focusd-Token": token, ...(opts.headers || {}) }
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || "请求失败");
    return data;
  };

  const fmt = (seconds) => {
    const s = Math.max(0, Math.round(seconds || 0));
    const h = Math.floor(s / 3600);
    const m = Math.round((s % 3600) / 60);
    if (h > 0) return h + " 小时 " + String(m).padStart(2, "0") + " 分";
    return Math.max(1, m) + " 分钟";
  };

  const showToast = (msg, err = false) => {
    const el = $("#toast");
    el.textContent = msg;
    el.classList.toggle("err", err);
    el.hidden = false;
    clearTimeout(showToast.t);
    showToast.t = setTimeout(() => { el.hidden = true; }, 3200);
  };

  const renderQuote = () => {
    const list = (state.info && state.info.quotes) || [];
    if (!list.length) return;
    state.quoteIndex %= list.length;
    $("#quote").textContent = list[state.quoteIndex];
    $("#quoteIndex").textContent = "语录 " + (state.quoteIndex + 1) + " / " + list.length;
  };

  const nextQuote = () => {
    const list = (state.info && state.info.quotes) || [];
    if (!list.length) return;
    state.quoteIndex = (state.quoteIndex + 1) % list.length;
    renderQuote();
  };

  async function load() {
    try {
      const info = await api("/api/block/info");
      state.info = info;
      const kind = info.kind === "site" ? "网站限制" : "应用限制";
      $("#kindLabel").textContent = kind;
      $("#targetLabel").textContent = info.label || info.target;
      const compact = info.mode === "app";
      document.body.classList.toggle("compact", compact);
      if (compact) {
        $("#cardCopy").textContent = "应用仍在运行。为了不丢失进度，DeepFocus 不会替你关闭它。";
        $("#auditText").textContent = "关闭应用后，这里的按钮才会变为可用。";
        $("#leaveBtn").textContent = "已经关闭 " + (info.label || info.target);
        $("#leaveBtn").disabled = !!info.processRunning;
      } else {
        $("#leaveBtn").hidden = false;
        $("#leaveBtn").disabled = false;
        $("#leaveBtn").textContent = "关闭该网站全部标签页";
        $("#cardCopy").textContent = "这个网站今天已经到上限。关闭全部该网站标签页，或者临时放行 10 分钟。";
        $("#auditText").textContent = "只会关闭该网站的标签页，其它标签页保留；每次突破都会记录在本地。";
      }
      const isOpens = info.mode === "app" && info.unit === "opens";
      if (isOpens) {
        $("#usageLabel").textContent = "今天已经打开";
        $("#usageText").textContent = (info.usedSeconds || 0) + " 次";
        $("#limitText").textContent = "每日上限 " + (info.maxOpens || info.limitMinutes || 0) + " 次";
        $("#overText").textContent = (info.usedSeconds || 0) >= (info.maxOpens || info.limitMinutes || 0) ? "今日次数已用完" : "还可打开";
        $("#cardCopy").textContent = "这个应用今天的打开次数已经用完。关闭提醒不会重置次数；下次启动会再次被拦下。";
        $("#auditText").textContent = "只有所有进程退出后再次启动才计为一次；最小化到托盘不算退出。";
      } else {
        $("#usageText").textContent = fmt(info.usedSeconds);
        $("#limitText").textContent = "上限 " + fmt(info.limitMinutes * 60);
        const over = Math.max(0, info.usedSeconds - info.limitMinutes * 60);
        $("#overText").textContent = over > 0 ? "已超出 " + fmt(over) : "接近上限";
      }
      $("#bypassMinutes").textContent = info.bypassMinutes;
      let pct;
      if (isOpens) {
        pct = Math.min(100, Math.round(((info.usedSeconds || 0) / Math.max(1, info.maxOpens || info.limitMinutes || 1)) * 100));
      } else {
        pct = Math.min(100, Math.round((info.usedSeconds / Math.max(1, info.limitMinutes * 60)) * 100));
      }
      const bar = $("#usageBar");
      bar.style.width = pct + "%";
      bar.classList.toggle("over", pct >= 100);

      // pick a random starting quote so repeated blocks do not always open on
      // the same line.
      const list = info.quotes || [];
      if (list.length) {
        state.quoteIndex = Math.floor(Math.random() * list.length);
        renderQuote();
      }
      if (info.mode === "app") startProcessPolling();
      // Rotate the quote on its own; the button is only a manual shortcut.
      clearInterval(state.timer);
      if (list.length > 1) state.timer = setInterval(nextQuote, 7000);
      window.scrollTo(0, 0);
    } catch (err) {
      showToast(err.message, true);
    }
  }


  let processTimer = null;
  async function refreshProcess() {
    if (!state.info || state.info.mode !== "app") return;
    try {
      const info = await api("/api/block/info");
      state.info.processRunning = info.processRunning;
      const btn = $("#leaveBtn");
      btn.disabled = !!info.processRunning;
      btn.textContent = info.processRunning
        ? "已经关闭 " + (state.info.label || state.info.target)
        : "关闭提醒，返回工作";
    } catch (_) {}
  }
  function startProcessPolling() {
    clearInterval(processTimer);
    processTimer = setInterval(refreshProcess, 2000);
  }

  $("#nextQuote").addEventListener("click", nextQuote);

  const dragHandle = $("#dragHandle");
  if (dragHandle) {
    dragHandle.addEventListener("pointerdown", async (event) => {
      event.preventDefault();
      try {
        await api("/api/block/dragstart", { method: "POST", body: "{}" });
      } catch (_) {}
    });
  }

  $("#leaveBtn").addEventListener("click", async () => {
    if (state.busy) return;
    const isApp = state.info && state.info.mode === "app";
    if (isApp && state.info.processRunning) return;
    state.busy = true;
    try {
      if (!isApp) showToast("正在关闭该网站的全部标签页…");
      const data = await api(isApp ? "/api/block/close" : "/api/block/leave", { method: "POST", body: "{}" });
      if (!isApp) {
        const count = Number(data.closed || 0);
        showToast(count > 0 ? "已关闭 " + count + " 个该网站标签页。" : "没有找到可关闭的该网站标签页。");
      }
    } catch (err) {
      showToast(err.message, true);
      state.busy = false;
    }
  });

  const modal = $("#modal");
  const reason = $("#reason");
  const confirm = $("#confirmBypass");
  let bypassMinutes = 10;
  const customBypass = $("#customBypassMinutes");
  $$("[data-bypass-minutes]", modal).forEach(function (button) {
    button.addEventListener("click", function () {
      bypassMinutes = parseInt(button.dataset.bypassMinutes, 10) || 10;
      customBypass.value = "";
      $$("[data-bypass-minutes]", modal).forEach(function (b) { b.classList.toggle("active", b === button); });
    });
  });
  customBypass.addEventListener("input", function () {
    const value = parseInt(customBypass.value, 10);
    if (value >= 1 && value <= 1440) {
      bypassMinutes = value;
      $$("[data-bypass-minutes]", modal).forEach(function (b) { b.classList.remove("active"); });
    }
  });

  const openBypass = () => {
    modal.hidden = false;
    reason.value = "";
    confirm.disabled = true;
    $("#reasonHint").textContent = "至少写几个字，便于以后回看。";
    setTimeout(() => reason.focus(), 40);
  };
  const closeBypass = () => { modal.hidden = true; };

  $("#bypassBtn").addEventListener("click", openBypass);
  $("#cancelBypass").addEventListener("click", closeBypass);
  modal.addEventListener("click", (e) => { if (e.target === modal) closeBypass(); });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && !modal.hidden) closeBypass();
  });

  reason.addEventListener("input", () => {
    const ok = reason.value.trim().length >= 2;
    confirm.disabled = !ok;
    $("#reasonHint").textContent = ok ? "理由只会保存在本地。" : "至少写几个字，便于以后回看。";
  });

  confirm.addEventListener("click", async () => {
    const text = reason.value.trim();
    if (text.length < 2 || state.busy) return;
    if (bypassMinutes < 1 || bypassMinutes > 1440) return showToast("放行时长需要在 1–1440 分钟之间", true);
    state.busy = true;
    confirm.disabled = true;
    try {
      await api("/api/block/bypass", { method: "POST", body: JSON.stringify({ reason: text, minutes: bypassMinutes }) });
      modal.hidden = true;
      showToast("已临时放行 " + bypassMinutes + " 分钟，本次选择已记录。");
    } catch (err) {
      showToast(err.message, true);
      confirm.disabled = false;
      state.busy = false;
    }
  });

  load();
})();
