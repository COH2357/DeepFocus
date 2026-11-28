const HOST = "com.deepfocus.host";
const BLOCK_PAGE = chrome.runtime.getURL("block.html");
const ALLOWANCE_KEY = "deepfocus_allowances";
const ALLOWANCE_POLL = "deepfocus-allowance-poll";
const ALLOWANCE_EXPIRY = "deepfocus-allowance-expiry";

const browser = (() => {
  const ua = navigator.userAgent || "";
  if (/Edg\//.test(ua)) return "edge";
  if (/Firefox\//.test(ua)) return "firefox";
  if (/Brave\//.test(ua)) return "brave";
  return "chrome";
})();

function nativeMessage(message) {
  return new Promise((resolve) => {
    chrome.runtime.sendNativeMessage(HOST, message, (reply) => {
      if (chrome.runtime.lastError) {
        resolve(null);
        return;
      }
      resolve(reply || null);
    });
  });
}

function blockURL(reply, url) {
  const block = new URL(BLOCK_PAGE);
  block.searchParams.set("site", reply.site || "");
  block.searchParams.set("label", reply.label || reply.site || "");
  block.searchParams.set("reason", reply.reason || "strong");
  block.searchParams.set("used", String(reply.usedSeconds || 0));
  block.searchParams.set("limit", String(reply.limitMinutes || 0));
  block.searchParams.set("remaining", String(reply.remainingSeconds || 0));
  block.searchParams.set("url", url);
  return block.toString();
}

function redirectToBlock(tabId, reply, url) {
  chrome.tabs.update(tabId, { url: blockURL(reply, url) }).catch(() => {});
}

async function loadAllowances() {
  const data = await chrome.storage.local.get(ALLOWANCE_KEY);
  return data[ALLOWANCE_KEY] || {};
}

async function saveAllowances(allowances) {
  await chrome.storage.local.set({ [ALLOWANCE_KEY]: allowances });
}

async function rememberAllowance(site, until, minutes, withinLimit) {
  if (!site || !until) return;
  const allowances = await loadAllowances();
  allowances[site] = { until: Number(until), minutes: Number(minutes) || 0, withinLimit: !!withinLimit };
  await saveAllowances(allowances);
  await scheduleAllowanceCheck();
}

async function scheduleAllowanceCheck() {
  const allowances = await loadAllowances();
  const now = Math.floor(Date.now() / 1000);
  const future = Object.values(allowances).filter((entry) => Number(entry.until) > now);
  if (!future.length) {
    await chrome.alarms.clear(ALLOWANCE_EXPIRY);
    await chrome.alarms.clear(ALLOWANCE_POLL);
    return;
  }
  const next = Math.min(...future.map((entry) => Number(entry.until)));
  chrome.alarms.create(ALLOWANCE_EXPIRY, { when: Math.max(Date.now() + 1000, next * 1000 + 500) });
  // A periodic sweep also catches a disabled laptop waking after the deadline.
  chrome.alarms.create(ALLOWANCE_POLL, { periodInMinutes: 0.5 });
}

async function checkAllowances() {
  const allowances = await loadAllowances();
  const now = Math.floor(Date.now() / 1000);
  if (!Object.keys(allowances).length) return;
  const remaining = {};
  for (const [site, entry] of Object.entries(allowances)) {
    if (Number(entry.until) > now) {
      remaining[site] = entry;
      continue;
    }
    await nativeMessage({
      type: "allowance_ended",
      browser,
      version: "0.1.2",
      site,
      minutes: Number(entry.minutes) || 0,
      withinLimit: !!entry.withinLimit
    });
  }
  await saveAllowances(remaining);
  const tabs = await chrome.tabs.query({ url: ["http://*/*", "https://*/*"] });
  for (const tab of tabs) {
    if (!tab.id || !tab.url) continue;
    const reply = await nativeMessage({
      type: "navigation",
      browser,
      version: "0.1.2",
      url: tab.url,
      tabId: tab.id
    });
    if (reply && reply.action === "block") {
      redirectToBlock(tab.id, reply, tab.url);
    }
  }
  await scheduleAllowanceCheck();
}

async function heartbeat() {
  await nativeMessage({ type: "heartbeat", browser, version: "0.1.2" });
}

async function guardNavigation(details) {
  if (details.frameId !== 0) return;
  const url = details.url || "";
  if (!/^https?:/i.test(url)) return;
  if (details.tabId < 0) return;

  const reply = await nativeMessage({
    type: "navigation",
    browser,
    version: "0.1.2",
    url,
    tabId: details.tabId
  });
  if (!reply) {
    const failed = new URL(BLOCK_PAGE);
    failed.searchParams.set("unavailable", "1");
    failed.searchParams.set("url", url);
    chrome.tabs.update(details.tabId, { url: failed.toString() }).catch(() => {});
    return;
  }
  if (reply.action !== "block") return;
  redirectToBlock(details.tabId, reply, url);
}

chrome.webNavigation.onBeforeNavigate.addListener((details) => {
  guardNavigation(details).catch(() => {});
});

chrome.webNavigation.onCommitted.addListener((details) => {
  if (details.frameId === 0 && /^https?:/i.test(details.url || "")) heartbeat();
});

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!message || message.type !== "grant_access") return false;
  nativeMessage({
    type: "request_access",
    browser,
    version: "0.1.2",
    site: message.site || "",
    url: message.url || "",
    minutes: Number(message.minutes) || 0,
    reason: message.reason || ""
  }).then((reply) => {
    if (reply && reply.ok && reply.until) rememberAllowance(reply.site, reply.until, message.minutes, reply.withinLimit).catch(() => {});
    sendResponse(reply || { ok: false, error: "DeepFocus 服务未运行" });
  });
  return true;
});

chrome.runtime.onInstalled.addListener(() => {
  chrome.alarms.create("deepfocus-heartbeat", { periodInMinutes: 1 });
  heartbeat();
  scheduleAllowanceCheck().catch(() => {});
});

chrome.runtime.onStartup.addListener(() => {
  chrome.alarms.create("deepfocus-heartbeat", { periodInMinutes: 1 });
  heartbeat();
  scheduleAllowanceCheck().catch(() => {});
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === "deepfocus-heartbeat") heartbeat();
  if (alarm.name === ALLOWANCE_POLL || alarm.name === ALLOWANCE_EXPIRY) {
    checkAllowances().catch(() => {});
  }
});

heartbeat();
scheduleAllowanceCheck().catch(() => {});
