import {
  createBrowserAdapter,
  createHandoffController,
  DEFAULT_SETTINGS,
  diagnosticFor,
  isDownloadFilename,
  isDownloadMime,
  isDownloadMimeValue,
  isDownloadURL,
  isHTTPURL,
  normalizeSettings,
  normalizeSiteRule,
  shouldAutomaticallyIntercept,
} from "./core.mjs";

const isFirefox = typeof browser !== "undefined";
const ext = isFirefox ? browser : chrome;
const adapter = createBrowserAdapter(ext, isFirefox);
const handoff = createHandoffController(adapter);
let settings = normalizeSettings(DEFAULT_SETTINGS);
let connection = { success: false, outcome: "app_unavailable", message: "Checking Lunefetch connection" };
let failures = [];

function storageGet(key) {
  return browserCall(ext.storage.local.get.bind(ext.storage.local), key).then((result) => result[key]);
}

function storageSet(values) {
  return browserCall(ext.storage.local.set.bind(ext.storage.local), values);
}

function browserCall(fn, ...args) {
  if (isFirefox) return Promise.resolve(fn(...args));
  return new Promise((resolve, reject) => fn(...args, (value) => {
    const error = ext.runtime.lastError;
    if (error) reject(new Error(error.message));
    else resolve(value);
  }));
}

async function saveSettings(next) {
  settings = normalizeSettings(next);
  await storageSet({ settings });
  await refreshUI();
  return settings;
}

async function loadSettings() {
  settings = normalizeSettings(await storageGet("settings"));
  const storedFailures = await storageGet("handoffFailures");
  failures = Array.isArray(storedFailures) ? storedFailures : [];
  await storageSet({ settings, handoffFailures: failures.slice(0, 10) });
}

async function rememberFailure(result, item) {
  if (result.success || !isHTTPURL(item.url)) return;
  failures = [{
    id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
    url: item.url,
    filename: item.filename || "",
    saveDir: item.saveDir || "",
    outcome: result.outcome,
    message: diagnosticFor(result),
    timestamp: Date.now(),
  }, ...failures].slice(0, 10);
  await storageSet({ handoffFailures: failures });
}

function report(result, url = "") {
  const message = diagnosticFor(result);
  if (result.success) console.info("Lunefetch:", message, url);
  else console.warn("Lunefetch:", message, result.message || "", url);
  return result;
}

async function notify(result, url = "") {
  if (!settings.notifications || !ext.notifications?.create) return;
  const title = result.success ? "Sent to Lunefetch" : "Lunefetch kept browser download";
  const message = url ? `${diagnosticFor(result)}\n${url}` : diagnosticFor(result);
  await ext.notifications.create({ type: "basic", iconUrl: "icons/icon-128.png", title, message });
}

async function showFeedback(result) {
  if (!ext.action?.setBadgeText) return;
  await ext.action.setBadgeText({ text: result.success ? "OK" : "!" });
  if (ext.action.setBadgeBackgroundColor) {
    await ext.action.setBadgeBackgroundColor({ color: result.success ? "#0A7D32" : "#B42318" });
  }
  setTimeout(() => refreshUI().catch(() => {}), 2500);
}

async function refreshUI() {
  const connected = connection.success;
  const badge = !settings.enabled ? "OFF" : connected ? "" : "!";
  if (ext.action?.setBadgeText) await ext.action.setBadgeText({ text: badge });
  if (ext.action?.setBadgeBackgroundColor) {
    await ext.action.setBadgeBackgroundColor({ color: !settings.enabled ? "#555555" : connected ? "#0A7D32" : "#B42318" });
  }
  if (ext.action?.setTitle) {
    await ext.action.setTitle({ title: settings.enabled ? diagnosticFor(connection) : "Lunefetch interception is off" });
  }
  if (ext.contextMenus?.update) {
    await ext.contextMenus.update("lunefetch-download", { visible: settings.enabled && settings.contextMenu });
    await ext.contextMenus.update("lunefetch-download-all", { visible: settings.enabled && settings.contextMenu });
  }
}

async function updateConnectionStatus() {
  connection = report(await handoff.health());
  await refreshUI();
  return connection;
}

function mayIntercept(url) {
  return shouldAutomaticallyIntercept(url, settings, connection.success);
}

async function sendWithFeedback(url, hints = {}, recordFailure = true) {
  const result = report(await handoff.send(url, hints), url);
  if (recordFailure) await rememberFailure(result, { url, filename: hints.filename, saveDir: hints.saveDir });
  await showFeedback(result);
  await notify(result, url);
  return result;
}

