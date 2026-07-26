const NATIVE_HOST = "com.lyravein.lunefetch";

// File extensions that should be captured by Lunefetch.
const DOWNLOAD_EXTENSIONS = [
  "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "zst",
  "iso", "img", "dmg",
  "exe", "msi", "deb", "rpm", "pkg", "apk",
  "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm",
  "mp3", "flac", "wav", "aac", "ogg",
  "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx",
];

// MIME types that should always be treated as downloads.
const DOWNLOAD_MIME_TYPES = [
  "application/zip", "application/x-rar-compressed", "application/x-7z-compressed",
  "application/x-tar", "application/gzip", "application/x-bzip2", "application/x-xz",
  "application/x-iso9660-image", "application/octet-stream",
  "application/x-msdownload", "application/vnd.debian.binary-package",
  "application/x-rpm", "application/vnd.android.package-archive",
  "video/mp4", "video/x-matroska", "video/avi", "video/quicktime", "video/webm",
  "audio/mpeg", "audio/flac", "audio/wav", "audio/aac", "audio/ogg",
  "application/pdf",
];

function isDownloadURL(url) {
  try {
    const path = new URL(url).pathname.toLowerCase();
    const ext = path.split(".").pop();
    return DOWNLOAD_EXTENSIONS.includes(ext);
  } catch {
    return false;
  }
}

function isDownloadMime(headers) {
  const ct = (headers.find(h => h.name.toLowerCase() === "content-type")?.value || "")
    .split(";")[0].trim();
  return DOWNLOAD_MIME_TYPES.includes(ct);
}

async function sendToLunefetch(url) {
  try {
    const response = await browser.runtime.sendNativeMessage(NATIVE_HOST, { url });
    if (!response.success) {
      console.error("Lunefetch: native host error:", response.error);
      return false;
    }
    console.log("Lunefetch: sent to TUI:", url);
    return true;
  } catch (err) {
    console.error("Lunefetch: could not reach native host:", err);
    return false;
  }
}

// Method 1: intercept via webRequest — fires before download dialog appears.
// Intercepts: Content-Disposition attachment/inline + known MIME type or file extension.
browser.webRequest.onHeadersReceived.addListener(
  async (details) => {
    if (details.method !== "GET") return {};
    if (details.url.startsWith("blob:") || details.url.startsWith("data:")) return {};

    const headers = details.responseHeaders || [];
    const contentDisposition = (headers
      .find(h => h.name.toLowerCase() === "content-disposition")
      ?.value || "").toLowerCase();

    const isAttachment = contentDisposition.includes("attachment");
    const isInlineFile = contentDisposition.includes("inline") && isDownloadMime(headers);
    const isKnownMime = isDownloadMime(headers);
    const isKnownExt = isDownloadURL(details.url);

    if (!isAttachment && !isInlineFile && !isKnownMime && !isKnownExt) return {};

    console.log("Lunefetch: webRequest intercepted:", details.url);

    const ok = await sendToLunefetch(details.url);
    if (ok) {
      return { cancel: true };
    }
    return {};
  },
  { urls: ["<all_urls>"] },
  ["blocking", "responseHeaders"]
);

// Method 2: downloads.onCreated as fallback (catches anything webRequest misses).
browser.downloads.onCreated.addListener(async (item) => {
  if (item.url.startsWith("blob:") || item.url.startsWith("data:")) return;

  console.log("Lunefetch: downloads.onCreated:", item.url);
  await browser.downloads.cancel(item.id);
  await browser.downloads.erase({ id: item.id });

  const ok = await sendToLunefetch(item.url);
  if (!ok) {
    browser.downloads.download({ url: item.url });
  }
});

// Method 3: context menu — right-click any link/image/video.
browser.contextMenus.create({
  id: "lunefetch-download",
  title: "Download with Lunefetch",
  contexts: ["link", "image", "video", "audio"],
});

browser.contextMenus.onClicked.addListener(async (info) => {
  const url = info.linkUrl || info.srcUrl;
  if (!url) return;
  console.log("Lunefetch: context menu download:", url);
  await sendToLunefetch(url);
});

console.log("Lunefetch extension loaded.");
