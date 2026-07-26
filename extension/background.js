const NATIVE_HOST = "com.lyravein.lunefetch";

// Intercept all downloads before they start.
browser.downloads.onCreated.addListener(async (item) => {
  // Skip blob URLs (in-page downloads) and data URIs — can't resume these.
  if (item.url.startsWith("blob:") || item.url.startsWith("data:")) {
    return;
  }

  // Cancel the browser's own download immediately.
  await browser.downloads.cancel(item.id);
  await browser.downloads.erase({ id: item.id });

  // Forward URL to the native host, which sends it to the running TUI.
  try {
    const response = await browser.runtime.sendNativeMessage(NATIVE_HOST, {
      url: item.url,
    });
    if (!response.success) {
      console.error("Lunefetch: native host error:", response.error);
      // Fall back: re-open the URL so the browser can download it normally.
      browser.downloads.download({ url: item.url });
    }
  } catch (err) {
    console.error("Lunefetch: could not reach native host:", err);
    // Fall back: let the browser download it normally.
    browser.downloads.download({ url: item.url });
  }
});

console.log("Lunefetch extension loaded.");