if (isFirefox && ext.webRequest) {
  ext.webRequest.onHeadersReceived.addListener(
    async (details) => {
      if (details.method !== "GET" || !isHTTPURL(details.url)) return {};
      const headers = details.responseHeaders || [];
      const disposition = (headers.find((header) => header.name.toLowerCase() === "content-disposition")?.value || "").toLowerCase();
      const capture = disposition.includes("attachment") || isDownloadMime(headers, settings.mimeTypes) || isDownloadURL(details.url, settings.extensions);
      if (!capture) return {};
      if (!mayIntercept(details.url)) return {};
      const result = report(await handoff.interceptFirefox(details.url), details.url);
      await rememberFailure(result, { url: details.url });
      await showFeedback(result);
      await notify(result, details.url);
      return result.success ? { cancel: true } : {};
    },
    { urls: ["<all_urls>"] },
    ["blocking", "responseHeaders"],
  );
}

ext.downloads.onCreated.addListener(async (item) => {
  if (!mayIntercept(item.url)) return;
  const capture = isDownloadURL(item.url, settings.extensions)
    || isDownloadFilename(item.filename, settings.extensions)
    || isDownloadMimeValue(item.mime, settings.mimeTypes);
  if (!capture) return;
  const outcome = await handoff.handleCreated(item);
  if (outcome.result) {
    report(outcome.result, item.url);
    await rememberFailure(outcome.result, { url: item.url, filename: item.filename });
    await showFeedback(outcome.result);
    await notify(outcome.result, item.url);
  }
  if (outcome.error) console.error("Lunefetch: browser fallback failed:", outcome.error.message, item.url);
});

ext.contextMenus.create({
  id: "lunefetch-download",
  title: "Download with Lunefetch",
  contexts: ["link", "image", "video", "audio"],
});
ext.contextMenus.create({
  id: "lunefetch-download-all",
  title: "Download all with Lunefetch",
  contexts: ["page"],
});

ext.contextMenus.onClicked.addListener(async (info, tab) => {
  if (!settings.enabled || !settings.contextMenu) return;
  if (info.menuItemId === "lunefetch-download-all") {
    const results = await browserCall(ext.scripting.executeScript.bind(ext.scripting), {
      target: { tabId: tab.id },
      func: () => [...new Set([...document.querySelectorAll("a[href]")].map((link) => link.href)
        .filter((url) => url.startsWith("http://") || url.startsWith("https://")))].slice(0, 100),
    });
    const urls = results?.[0]?.result || [];
    await storageSet({ batchDraft: { source: tab.url || "", urls, createdAt: Date.now() } });
    await browserCall(ext.tabs.create.bind(ext.tabs), { url: ext.runtime.getURL("batch.html") });
    return;
  }
  if (info.menuItemId !== "lunefetch-download") return;
  const url = info.linkUrl || info.srcUrl;
  if (!isHTTPURL(url)) {
    console.warn("Lunefetch:", diagnosticFor({ outcome: "invalid_url" }));
    return;
  }
  await sendWithFeedback(url);
});

async function handleMessage(message) {
  if (message?.type === "get-state") return { settings, connection, diagnostic: diagnosticFor(connection), failures };
  if (message?.type === "refresh-status") {
    await updateConnectionStatus();
    return { settings, connection, diagnostic: diagnosticFor(connection) };
  }
  if (message?.type === "update-settings") return saveSettings({ ...settings, ...message.settings });
  if (message?.type === "reset-settings") return saveSettings(DEFAULT_SETTINGS);
  if (message?.type === "bypass-site") {
    const host = normalizeSiteRule(message.host);
    if (!host) return Promise.reject(new Error("Invalid site"));
    return saveSettings({ ...settings, bypassUntil: { ...settings.bypassUntil, [host]: Date.now() + 60 * 60 * 1000 } });
  }
  if (message?.type === "send-download") return sendWithFeedback(message.item?.url, message.item || {});
  if (message?.type === "send-batch") {
    const items = Array.isArray(message.items) ? message.items.slice(0, 100) : [];
    const results = [];
    for (const item of items) results.push(await sendWithFeedback(item.url, item));
    return results;
  }
  if (message?.type === "retry-failure") {
    const failed = failures.find((item) => item.id === message.id);
    if (!failed) return Promise.reject(new Error("Failure entry no longer exists"));
    const result = await sendWithFeedback(failed.url, failed, false);
    if (result.success) {
      failures = failures.filter((item) => item.id !== failed.id);
      await storageSet({ handoffFailures: failures });
    } else {
      failures = failures.map((item) => item.id === failed.id
        ? { ...item, outcome: result.outcome, message: diagnosticFor(result), timestamp: Date.now() }
        : item);
      await storageSet({ handoffFailures: failures });
    }
    return result;
  }
  if (message?.type === "clear-failures") {
    failures = [];
    await storageSet({ handoffFailures: failures });
    return true;
  }
  return undefined;
}

ext.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (isFirefox) return handleMessage(message);
  handleMessage(message).then(sendResponse, (error) => sendResponse({ error: error.message }));
  return true;
});

async function start() {
  await loadSettings();
  await updateConnectionStatus();
}

export const startup = start().catch((error) => {
  console.warn("Lunefetch: initialization failed:", error.message);
  throw error;
});
