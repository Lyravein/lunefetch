import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const root = new URL("..", import.meta.url);
const version = (await readFile(new URL("../../VERSION", import.meta.url), "utf8")).trim();

async function pngSize(url) {
  const data = await readFile(url);
  assert.equal(data.subarray(1, 4).toString(), "PNG");
  return [data.readUInt32BE(16), data.readUInt32BE(20)];
}

for (const target of ["chromium", "firefox"]) {
  test(`${target} manifest references valid packaged assets and required permissions`, async () => {
    const manifest = JSON.parse(await readFile(new URL(`manifests/${target}.json`, root), "utf8"));
    assert.equal(manifest.manifest_version, 3);
    assert.equal(manifest.version, version);
    for (const permission of ["downloads", "nativeMessaging", "contextMenus", "storage", "notifications", "activeTab", "scripting"]) {
      assert.ok(manifest.permissions.includes(permission), `missing ${permission}`);
    }
    assert.deepEqual(manifest.host_permissions, ["<all_urls>"]);

    const assets = [
      "src/background.js", "src/core.mjs", "src/popup.html", "src/options.html", "src/batch.html", "src/batch.mjs",
      "icons/icon-16.png", "icons/icon-32.png", "icons/icon-48.png", "icons/icon-128.png",
    ];
    for (const asset of assets) await access(new URL(asset, root));
    for (const size of [16, 32, 48, 128]) {
      assert.deepEqual(await pngSize(new URL(`icons/icon-${size}.png`, root)), [size, size]);
      assert.equal(manifest.icons[String(size)], `icons/icon-${size}.png`);
    }
    if (target === "chromium") assert.equal(manifest.background.service_worker, "background.js");
    else assert.deepEqual(manifest.background.scripts, ["background.js"]);
    assert.equal(path.extname(manifest.action.default_popup), ".html");
  });
}

test("store listing includes descriptions, privacy, permissions, and screenshots", async () => {
  for (const file of ["listing.md", "privacy.md", "permissions.md"]) {
    assert.ok((await readFile(new URL(`store/${file}`, root), "utf8")).length > 200);
  }
  for (const file of ["settings-1280x800.png", "popup-1280x800.png"]) {
    assert.deepEqual(await pngSize(new URL(`store/screenshots/${file}`, root)), [1280, 800]);
  }
});
