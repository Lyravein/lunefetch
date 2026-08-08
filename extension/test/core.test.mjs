import assert from "node:assert/strict";
import test from "node:test";

import {
  OUTCOMES,
  DEFAULT_SETTINGS,
  createBrowserAdapter,
  createHandoffController,
  diagnosticFor,
  isDownloadMime,
  isDownloadURL,
  isHTTPURL,
  normalizeSettings,
  normalizeNativeResult,
  normalizeDownloadHint,
  normalizeSiteRule,
  shouldAutomaticallyIntercept,
  siteDecision,
} from "../src/core.mjs";

function promiseAdapter({ native = { success: true, outcome: OUTCOMES.ACCEPTED }, cancel, erase } = {}) {
  return {
    sendNative: async () => native,
    cancel: cancel || (async () => {}),
    erase: erase || (async () => {}),
  };
}

test("recognizes only replayable HTTP URLs", () => {
  assert.equal(isHTTPURL("https://example.com/file?token=abc"), true);
  assert.equal(isHTTPURL("http://example.com/file"), true);
  assert.equal(isHTTPURL("blob:https://example.com/id"), false);
  assert.equal(isHTTPURL("data:text/plain,hello"), false);
  assert.equal(isHTTPURL("file:///tmp/file"), false);
  assert.equal(isDownloadURL("https://example.com/archive.ZIP?token=abc"), true);
  assert.equal(isDownloadURL("https://example.com/page"), false);
});

test("normalizes explicit hints without accepting filename paths", () => {
  assert.deepEqual(normalizeDownloadHint({
    url: "https://example.com/file", filename: "release.zip", saveDir: "/tmp/downloads",
  }), { url: "https://example.com/file", filename: "release.zip", save_dir: "/tmp/downloads" });
  assert.equal(normalizeDownloadHint({ url: "https://example.com/file", filename: "../secret" }), null);
  assert.equal(normalizeDownloadHint({ url: "file:///tmp/file" }), null);
});

test("detects configured download MIME types", () => {
  assert.equal(isDownloadMime([{ name: "Content-Type", value: "application/zip; charset=binary" }]), true);
  assert.equal(isDownloadMime([{ name: "content-type", value: "text/html" }]), false);
});

test("normalizes persisted settings and rejects malformed rules", () => {
  const settings = normalizeSettings({
    enabled: false,
    extensions: [".ZIP", "zip", "bad value"],
    mimeTypes: ["Application/ZIP", "invalid"],
    allowSites: ["https://Downloads.Example.com/path", "*.example.org", "bad host!"],
  });
  assert.equal(settings.enabled, false);
  assert.deepEqual(settings.extensions, ["zip"]);
  assert.deepEqual(settings.mimeTypes, ["application/zip"]);
  assert.deepEqual(settings.allowSites, ["downloads.example.com", "example.org"]);
  assert.deepEqual(normalizeSettings({}).extensions, DEFAULT_SETTINGS.extensions);
  assert.deepEqual(normalizeSettings(null).extensions, DEFAULT_SETTINGS.extensions);
  assert.equal(normalizeSiteRule("https://example.com/path"), "example.com");
});

test("applies temporary bypass and block rules before allow rules", () => {
  const settings = normalizeSettings({
    allowSites: ["example.com"],
    blockSites: ["private.example.com"],
    bypassUntil: { "downloads.example.com": 2000 },
  });
  assert.equal(siteDecision("https://cdn.example.com/file", settings, 1000).allowed, true);
  assert.equal(siteDecision("https://private.example.com/file", settings, 1000).reason, "blocked_site");
  assert.equal(siteDecision("https://downloads.example.com/file", settings, 1000).reason, "temporary_bypass");
  assert.equal(siteDecision("https://other.test/file", settings, 1000).reason, "not_allowlisted");
  assert.equal(siteDecision("https://downloads.example.com/file", settings, 3000).allowed, true);
});

test("applies global, automatic, and offline fallback controls without weakening preservation", () => {
  const url = "https://example.com/file.zip";
  assert.equal(shouldAutomaticallyIntercept(url, normalizeSettings({}), true), true);
  assert.equal(shouldAutomaticallyIntercept(url, normalizeSettings({ enabled: false }), true), false);
  assert.equal(shouldAutomaticallyIntercept(url, normalizeSettings({ automaticInterception: false }), true), false);
  assert.equal(shouldAutomaticallyIntercept(url, normalizeSettings({ browserFallback: false }), false), false);
  assert.equal(shouldAutomaticallyIntercept(url, normalizeSettings({ browserFallback: true }), false), true);
});

test("normalizes malformed native-host responses", () => {
  assert.deepEqual(normalizeNativeResult({ success: true }), {
    success: false,
    outcome: OUTCOMES.INTERNAL_ERROR,
    message: "Native host returned an invalid response",
  });
  assert.equal(normalizeNativeResult(null, new Error("host missing")).outcome, OUTCOMES.APP_UNAVAILABLE);
});

