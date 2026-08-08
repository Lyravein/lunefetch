# Security Policy

## Supported Versions

Lunefetch is currently a development preview. Security fixes are applied only
to the latest revision of the default branch.

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

The local API listens on loopback and requires the token stored at
`~/.config/lunefetch/api-token`. Treat that file and URLs containing signed
query parameters as secrets.

Authenticated browser transfers are intentionally not implemented. The threat
model and the controls required before reconsidering this boundary are recorded
in `docs/authenticated-download-threat-model.md`.
