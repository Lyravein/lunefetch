# Lunefetch {{VERSION}}

- Fixed: expired one-hour site pauses are now cleaned up instead of being kept
  forever.
- Fixed: long browsing sessions no longer grow the extension's memory without
  bound.
- Fixed: only real downloads are intercepted. Background requests a page makes on
  its own, such as analytics pings and internal data fetches, are ignored even
  when their response headers look like a file.
- Fixed: an address with no file extension no longer matches your file rules.
- MV3 fix: interception now always respects your current settings, even after the
  background worker restarts.
- Only the extension's own pages can queue downloads or change settings.
- Context menu entries survive background worker restarts.
- Batch confirmation drafts expire, and collected links are validated and capped.
- Reliable native-host handoff with browser-download preservation on failures.
- Popup connection status, interception controls, custom file rules, and site rules.
- Firefox and Chromium packages built from the same tested extension source.
- Firefox 142 or newer is required.

See the repository changelog and security policy for application-level changes and security guidance.