test("times out a native host that never responds", async () => {
  const hanging = createHandoffController({
    ...promiseAdapter(),
    sendNative: async () => new Promise(() => {}),
  }, {
    nativeTimeout: 1,
    setTimer: (callback) => { queueMicrotask(callback); return 1; },
    clearTimer: () => {},
  });
  const result = await hanging.send("https://example.com/file.zip");
  assert.equal(result.outcome, OUTCOMES.APP_UNAVAILABLE);
  assert.match(result.message, /timed out/);
});

test("sends filename and destination hints without request credentials", async () => {
  let message;
  const controller = createHandoffController({
    ...promiseAdapter(),
    sendNative: async (value) => { message = value; return { success: true, outcome: OUTCOMES.ACCEPTED }; },
  });
  await controller.send("https://example.com/file", { filename: "release.zip", saveDir: "/tmp/downloads" });
  assert.deepEqual(message, {
    action: "download", url: "https://example.com/file", filename: "release.zip", save_dir: "/tmp/downloads",
  });
  assert.equal("cookies" in message, false);
  assert.equal("headers" in message, false);
  assert.equal("referrer" in message, false);
});

test("provides distinct installation and startup diagnostics", () => {
  assert.match(diagnosticFor({ outcome: OUTCOMES.APP_UNAVAILABLE, message: "Specified native host not found" }), /Install/);
  assert.match(diagnosticFor({ outcome: OUTCOMES.APP_UNAVAILABLE, message: "Start Lunefetch and try again" }), /Start/);
});

test("uses Chromium callback APIs through the adapter", async () => {
  const api = {
    runtime: {
      lastError: null,
      sendNativeMessage(host, message, callback) {
        assert.equal(host, "com.lyravein.lunefetch");
        assert.equal(message.action, "health");
        callback({ success: true, outcome: OUTCOMES.ACCEPTED });
      },
    },
    downloads: {
      cancel(_id, callback) { callback(); },
      erase(_query, callback) { callback([]); },
      download(_options, callback) { callback(1); },
    },
  };
  const adapter = createBrowserAdapter(api, false);
  assert.equal((await adapter.sendNative({ action: "health" })).outcome, OUTCOMES.ACCEPTED);
});

test("uses Firefox Promise APIs through the adapter", async () => {
  const api = {
    runtime: {
      sendNativeMessage: async () => ({ success: true, outcome: OUTCOMES.ACCEPTED }),
    },
    downloads: {
      cancel: async () => {},
      erase: async () => [],
      download: async () => 7,
    },
  };
  const adapter = createBrowserAdapter(api, true);
  assert.equal((await adapter.sendNative({ action: "health" })).outcome, OUTCOMES.ACCEPTED);
});

test("preserves a browser download when cancellation fails after acceptance", async () => {
  const controller = createHandoffController(promiseAdapter({
    cancel: async () => { throw new Error("cannot cancel"); },
  }));
  const outcome = await controller.handleCreated({ id: 1, url: "https://example.com/file.zip" });
  assert.equal(outcome.preserved, true);
  assert.equal(controller.stateFor(1).state, "accepted_browser_preserved");
});

test("preserves concurrent identical browser downloads when handoff fails", async () => {
  const controller = createHandoffController(promiseAdapter({
    native: { success: false, outcome: OUTCOMES.APP_UNAVAILABLE },
  }));
  const first = await controller.handleCreated({ id: 1, url: "https://example.com/file.zip" });
  const second = await controller.handleCreated({ id: 2, url: "https://example.com/file.zip" });
  assert.equal(first.preserved, true);
  assert.equal(second.preserved, true);
  assert.equal(controller.stateFor(1).state, "browser_preserved");
  assert.equal(controller.stateFor(2).state, "browser_preserved");
});

test("preserves the cancelled browser record when erase fails", async () => {
  const controller = createHandoffController(promiseAdapter({
    erase: async () => { throw new Error("cannot erase"); },
  }));
  const outcome = await controller.handleCreated({ id: 1, url: "https://example.com/file.zip" });
  assert.equal(outcome.preserved, true);
  assert.equal(controller.stateFor(1).state, "accepted_erase_failed");
});

test("suppresses Firefox's follow-up download event after a successful interception", async () => {
  const controller = createHandoffController(promiseAdapter());
  const result = await controller.interceptFirefox("https://example.com/file.zip");
  assert.equal(result.success, true);
  const outcome = await controller.handleCreated({ id: 4, url: "https://example.com/file.zip" });
  assert.equal(outcome.deduplicated, true);
  assert.equal(outcome.preserved, false);
});

test("preserves Firefox's duplicate browser event if cancellation fails", async () => {
  const controller = createHandoffController(promiseAdapter({
    cancel: async () => { throw new Error("cannot cancel"); },
  }));
  await controller.interceptFirefox("https://example.com/file.zip");
  const outcome = await controller.handleCreated({ id: 4, url: "https://example.com/file.zip" });
  assert.equal(outcome.deduplicated, true);
  assert.equal(outcome.preserved, true);
});
