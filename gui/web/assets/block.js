(() => {
  const $ = (s, r = document) => r.querySelector(s);
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
        $("#leaveBtn").textContent = "退出该网页";
        $("#cardCopy").textContent = "这个网站今天已经到上限。你可以退出当前标签页，或者临时放行 10 分钟。";
        $("#auditText").textContent = "每次突破都会记录在本地；退出网页不会获得额外额度。";
      }
      $("#usageText").textContent = fmt(info.usedSeconds);
      $("#limitText").textContent = "上限 " + fmt(info.limitMinutes * 60);
      const over = Math.max(0, info.usedSeconds - info.limitMinutes * 60);
      $("#overText").textContent = over > 0 ? "已超出 " + fmt(over) : "接近上限";
      $("#bypassMinutes").textContent = info.bypassMinutes;
      const pct = Math.min(100, Math.round((info.usedSeconds / Math.max(1, info.limitMinutes * 60)) * 100));
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
      await api(isApp ? "/api/block/close" : "/api/block/leave", { method: "POST", body: "{}" });
      if (!isApp) showToast("正在关闭当前网页…");
    } catch (err) {
      showToast(err.message, true);
      state.busy = false;
    }
  });

  const modal = $("#modal");
  const reason = $("#reason");
  const confirm = $("#confirmBypass");

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
    state.busy = true;
    confirm.disabled = true;
    try {
      await api("/api/block/bypass", { method: "POST", body: JSON.stringify({ reason: text }) });
      modal.hidden = true;
      showToast("已临时放行 " + state.info.bypassMinutes + " 分钟，本次选择已记录。");
    } catch (err) {
      showToast(err.message, true);
      confirm.disabled = false;
      state.busy = false;
    }
  });

  load();
})();
