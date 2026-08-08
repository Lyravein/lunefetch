import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { chromium } from "playwright";

const extensionPath = path.resolve("dist/chromium-unpacked");
const manifest = JSON.parse(await readFile(path.join(extensionPath, "manifest.json"), "utf8"));
assert.equal(manifest.manifest_version, 3);

const userDataDir = await mkdtemp(path.join(os.tmpdir(), "lunefetch-chromium-"));
const context = await chromium.launchPersistentContext(userDataDir, {
  channel: "chromium",
  headless: true,
  args: [
    `--disable-extensions-except=${extensionPath}`,
    `--load-extension=${extensionPath}`,
  ],
});

try {
  let workers = context.serviceWorkers();
  if (!workers.length) workers = [await context.waitForEvent("serviceworker", { timeout: 15000 })];
  const worker = workers[0];
  const extensionID = new URL(worker.url()).host;
  assert.ok(extensionID);

  const popup = await context.newPage();
  await popup.goto(`chrome-extension://${extensionID}/popup.html`);
  await popup.locator("#enabled").waitFor();
  assert.equal(await popup.locator(".brand strong").textContent(), "Lunefetch");
  assert.match(await popup.locator("#status-detail").textContent(), /Install|Start|connected|Contacting/i);

  const options = await context.newPage();
  await options.goto(`chrome-extension://${extensionID}/options.html`);
  await options.locator("#extensions").waitFor();
  assert.match(await options.locator("#extensions").inputValue(), /zip/);

  const batch = await context.newPage();
  await batch.goto(`chrome-extension://${extensionID}/batch.html`);
  await batch.locator("#send").waitFor();
  assert.equal(await batch.locator("h1").textContent(), "Confirm downloads");
  assert.equal(await batch.locator("#send").isDisabled(), true);
} finally {
  await context.close();
  await rm(userDataDir, { recursive: true, force: true });
}
