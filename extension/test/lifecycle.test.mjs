import assert from "node:assert/strict";
import test from "node:test";

import { isAbsoluteDir, normalizeDownloadHint } from "../src/core.mjs";
import {
  accepted,
  clearBrowserGlobals,
  createMockBrowser,
  createNativeHost,
  importBackground,
  loadBackground,
} from "./mock-browser.mjs";

test.afterEach(clearBrowserGlobals);

// MV3 terminates the service worker when idle and restarts it on the next event.
// Module-scope state is therefore empty when a listener runs, so a listener that
// reads settings before initialization finishes would use defaults and intercept
// a download the user had explicitly disabled.
test("a download arriving before initialization still honours persisted settings", async () => {
  const storageData = { settings: { enabled: false } };
  const nativeHost = createNativeHost(async () => accepted());
  const mock = createMockBrowser({ nativeHost, storageData });

  const module = await importBackground(mock, false);
  // Do NOT await module.startup: emit while initialization is still in flight.
  await mock.api.downloads.onCreated.emit({
    id: 3,
    url: "https://example.com/file.zip",
    filename: "file.zip",
    mime: "application/zip",
  });
  await module.startup;

  assert.deepEqual(nativeHost.calls.filter((call) => call.action === "download"), []);
  assert.deepEqual(mock.calls.cancelled, []);
});

test("a context-menu click during startup is not lost", async () => {
  const nativeHost = createNativeHost(async () => accepted());
  const mock = createMockBrowser({ nativeHost });

  const module = await importBackground(mock, false);
  await mock.api.contextMenus.onClicked.emit({
    menuItemId: "lunefetch-download",
    linkUrl: "https://example.com/manual.iso",
  });
  await module.startup;

  assert.deepEqual(
    nativeHost.calls.filter((call) => call.action === "download").map((call) => call.url),
    ["https://example.com/manual.iso"],
  );
});

// contextMenus.create throws on a duplicate id, and the worker re-runs this file
// on every restart, so creation has to clear the previous items first.
test("context menus are recreated cleanly across service-worker restarts", async () => {
  const storageData = {};
  const first = createMockBrowser({ storageData });
  await loadBackground(first, false);
  assert.deepEqual(first.calls.contextCreated.map((item) => item.id), [
    "lunefetch-download",
    "lunefetch-download-all",
  ]);

  const second = createMockBrowser({ storageData });
  await loadBackground(second, false);
  assert.equal(second.calls.contextRemovedAll, 1);
  assert.deepEqual(second.calls.contextCreated.map((item) => item.id), [
    "lunefetch-download",
    "lunefetch-download-all",
  ]);
});

test("context menu items start hidden when interception is disabled", async () => {
  const mock = createMockBrowser({ storageData: { settings: { enabled: false } } });
  await loadBackground(mock, false);
  for (const item of mock.calls.contextCreated) {
    assert.equal(item.visible, false, `${item.id} should start hidden`);
  }
});

// A web page that learns the extension id, or any other installed extension,
// must not be able to queue downloads or rewrite settings.
test("messages from untrusted senders are rejected", async () => {
  const nativeHost = createNativeHost(async () => accepted());
  const mock = createMockBrowser({ nativeHost });
  await loadBackground(mock, false);

  const untrusted = [
    { id: "some-other-extension" },
    { tab: { id: 4 }, url: "https://evil.example.com/page" },
    { url: "https://evil.example.com/page", origin: "https://evil.example.com" },
    // A content script injected into a page: the sender carries this
    // extension's id but the host page's URL.
    { id: "mock-extension-id", tab: { id: 7 }, url: "https://evil.example.com/page" },
    // Tab-scoped with nothing to verify.
    { id: "mock-extension-id", tab: { id: 8 } },
  ];
  for (const sender of untrusted) {
    let response;
    await mock.api.runtime.onMessage.emit(
      { type: "send-download", item: { url: "https://evil.example.com/payload.exe" } },
      sender,
      (value) => { response = value; },
    );
    assert.equal(response?.error, "Untrusted sender", `sender ${JSON.stringify(sender)} was trusted`);
  }

  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(nativeHost.calls.filter((call) => call.action === "download"), []);
});

// An extension page opened in a tab (options.html, batch.html) carries
// sender.tab in Chromium. Rejecting every tab-scoped sender broke the options
// page: its get-state never resolved and every field rendered empty.
test("extension pages opened in a tab are accepted", async () => {
  const mock = createMockBrowser({ nativeHost: createNativeHost(async () => accepted()) });
  await loadBackground(mock, false);

  let response;
  await mock.api.runtime.onMessage.emit(
    { type: "get-state" },
    {
      id: mock.api.runtime.id,
      url: "mock-extension://options.html",
      origin: "mock-extension://",
      tab: { id: 12, url: "mock-extension://options.html" },
    },
    (value) => { response = value; },
  );
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.ok(response?.settings, "an extension page in a tab was refused");
  assert.ok(response.settings.extensions.length > 0);
});

