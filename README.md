# Lunefetch

Multi-threaded download manager with TUI interface and browser integration, built with Go.

## Features

- Multi-part parallel downloading with configurable chunks
- Resume capability for interrupted downloads
- Dynamic chunk calculation based on file size:
  - < 1 GB: 4 chunks
  - 1 GB – 5 GB: 8 chunks
  - 5 GB – 100 GB: 16 chunks
  - > 100 GB: 32 chunks
- SQLite-based state persistence
- Terminal UI with real-time progress tracking
- Browser integration via Firefox/Zen extension (native messaging)

## Installation

### Prerequisites

- Go 1.22 or higher

### Build

```bash
go build -o lunefetch
```

### Browser Extension

Install the native messaging host and load the Firefox extension:

```bash
chmod +x install.sh
./install.sh
```

Then in Firefox/Zen:
1. Open `about:debugging` → This Firefox
2. Click **Load Temporary Add-on**
3. Select `extension/manifest.json`

## Usage

### Start TUI

```bash
./lunefetch
```

### Direct download

```bash
./lunefetch -u <url> [-c chunks] [-o output_dir]
```

### List downloads

```bash
./lunefetch --list
```

### Resume download

```bash
./lunefetch --resume <id>
```

### TUI keybindings

| Key     | Action         |
|---------|----------------|
| `n`     | New download   |
| `r`     | Resume         |
| `p`     | Pause          |
| `d`     | Delete         |
| `enter` | Detail view    |
| `esc`   | Back           |
| `q`     | Quit           |

## Configuration

Config file: `~/.config/lunefetch/config.yaml`

```yaml
download_dir: ~/Downloads
max_retries: 3
timeout: 30
chunk_rules:
  small: 4     # < 1 GB
  medium: 8    # 1 GB - 5 GB
  large: 16    # 5 GB - 100 GB
  xlarge: 32   # > 100 GB
small_size: 1073741824    # 1 GB
medium_size: 5368709120   # 5 GB
large_size: 107374182400  # 100 GB
```

## Architecture

```
lunefetch/
├── cmd/
│   └── native-host/     # Firefox native messaging host
├── extension/           # Firefox/Zen WebExtension
├── internal/
│   ├── api/             # Local HTTP server (browser integration)
│   ├── core/            # Download engine
│   ├── storage/         # SQLite state management
│   ├── config/          # Configuration
│   └── ui/              # TUI (bubbletea)
└── db/
    └── migrations/      # SQLite schema
```

## License

MIT
