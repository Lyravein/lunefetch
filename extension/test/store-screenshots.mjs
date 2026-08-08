import { mkdir, mkdtemp, readFile, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { chromium } from "playwright";

const extensionPath = path.resolve("dist/chromium-unpacked");
const outputPath = path.resolve("store/screenshots");
const manifest = JSON.parse(await readFile(path.join(extensionPath, "manifest.json"), "utf8"));
if (manifest.manifest_version !== 3) throw new Error("Chromium extension is not built");

await mkdir(outputPath, { recursive: true });
const userDataDir = await mkdtemp(path.join(os.tmpdir(), "lunefetch-store-"));
const context = await chromium.launchPersistentContext(userDataDir, {
  channel: "chromium",
  headless: true,
  viewport: { width: 1280, height: 800 },
  args: [
    `--disable-extensions-except=${extensionPath}`,
    `--load-extension=${extensionPath}`,
  ],
});

try {
  let workers = context.serviceWorkers();
  if (!workers.length) workers = [await context.waitForEvent("serviceworker", { timeout: 15000 })];
  const extensionID = new URL(workers[0].url()).host;

  const options = await context.newPage();
  await options.goto(`chrome-extension://${extensionID}/options.html`);
  await options.locator("#extensions").waitFor();
  await options.screenshot({ path: path.join(outputPath, "settings-1280x800.png") });

  const popup = await context.newPage();
  await popup.goto(`chrome-extension://${extensionID}/popup.html`);
  await popup.locator("#enabled").waitFor();
  await popup.addStyleTag({ content: "body.popup { margin: 190px auto 0; }" });
  await popup.screenshot({ path: path.join(outputPath, "popup-1280x800.png") });
} finally {
  await context.close();
  await rm(userDataDir, { recursive: true, force: true });
}
