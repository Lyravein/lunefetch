export const NATIVE_HOST = "com.lyravein.lunefetch";

export const OUTCOMES = Object.freeze({
  ACCEPTED: "accepted",
  APP_UNAVAILABLE: "app_unavailable",
  INVALID_URL: "invalid_url",
  UNAUTHORIZED: "unauthorized",
  QUEUE_FULL: "queue_full",
  INTERNAL_ERROR: "internal_error",
});

export const DOWNLOAD_EXTENSIONS = new Set([
  "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "zst", "iso", "img", "dmg",
  "exe", "msi", "deb", "rpm", "pkg", "apk", "mp4", "mkv", "avi", "mov",
  "wmv", "flv", "webm", "mp3", "flac", "wav", "aac", "ogg", "pdf", "doc",
  "docx", "xls", "xlsx", "ppt", "pptx",
]);

export const DOWNLOAD_MIME_TYPES = new Set([
  "application/zip", "application/x-rar-compressed", "application/x-7z-compressed",
  "application/x-tar", "application/gzip", "application/x-bzip2", "application/x-xz",
  "application/x-iso9660-image", "application/octet-stream", "application/x-msdownload",
  "application/vnd.debian.binary-package", "application/x-rpm",
  "application/vnd.android.package-archive", "video/mp4", "video/x-matroska", "video/avi",
  "video/quicktime", "video/webm", "audio/mpeg", "audio/flac", "audio/wav", "audio/aac",
  "audio/ogg", "application/pdf",
]);

export const DEFAULT_SETTINGS = Object.freeze({
  version: 1,
  enabled: true,
  automaticInterception: true,
  contextMenu: true,
  browserFallback: true,
  notifications: true,
  extensions: [...DOWNLOAD_EXTENSIONS],
  mimeTypes: [...DOWNLOAD_MIME_TYPES],
  allowSites: [],
  blockSites: [],
  bypassUntil: {},
});

function cleanList(values, normalize) {
  if (!Array.isArray(values)) return [];
  return [...new Set(values.map(normalize).filter(Boolean))];
}

function normalizeExtension(value) {
  const extension = String(value).trim().toLowerCase().replace(/^\.+/, "");
  return /^[a-z0-9][a-z0-9+_-]{0,31}$/.test(extension) ? extension : "";
}

