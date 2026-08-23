# Security Policy

## Supported Versions

Security fixes are applied to the latest release and the default branch. The
current release is 1.1.0.

## Reporting a Vulnerability

Do not open a public issue containing exploit details, private download URLs,
API tokens, proxy credentials, or other secrets. Report the issue privately to
the project maintainer through GitHub's private vulnerability reporting when
available.

Include the affected revision, platform, impact, reproduction steps, and any
suggested remediation. Avoid testing beyond the minimum proof needed and do not
access data that is not yours.

## Browser Integration Scope

The extension and native host hand off a URL plus optional user-entered filename
and destination hints. They do not copy browser cookies, authorization headers,
referrers, request bodies, or page content. Only use automatic interception for
replayable, unauthenticated HTTP/HTTPS GETs.

A destination hint from the extension is confined to the configured download
directory by the desktop application, so a caller holding the API token cannot
write files elsewhere.

The local API listens on loopback and requires a bearer token stored at
`~/.config/lunefetch/api-token` on Linux and `%AppData%\lunefetch\api-token` on
Windows, with owner-only permissions. Treat that file, and URLs containing signed
query parameters, as secrets.

## Network Destination Policy

By default the downloader refuses loopback, private, link-local, CGNAT
(`100.64.0.0/10`, which includes Tailscale tailnets), and other reserved address
ranges, so a URL originating from a web page cannot be used to probe an internal
network. Addresses are screened after DNS resolution and again on every redirect,
which closes DNS rebinding.

Setting `allow_local_hosts: true` relaxes this for NAS and home-server use. Cloud
instance-metadata endpoints remain blocked regardless of that setting.

Authenticated browser transfers are intentionally not implemented. The threat
model and the controls required before reconsidering this boundary are recorded
in `docs/authenticated-download-threat-model.md`.
