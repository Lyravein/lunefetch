import assert from "node:assert/strict";
import test from "node:test";

import { OUTCOMES } from "../src/core.mjs";
import {
  accepted,
  clearBrowserGlobals,
  createMockBrowser,
  createNativeHost,
  loadBackground,
} from "./mock-browser.mjs";

test.afterEach(clearBrowserGlobals);

test("Chromium automatic and context-menu handoffs use the native host", async () => {
  const nativeHost = createNativeHost(async () => accepted());
  const mock = createMockBrowser({ nativeHost });
  await loadBackground(mock, false);

  await mock.api.downloads.onCreated.emit({
    id: 41,
    url: "https://cdn.example.com/archive.zip?signature=a%2Bb%3D",
    filename: "archive.zip",
    mime: "application/zip",
  });
  await mock.api.contextMenus.onClicked.emit({ menuItemId: "lunefetch-download", linkUrl: "https://example.com/manual.iso" });

  assert.deepEqual(nativeHost.calls.map((call) => call.action), ["health", "download", "download"]);
  assert.equal(nativeHost.calls[1].url, "https://cdn.example.com/archive.zip?signature=a%2Bb%3D");
  assert.deepEqual(mock.calls.cancelled, [41]);
  assert.deepEqual(mock.calls.erased, [41]);
});

test("Download all opens a confirmation draft without handing links off", async () => {
  const nativeHost = createNativeHost(async () => accepted());
  const links = ["https://example.com/a.zip", "https://cdn.example.com/b.iso"];
  const mock = createMockBrowser({ nativeHost, pageLinks: links });
  await loadBackground(mock, false);

  await mock.api.contextMenus.onClicked.emit(
    { menuItemId: "lunefetch-download-all" },
    { id: 8, url: "https://example.com/downloads" },
  );

  assert.deepEqual(nativeHost.calls, [{ action: "health" }]);
  assert.deepEqual(mock.storageData.batchDraft.urls, links);
  assert.deepEqual(mock.calls.tabsCreated, [{ url: "mock-extension://batch.html" }]);
});

test("batch hints are forwarded and failures can be retried", async () => {
  let attempt = 0;
  const nativeHost = createNativeHost(async (message) => {
    if (message.action === "health") return accepted();
    attempt++;
    return attempt === 1 ? { success: false, outcome: OUTCOMES.QUEUE_FULL } : accepted();
  });
  const mock = createMockBrowser({ nativeHost });
  await loadBackground(mock, false);
  const item = { url: "https://example.com/a.zip", filename: "release.zip", saveDir: "/tmp/downloads" };

  const [results] = await mock.api.runtime.onMessage.emit({ type: "send-batch", items: [item] }, {}, () => {});
  assert.equal(results, true);
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(nativeHost.calls[1], { action: "download", url: item.url, filename: item.filename, save_dir: item.saveDir });
  assert.equal(mock.storageData.handoffFailures.length, 1);

  const id = mock.storageData.handoffFailures[0].id;
  await mock.api.runtime.onMessage.emit({ type: "retry-failure", id }, {}, () => {});
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(mock.storageData.handoffFailures.length, 0);
});

test("Chromium preserves the browser download for unavailable and malformed hosts", async () => {
  for (const response of [
    new Error("Specified native host not found"),
    { success: true },
  ]) {
    const nativeHost = createNativeHost(async (message) => {
      if (message.action === "health") return accepted();
      if (response instanceof Error) throw response;
      return response;
    });
    const mock = createMockBrowser({ nativeHost });
    await loadBackground(mock, false);
    await mock.api.downloads.onCreated.emit({ id: 7, url: "https://example.com/file.zip", filename: "file.zip" });
    assert.deepEqual(mock.calls.cancelled, []);
    assert.deepEqual(mock.calls.erased, []);
  }
});

test("Chromium service-worker restart reloads persisted controls", async () => {
  const storageData = {};
  const first = createMockBrowser({ storageData });
  await loadBackground(first, false);
  const [response] = await first.api.runtime.onMessage.emit(
    { type: "update-settings", settings: { enabled: false } },
    {},
    () => {},
  );
  assert.equal(response, true);
  await new Promise((resolve) => setTimeout(resolve, 0));

  const second = createMockBrowser({ storageData });
  await loadBackground(second, false);
  await second.api.downloads.onCreated.emit({ id: 9, url: "https://example.com/file.zip", filename: "file.zip" });

  assert.equal(storageData.settings.enabled, false);
  assert.deepEqual(second.calls.cancelled, []);
  assert.equal(second.calls.badges.at(-1), "OFF");
});

test("Firefox blocking interception suppresses its duplicate download event", async () => {
  const nativeHost = createNativeHost(async () => accepted());
  const mock = createMockBrowser({ firefox: true, nativeHost });
  await loadBackground(mock, true);
  const url = "https://downloads.example.com/redirected/file.iso?token=abc";

  const [decision] = await mock.api.webRequest.onHeadersReceived.emit({
    method: "GET",
    url,
    responseHeaders: [{ name: "Content-Type", value: "application/octet-stream" }],
  });
  await mock.api.downloads.onCreated.emit({ id: 12, url, filename: "file.iso" });

  assert.deepEqual(decision, { cancel: true });
  assert.equal(nativeHost.calls.filter((call) => call.action === "download").length, 1);
  assert.deepEqual(mock.calls.cancelled, [12]);
  assert.deepEqual(mock.calls.erased, [12]);
});

test("Firefox preserves its response when Lunefetch shuts down during handoff", async () => {
  let rejectDownload;
  const nativeHost = createNativeHost((message) => {
    if (message.action === "health") return accepted();
    return new Promise((_, reject) => { rejectDownload = reject; });
  });
  const mock = createMockBrowser({ firefox: true, nativeHost });
  await loadBackground(mock, true);
  const pending = mock.api.webRequest.onHeadersReceived.emit({
    method: "GET",
    url: "https://example.com/file.zip",
    responseHeaders: [{ name: "Content-Disposition", value: "attachment" }],
  });
  await new Promise((resolve) => setTimeout(resolve, 0));
  rejectDownload(new Error("Lunefetch stopped"));

  assert.deepEqual(await pending, [{}]);
  assert.deepEqual(mock.calls.cancelled, []);
});

test("blob and data downloads are excluded before native handoff", async () => {
  const nativeHost = createNativeHost(async () => accepted());
  const mock = createMockBrowser({ nativeHost });
  await loadBackground(mock, false);

  await mock.api.downloads.onCreated.emit({ id: 1, url: "blob:https://example.com/id", filename: "file.zip" });
  await mock.api.downloads.onCreated.emit({ id: 2, url: "data:application/zip,abc", filename: "file.zip" });

  assert.deepEqual(nativeHost.calls, [{ action: "health" }]);
  assert.deepEqual(mock.calls.cancelled, []);
});
