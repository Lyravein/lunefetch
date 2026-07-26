# Lunefetch

A multi-threaded download manager with TUI interface, built with Go.

## Features

- Multi-part parallel downloading with configurable chunks
- Resume capability for interrupted downloads
- Dynamic chunk calculation based on file size:
  - < 1GB: 4 chunks
  - 1GB - 5GB: 8 chunks
  - > 5GB: 16 chunks
- SQLite-based state persistence
- Terminal UI with real-time progress tracking

## Installation

### Prerequisites

- Go 1.22 or higher

### Build

```bash
go build -o download-manager
```

## Usage

### Start TUI

```bash
./download-manager
```

### Direct download

```bash
./download-manager -u <url> [-c chunks] [-o output_dir]
```

### List downloads

```bash
./download-manager --list
```

### Resume download

```bash
./download-manager --resume <id>
```

## Configuration

Config file location: `~/.config/download-manager/config.yaml`

Default configuration:

```yaml
download_dir: ~/Downloads
max_retries: 3
timeout: 30
chunk_rules:
  small: 4    # < 1GB
  medium: 8   # 1GB - 5GB
  large: 16   # > 5GB
```

## Architecture

```
download-manager/
├── internal/
│   ├── core/          # Download engine
│   ├── storage/       # Database & file operations
│   ├── config/        # Configuration management
│   └── ui/            # TUI implementation
└── db/
    └── migrations/    # SQLite schema
```

## License

MIT
