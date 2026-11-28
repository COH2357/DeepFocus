(() => {
  const params = new URLSearchParams(location.search);
  const site = params.get("site") || "";
  const label = params.get("label") || site || "该网站";
  const reason = params.get("reason") || "strong";
  const used = Number(params.get("used") || 0);
  const limit = Number(params.get("limit") || 0);
  const remaining = Number(params.get("remaining") || 0);
  const original = params.get("url") || "";
  const unavailable = params.get("unavailable") === "1";
  const minutes = [5, 15, 30, 60];

  const $ = (id) => document.getElementById(id);
  const fmt = (s) => {
    const m = Math.max(0, Math.round(Number(s || 0) / 60));
    return m >= 60 ? Math.floor(m / 60) + " 小时 " + (m % 60) + " 分" : m + " 分钟";
  };

  $("siteLabel").textContent = label;
  $("used").textContent = limit > 0 ? fmt(used) : "—";
  $("limit").textContent = limit > 0 ? "上限 " + fmt(limit * 60) : "强力监管";
  $("mode").textContent = reason === "strong" && remaining <= 0 ? "已超出今日额度" : (reason === "limit" ? "已超出今日额度" : "需要说明理由");
  const pct = limit > 0 ? Math.min(100, Math.round((used / (limit * 60)) * 100)) : 100;
  const bar = $("bar");
  bar.style.width = Math.max(4, pct) + "%";
  bar.classList.toggle("over", pct >= 100);

  if (unavailable) {
    document.body.classList.add("unavailable");
    $("lede").textContent = "DeepFocus 本地服务没有响应。强力监管模式需要主程序运行；当前页面已暂停加载。";
    $("kicker").textContent = "服务未运行";
    $("quote").textContent = "先让 DeepFocus 恢复运行，再决定要不要继续。";
    const toast = $("toast");
    toast.hidden = false;
    toast.className = "toast err";
    toast.textContent = "无法连接 DeepFocus 本地服务";
    return;
  }

  if (reason === "limit") {
    $("lede").textContent = "这个网站今天的额度已经用完。说明用途后可以临时放行，记录会保存在本地。";
    $("kicker").textContent = "网站额度已到";
  } else if (reason === "strong") {
    if (remaining > 0) {
      $("lede").textContent = "强力监管要求进入这个网站前说明用途并选择放行时长；本次仍可使用今日剩余的正常额度。";
      $("kicker").textContent = "访问前确认";
    } else {
      $("lede").textContent = "这个网站今天的额度已经用完。强力监管要求说明用途并选择放行时长。";
      $("kicker").textContent = "额度已用完";
    }
  }

  const quoteEl = $("quote");
  const rotateQuotes = (list) => {
    if (!Array.isArray(list) || list.length < 2) return;
    let index = Math.floor(Math.random() * list.length);
    const show = () => {
      quoteEl.textContent = list[index % list.length];
      index++;
    };
    show();
    setInterval(show, 7000);
  };
  chrome.runtime.sendMessage({ type: "get_quotes" }, (reply) => {
    if (!chrome.runtime.lastError && reply && reply.ok && Array.isArray(reply.quotes)) rotateQuotes(reply.quotes);
  });

  let selected = 15;
  const reasonEl = $("reason");
  const grant = $("grant");
  const custom = $("customMinutes");
  const updateRemaining = () => {
    const el = $("remaining");
    const maxNormal = Math.floor(remaining / 60);
    if (maxNormal <= 0) {
      el.textContent = "今日正常额度已用完；本次放行将记为一次违反。";
      el.classList.add("over");
    } else if (selected > maxNormal) {
      el.textContent = "本次最多可正常放行 " + maxNormal + " 分钟；当前选择超出额度，放行将记为一次违反。";
      el.classList.add("over");
    } else {
      el.textContent = "今日最多还可正常放行 " + maxNormal + " 分钟；超出部分记为一次违反。";
      el.classList.remove("over");
    }
  };
  const refresh = () => {
    grant.disabled = reasonEl.value.trim().length < 2;
  };
  reasonEl.addEventListener("input", refresh);

  document.querySelectorAll("[data-minutes]").forEach((button) => {
    button.addEventListener("click", () => {
      selected = Number(button.dataset.minutes) || 15;
      custom.value = "";
      document.querySelectorAll("[data-minutes]").forEach((b) => b.classList.toggle("active", b === button));
      updateRemaining();
    });
  });
  custom.addEventListener("input", () => {
    const minutes = parseInt(custom.value, 10);
    if (minutes >= 1 && minutes <= 1440) {
      selected = minutes;
      document.querySelectorAll("[data-minutes]").forEach((b) => b.classList.remove("active"));
      updateRemaining();
    }
  });
  refresh();
  updateRemaining();

  const toast = (text, error) => {
    const el = $("toast");
    el.textContent = text;
    el.className = "toast" + (error ? " err" : "");
    el.hidden = false;
    clearTimeout(toast.t);
    toast.t = setTimeout(() => { el.hidden = true; }, 3600);
  };

  grant.addEventListener("click", () => {
    const text = reasonEl.value.trim();
    if (text.length < 2) return;
    if (selected < 1 || selected > 1440) {
      toast("放行时长需要在 1–1440 分钟之间", true);
      return;
    }
    grant.disabled = true;
    grant.textContent = "正在记录…";
    chrome.runtime.sendMessage({
      type: "grant_access",
      site,
      url: original,
      minutes: selected,
      reason: text
    }, (reply) => {
      if (chrome.runtime.lastError || !reply || !reply.ok) {
        toast((reply && reply.error) || "放行失败，DeepFocus 服务未响应", true);
        grant.disabled = false;
        grant.textContent = "确认放行";
        return;
      }
      toast("已放行 " + selected + " 分钟，正在返回…");
      if (original) {
        setTimeout(() => { location.replace(original); }, 250);
      } else {
        history.back();
      }
    });
  });

  $("back").addEventListener("click", () => {
    if (history.length > 1) history.back();
    else if (chrome.tabs && chrome.tabs.getCurrent) {
      chrome.tabs.getCurrent((tab) => { if (tab && tab.id) chrome.tabs.remove(tab.id); });
    }
  });
})();