function normalizeMime(value) {
  const mime = String(value).trim().toLowerCase();
  return /^[\w!#$&^_.+-]+\/[\w!#$&^_.+-]+$/.test(mime) ? mime : "";
}

export function normalizeSiteRule(value) {
  let host = String(value).trim().toLowerCase();
  if (!host) return "";
  try {
    if (host.includes("://")) host = new URL(host).hostname.toLowerCase();
  } catch {
    return "";
  }
  host = host.replace(/^\*\./, "").replace(/^\.+|\.+$/g, "");
  return /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)*[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(host) ? host : "";
}

export function normalizeSettings(value = {}) {
  if (!value || typeof value !== "object") value = {};
  const bypassUntil = {};
  if (value.bypassUntil && typeof value.bypassUntil === "object") {
    for (const [rawHost, rawExpiry] of Object.entries(value.bypassUntil)) {
      const host = normalizeSiteRule(rawHost);
      const expiry = Number(rawExpiry);
      if (host && Number.isFinite(expiry) && expiry > 0) bypassUntil[host] = expiry;
    }
  }
  return {
    version: DEFAULT_SETTINGS.version,
    enabled: value.enabled !== false,
    automaticInterception: value.automaticInterception !== false,
    contextMenu: value.contextMenu !== false,
    browserFallback: value.browserFallback !== false,
    notifications: value.notifications !== false,
    extensions: value.extensions === undefined
      ? [...DEFAULT_SETTINGS.extensions]
      : cleanList(value.extensions, normalizeExtension),
    mimeTypes: value.mimeTypes === undefined
      ? [...DEFAULT_SETTINGS.mimeTypes]
      : cleanList(value.mimeTypes, normalizeMime),
    allowSites: cleanList(value.allowSites, normalizeSiteRule),
    blockSites: cleanList(value.blockSites, normalizeSiteRule),
    bypassUntil,
  };
}

export function siteRuleMatches(hostname, rule) {
  const host = String(hostname).toLowerCase();
  return host === rule || host.endsWith(`.${rule}`);
}

export function siteDecision(rawURL, settings, now = Date.now()) {
  if (!isHTTPURL(rawURL)) return { allowed: false, reason: "invalid_url" };
  const hostname = new URL(rawURL).hostname.toLowerCase();
  if (Object.entries(settings.bypassUntil || {}).some(([rule, expiry]) => expiry > now && siteRuleMatches(hostname, rule))) {
    return { allowed: false, reason: "temporary_bypass", hostname };
  }
  if ((settings.blockSites || []).some((rule) => siteRuleMatches(hostname, rule))) {
    return { allowed: false, reason: "blocked_site", hostname };
  }
  if (settings.allowSites?.length && !settings.allowSites.some((rule) => siteRuleMatches(hostname, rule))) {
    return { allowed: false, reason: "not_allowlisted", hostname };
  }
  return { allowed: true, hostname };
}

export function shouldAutomaticallyIntercept(rawURL, settings, connected, now = Date.now()) {
  if (!settings.enabled || !settings.automaticInterception) return false;
  if (!settings.browserFallback && !connected) return false;
  return siteDecision(rawURL, settings, now).allowed;
}

export function isHTTPURL(raw) {
  try {
    const url = new URL(raw);
    return (url.protocol === "http:" || url.protocol === "https:") && Boolean(url.hostname);
  } catch {
    return false;
  }
}

export function normalizeDownloadHint(item = {}) {
  const url = String(item.url || "");
  if (!isHTTPURL(url)) return null;
  const filename = String(item.filename || "").trim();
  const saveDir = String(item.saveDir || "").trim();
  if (filename && (filename.includes("/") || filename.includes("\\") || filename.includes("\0"))) return null;
  if (saveDir.includes("\0")) return null;
  return { url, ...(filename ? { filename } : {}), ...(saveDir ? { save_dir: saveDir } : {}) };
}

export function isDownloadURL(raw, extensions = DOWNLOAD_EXTENSIONS) {
  if (!isHTTPURL(raw)) return false;
  return isDownloadFilename(new URL(raw).pathname, extensions);
}

export function isDownloadFilename(filename, extensions = DOWNLOAD_EXTENSIONS) {
  const pathname = String(filename).toLowerCase();
  const rules = extensions instanceof Set ? extensions : new Set(extensions);
  return rules.has(pathname.split(".").pop());
}

export function isDownloadMime(headers = [], mimeTypes = DOWNLOAD_MIME_TYPES) {
  const contentType = (headers.find((header) => header.name.toLowerCase() === "content-type")?.value || "")
    .split(";")[0].trim().toLowerCase();
  return isDownloadMimeValue(contentType, mimeTypes);
}

export function isDownloadMimeValue(contentType, mimeTypes = DOWNLOAD_MIME_TYPES) {
  const rules = mimeTypes instanceof Set ? mimeTypes : new Set(mimeTypes);
  return rules.has(String(contentType).split(";")[0].trim().toLowerCase());
}

export function normalizeNativeResult(response, error) {
  if (error) {
    return { success: false, outcome: OUTCOMES.APP_UNAVAILABLE, message: error.message || String(error) };
  }
  if (response?.success && response.outcome === OUTCOMES.ACCEPTED) return response;
  if (Object.values(OUTCOMES).includes(response?.outcome)) return response;
  return {
    success: false,
    outcome: OUTCOMES.INTERNAL_ERROR,
    message: response?.message || response?.error || "Native host returned an invalid response",
  };
}

export function diagnosticFor(result) {
  switch (result.outcome) {
    case OUTCOMES.ACCEPTED: return "Lunefetch is connected";
    case OUTCOMES.APP_UNAVAILABLE:
      if (/native host|not found|not registered/i.test(result.message || "")) {
        return "Install the Lunefetch browser integration, then retry";
      }
      return "Start Lunefetch, then retry the download";
    case OUTCOMES.INVALID_URL: return "Only HTTP and HTTPS downloads can be sent to Lunefetch";
    case OUTCOMES.UNAUTHORIZED: return "Restart Lunefetch or reinstall the browser integration";
    case OUTCOMES.QUEUE_FULL: return "Lunefetch is busy; the browser download was preserved";
    default: return "Lunefetch integration failed; check the extension console";
  }
}

export function createBrowserAdapter(api, firefox = typeof browser !== "undefined") {
  const callbackCall = (fn, ...args) => new Promise((resolve, reject) => {
    fn(...args, (value) => {
      const error = api.runtime.lastError;
      if (error) reject(new Error(error.message));
      else resolve(value);
    });
  });
  const call = (fn, ...args) => firefox ? Promise.resolve(fn(...args)) : callbackCall(fn, ...args);

  return {
    sendNative: (message) => call(api.runtime.sendNativeMessage.bind(api.runtime), NATIVE_HOST, message),
    cancel: (id) => call(api.downloads.cancel.bind(api.downloads), id),
    erase: (id) => call(api.downloads.erase.bind(api.downloads), { id }),
  };
}

export function createHandoffController(adapter, {
  now = Date.now,
  dedupeWindow = 15000,
  nativeTimeout = 10000,
  setTimer = setTimeout,
  clearTimer = clearTimeout,
} = {}) {
  let sequence = 0;
  const downloads = new Map();
  const intercepted = new Map();

  const rememberIntercept = (url) => {
    const entries = intercepted.get(url) || [];
    entries.push(now());
    intercepted.set(url, entries);
  };

  const claimIntercept = (url) => {
    const cutoff = now() - dedupeWindow;
    const entries = (intercepted.get(url) || []).filter((timestamp) => timestamp >= cutoff);
    if (!entries.length) {
      intercepted.delete(url);
      return false;
    }
    entries.shift();
    if (entries.length) intercepted.set(url, entries);
    else intercepted.delete(url);
    return true;
  };

  const send = async (url, hints = {}) => {
    const message = normalizeDownloadHint({ url, ...hints });
    if (!message) {
      return normalizeNativeResult({ outcome: OUTCOMES.INVALID_URL });
    }
    let timer;
    try {
      message.action = "download";
      const timeout = new Promise((_, reject) => {
        timer = setTimer(() => reject(new Error("Native host timed out")), nativeTimeout);
      });
      const response = await Promise.race([adapter.sendNative(message), timeout]);
      return normalizeNativeResult(response);
    } catch (error) {
      return normalizeNativeResult(null, error);
    } finally {
      if (timer !== undefined) clearTimer(timer);
    }
  };

  const interceptFirefox = async (url) => {
    const result = await send(url);
    if (result.success) rememberIntercept(url);
    return result;
  };

  const handleCreated = async (item) => {
    if (!isHTTPURL(item.url)) return { preserved: true, ignored: true };
    if (claimIntercept(item.url)) {
      try {
        await adapter.cancel(item.id);
      } catch (error) {
        return { preserved: true, deduplicated: true, error };
      }
      try {
        await adapter.erase(item.id);
      } catch (error) {
        return { preserved: true, deduplicated: true, error };
      }
      return { preserved: false, deduplicated: true };
    }

    const attempt = { id: ++sequence, browserDownloadId: item.id, url: item.url, state: "created" };
    downloads.set(item.id, attempt);

    const result = await send(item.url);
    attempt.result = result;
    if (!result.success) {
      attempt.state = "browser_preserved";
      return { preserved: true, result };
    }

    attempt.state = "accepted";
    try {
      await adapter.cancel(item.id);
      attempt.state = "cancelled";
    } catch (error) {
      attempt.state = "accepted_browser_preserved";
      return { preserved: true, result, error };
    }

    try {
      await adapter.erase(item.id);
    } catch (error) {
      attempt.eraseError = error;
      attempt.state = "accepted_erase_failed";
      return { preserved: true, result, error };
    }
    attempt.state = "completed";
    return { preserved: false, result };
  };

  return {
    send,
    health: async () => {
      let timer;
      try {
        const timeout = new Promise((_, reject) => {
          timer = setTimer(() => reject(new Error("Native host timed out")), nativeTimeout);
        });
        return normalizeNativeResult(await Promise.race([adapter.sendNative({ action: "health" }), timeout]));
      } catch (error) {
        return normalizeNativeResult(null, error);
      } finally {
        if (timer !== undefined) clearTimer(timer);
      }
    },
    interceptFirefox,
    handleCreated,
    stateFor: (downloadId) => downloads.get(downloadId),
  };
}
