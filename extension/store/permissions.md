# Permission Rationale

| Permission | Purpose |
|------------|---------|
| `downloads` | Observe matching browser downloads and cancel/erase them only after Lunefetch accepts the URL. |
| `nativeMessaging` | Send health checks and accepted HTTP/HTTPS URLs to the locally installed Lunefetch native host. |
| `contextMenus` | Add “Download with Lunefetch” to links and media. |
| `storage` | Save interception toggles, file rules, site rules, and temporary bypasses locally. |
| `notifications` | Show handoff success and recoverable failure feedback when enabled. |
| `activeTab` | Identify the current site for the one-hour bypass control in the popup. |
| `<all_urls>` | Detect eligible downloads and apply configured site rules across sites. Non-HTTP(S) URLs are rejected. |
| Firefox `webRequest` / `webRequestBlocking` | Stop an eligible Firefox response only after Lunefetch has accepted its URL, avoiding duplicate browser downloads. |

Firefox declares required `browsingActivity` data handling because download
URLs are transferred outside the extension to the locally installed Lunefetch
application. They are not sent to the developer or a remote service.