test("messages from the extension's own pages are accepted", async () => {
  const nativeHost = createNativeHost(async () => accepted());
  const mock = createMockBrowser({ nativeHost });
  await loadBackground(mock, false);

  let response;
  await mock.api.runtime.onMessage.emit(
    { type: "get-state" },
    { id: mock.api.runtime.id, url: "mock-extension://popup.html" },
    (value) => { response = value; },
  );
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.ok(response?.settings, "own popup should receive state");
});

// The desktop API rejects a relative save_dir with HTTP 400. Filtering it here
// turns a silent failure into a clear invalid-URL outcome.
test("relative destination hints are dropped before reaching the native host", async () => {
  const nativeHost = createNativeHost(async () => accepted());
  const mock = createMockBrowser({ nativeHost });
  await loadBackground(mock, false);

  await mock.api.runtime.onMessage.emit(
    {
      type: "send-batch",
      items: [{ url: "https://example.com/a.zip", saveDir: "../../etc" }],
    },
    { id: mock.api.runtime.id, url: "mock-extension://batch.html" },
    () => {},
  );
  await new Promise((resolve) => setTimeout(resolve, 0));

  const download = nativeHost.calls.find((call) => call.action === "download");
  assert.ok(download, "the download should still be attempted");
  assert.equal(download.save_dir, undefined, "a relative destination must not be forwarded");
});

test("normalizeDownloadHint rejects relative destinations and accepts absolute ones", () => {
  assert.equal(normalizeDownloadHint({ url: "https://example.com/a.zip", saveDir: "relative/dir" }), null);
  assert.equal(normalizeDownloadHint({ url: "https://example.com/a.zip", saveDir: "../escape" }), null);
  assert.deepEqual(
    normalizeDownloadHint({ url: "https://example.com/a.zip", saveDir: "/srv/downloads" }),
    { url: "https://example.com/a.zip", save_dir: "/srv/downloads" },
  );

  assert.equal(isAbsoluteDir("/tmp"), true);
  assert.equal(isAbsoluteDir("C:\\Users\\me\\Downloads"), true);
  assert.equal(isAbsoluteDir("\\\\server\\share"), true);
  assert.equal(isAbsoluteDir("tmp"), false);
  assert.equal(isAbsoluteDir("/tmp/with\0null"), false);
  assert.equal(isAbsoluteDir(""), false);
});

// A draft left in storage from an unrelated page must not be resent later, and
// the list must not live in storage indefinitely.
test("a stale batch draft is discarded on startup", async () => {
  const storageData = {
    batchDraft: {
      source: "https://old.example.com",
      urls: ["https://old.example.com/a.zip"],
      createdAt: Date.now() - 60 * 60 * 1000,
    },
  };
  const mock = createMockBrowser({ storageData });
  await loadBackground(mock, false);
  assert.equal(storageData.batchDraft, undefined, "an hour-old draft should be pruned");
});

test("a fresh batch draft survives startup", async () => {
  const storageData = {
    batchDraft: {
      source: "https://example.com",
      urls: ["https://example.com/a.zip"],
      createdAt: Date.now(),
    },
  };
  const mock = createMockBrowser({ storageData });
  await loadBackground(mock, false);
  assert.deepEqual(storageData.batchDraft.urls, ["https://example.com/a.zip"]);
});

test("collected page links are filtered to HTTP URLs and capped", async () => {
  const pageLinks = [
    "https://example.com/a.zip",
    "javascript:alert(1)",
    "data:text/plain,hello",
    ...Array.from({ length: 150 }, (_, index) => `https://example.com/f${index}.zip`),
  ];
  const mock = createMockBrowser({ pageLinks });
  await loadBackground(mock, false);

  await mock.api.contextMenus.onClicked.emit(
    { menuItemId: "lunefetch-download-all" },
    { id: 2, url: "https://example.com/list" },
  );

  const urls = mock.storageData.batchDraft.urls;
  assert.equal(urls.length, 100, "the draft should be capped at 100 links");
  assert.ok(urls.every((url) => url.startsWith("https://")), "only HTTP(S) links should survive");
});

test("failure history stays bounded across repeated failures", async () => {
  const nativeHost = createNativeHost(async (message) => {
    if (message.action === "health") return accepted();
    throw new Error("Specified native host not found");
  });
  const mock = createMockBrowser({ nativeHost });
  await loadBackground(mock, false);

  const items = Array.from({ length: 25 }, (_, index) => ({ url: `https://example.com/f${index}.zip` }));
  await mock.api.runtime.onMessage.emit(
    { type: "send-batch", items },
    { id: mock.api.runtime.id, url: "mock-extension://batch.html" },
    () => {},
  );
  await new Promise((resolve) => setTimeout(resolve, 0));

  assert.ok(
    mock.storageData.handoffFailures.length <= 10,
    `failure history grew to ${mock.storageData.handoffFailures.length}`,
  );
});

test("an oversized persisted failure list is trimmed on load", async () => {
  const storageData = {
    handoffFailures: Array.from({ length: 40 }, (_, index) => ({
      id: String(index),
      url: `https://example.com/f${index}.zip`,
      outcome: "app_unavailable",
      message: "stale",
      timestamp: Date.now(),
    })),
  };
  const mock = createMockBrowser({ storageData });
  await loadBackground(mock, false);
  assert.equal(storageData.handoffFailures.length, 10);
});
