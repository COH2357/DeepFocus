const HOST = "com.deepfocus.host";
const BLOCK_PAGE = chrome.runtime.getURL("block.html");

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

function isInternal(url) {
  return (
    url.startsWith("chrome-extension://") ||
    url.startsWith("edge-extension://") ||
    url.startsWith("about:") ||
    url.startsWith("chrome:") ||
    url.startsWith("edge:") ||
    url.startsWith("devtools:") ||
    url.startsWith("view-source:")
  );
}

async function heartbeat() {
  await nativeMessage({ type: "heartbeat", browser, version: "0.1.0" });
}

async function guardNavigation(details) {
  if (details.frameId !== 0) return;
  const url = details.url || "";
  if (!/^https?:/i.test(url)) return;
  if (details.tabId < 0) return;

  const reply = await nativeMessage({
    type: "navigation",
    browser,
    version: "0.1.0",
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

  const block = new URL(BLOCK_PAGE);
  block.searchParams.set("site", reply.site || "");
  block.searchParams.set("label", reply.label || reply.site || "");
  block.searchParams.set("reason", reply.reason || "strong");
  block.searchParams.set("used", String(reply.usedSeconds || 0));
  block.searchParams.set("limit", String(reply.limitMinutes || 0));
  block.searchParams.set("url", url);
  chrome.tabs.update(details.tabId, { url: block.toString() }).catch(() => {});
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
    version: "0.1.0",
    site: message.site || "",
    url: message.url || "",
    minutes: Number(message.minutes) || 0,
    reason: message.reason || ""
  }).then((reply) => sendResponse(reply || { ok: false, error: "DeepFocus 服务未运行" }));
  return true;
});

chrome.runtime.onInstalled.addListener(() => {
  chrome.alarms.create("deepfocus-heartbeat", { periodInMinutes: 1 });
  heartbeat();
});

chrome.runtime.onStartup.addListener(() => {
  chrome.alarms.create("deepfocus-heartbeat", { periodInMinutes: 1 });
  heartbeat();
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === "deepfocus-heartbeat") heartbeat();
});

heartbeat();
