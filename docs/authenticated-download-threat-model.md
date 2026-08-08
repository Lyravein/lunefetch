# Authenticated Download Protocol Decision

## Decision

Lunefetch will not transfer browser cookies, authorization headers, referrers,
request bodies, or browser-managed credentials in the current protocol. Phase 5
evaluated an opt-in authenticated mode and deferred implementation because the
required secret lifecycle and consent controls do not exist yet.

## Assets and Trust Boundaries

- Session cookies, bearer tokens, signed URLs, and account-specific headers are secrets.
- Web pages and download origins are untrusted input.
- The extension has browsing visibility, while the native host can reach the local API.
- Extension storage and process arguments are not approved secret stores.
- Logs, notifications, failure history, and diagnostics must not receive browser-managed credentials. Failed signed URLs remain locally retryable and must be treated as secrets.

## Principal Threats

- A compromised page triggers credential export to an attacker-selected URL.
- Broad host permissions expose unrelated sessions without clear per-origin consent.
- Secrets persist in extension storage, local state, crash reports, logs, or process memory.
- Redirects move credentials across origins or downgrade transport security.
- Retry and batch features replay expired or unintended authorization.
- A local process impersonates Lunefetch or reads a reusable credential envelope.

## Requirements Before Reconsideration

1. Explicit per-origin and per-download consent with a precise credential preview.
2. Origin-bound, short-lived capability tokens instead of raw reusable browser secrets.
3. OS-backed secure storage with expiry, revocation, and deletion behavior.
4. End-to-end authenticated encryption between extension and the intended app instance.
5. Redirect policy that never forwards credentials across origins without new consent.
6. Strict redaction from logs, notifications, diagnostics, history, and backups.
7. Independent security review plus Chromium and Firefox abuse-case tests.

Until all requirements are designed and reviewed, authenticated transfers remain
disabled. Signed query URLs can still be handed off as URLs and must be treated as
secrets by users and diagnostics.
