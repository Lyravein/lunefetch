# Browser Extension Privacy Disclosure

Lunefetch Browser Integration does not send telemetry, analytics, advertising
identifiers, cookies, authorization headers, referrers, or request bodies to
the developer or any third-party service.

When interception is enabled, the extension processes download URLs and sends
an accepted HTTP or HTTPS URL through the browser native-messaging API to the
Lunefetch application installed on the same computer. URLs can contain private
or signed query parameters and should be treated as secrets. The local native
host forwards the URL only to Lunefetch's authenticated loopback API.

Settings, site rules, and temporary bypass entries are stored in the browser's
local extension storage. They are not synchronized by this extension and are
not transmitted to the developer.

Locally stored data is bounded: the recent-failure list keeps at most ten
entries, and a “Download all” confirmation draft is discarded after ten minutes.

Users can disable interception globally, limit interception by site, or remove
the extension and native host. See `docs/browser-installation.md` for uninstall
instructions.
