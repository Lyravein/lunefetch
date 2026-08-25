# Configuration

Lunefetch reads a YAML config file at startup. Every field is range-checked on
load and on save: an invalid config is rejected rather than silently corrected,
so a typo fails loudly instead of changing behaviour.

## File locations

| | Linux | Windows |
|---|---|---|
| Config, API token | `~/.config/lunefetch` | `%AppData%\lunefetch` |
| Database, instance lock | `$XDG_DATA_HOME` or `~/.local/share/lunefetch` | `%LocalAppData%\lunefetch` |

## Fields

Defined in `internal/config/config.go`.

| Field | Meaning |
|---|---|
| `download_dir` | Default download directory |
| `max_concurrent_downloads` | Concurrent download limit |
| `max_retries` | Retry attempts per chunk (default 3) |
| `retry_backoff_s` | Base backoff seconds, doubling per attempt, capped at two minutes (default 1) |
| `min_free_space_mb` | Minimum free disk space required before starting |
| `timeout` | Request timeout in seconds |
| `chunk_rules` | Chunk count per size bucket (`small`, `medium`, `large`, `xlarge`) |
| `small_size`, `medium_size`, `large_size` | Size thresholds that select a chunk rule |
| `global_speed_limit` | Bytes per second across all downloads; `0` is unlimited |
| `proxy_url` | `http`, `https`, or `socks5` URL; empty means direct |
| `notifications` | Desktop notification on completion |
| `close_to_tray` | Hide on close and keep running (default `true`) |
| `history_retention_days` | `0` keeps history forever |
| `allow_local_hosts` | Permit LAN, loopback, and link-local targets |

Per-download speed limits are set from a row's action menu and stored in the
database, not the config file. The global limit and a per-download limit are both
enforced at once, and the stricter one wins.

## Background mode

With `close_to_tray` enabled, closing the window hides Lunefetch and keeps active
downloads running. Use the tray icon to reopen the window, add a download, or
quit. Set `close_to_tray: false` for normal close behaviour.

## Database schema

SQLite, with `foreign_keys` and `busy_timeout` set in the connection string.

`downloads`
: `id`, `url`, `filename`, `save_dir`, `category`, `speed_limit`, `total_size`,
  `downloaded_size`, `status`, `supports_ranges`, `num_chunks`, `etag`,
  `last_modified`, `queue_position`, `scheduled_at`, `created_at`, `updated_at`,
  `deleted_at`

`chunks`
: `id`, `download_id`, `chunk_index`, `start_byte`, `end_byte`,
  `downloaded_size`, `status`, `error`, `retry_count`, `created_at`, `updated_at`

History is a soft-delete view of `downloads` where `deleted_at IS NOT NULL`, so
removing a download from the list preserves its record.
