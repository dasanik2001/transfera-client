# Transfera CLI (Go)

A command-line client for [Transfera](../README.md) — secure P2P file sharing, optimized for large file transfers (100MB+) with minimal memory usage.

## Quick Start

### Prerequisites

- [Go 1.21+](https://go.dev/dl/) installed
- Connects out-of-the-box to the official production server (`https://transfera-api.onrender.com`). No local server setup required!

### Install & Build

```bash
cd CLI_go

# Download dependencies
go mod tidy

# Build the binary
go build -o transfera.exe .     # Windows
go build -o transfera .         # Linux/macOS
```

### Usage (Cross-Device Transfer, e.g. Laptop to Desktop)

1. **On your Laptop** (Upload a photo):
   ```bash
   ./transfera upload my_photo.jpg
   ```
   Output:
   ```
   ✓ File ready to share!
   ┌────────────────────────────────────────
   │  Invite code: 55228
   │  Max downloads: 1
   └────────────────────────────────────────
   ```

2. **On your Desktop** (Download the photo):
   ```bash
   ./transfera download 55228
   # Or specify an output directory
   ./transfera download 55228 -o ~/Pictures/
   ```

3. **Check Server Health**:
   ```bash
   ./transfera health
   ```

## Commands

### `transfera health`

Check if the API server is reachable.

```bash
transfera health
transfera --api http://127.0.0.1:8080 health  # to check a local dev server
```

### `transfera upload <file>`

Upload a file (photo, video, archive, document) and receive an invite code to share.

```bash
transfera upload vacation.jpg                  # Basic photo upload
transfera upload video.mp4 -n 5               # Allow 5 downloads
transfera upload huge.zip --max-size 500       # Allow files up to 500MB
```

**Flags:**
| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--max-downloads` | `-n` | 1 | Maximum downloads allowed (1-100) |
| `--max-size` | `-s` | 100 | Max file size in MB |

### `transfera download <invite-code>`

Download a file using an invite code shared with you.

```bash
transfera download 52341                       # Download to current directory
transfera download 52341 -o ./received/        # Download to specific directory (creates it if missing)
transfera download 52341 --output-name doc.pdf # Override filename
```

**Flags:**
| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | `.` | Output directory (automatically created if it doesn't exist) |
| `--output-name` | | | Override download filename |

### `transfera room` (Collaborative Multi-User Rooms)

Create or join collaborative multi-user rooms for group file sharing and live chat.

#### 1. Interactive Terminal UI
Run `transfera` with no arguments to launch the guided interactive menu, then select **Option 3: 👥 Collaborative Rooms (Create/Join)** to create or join a room interactively.

#### 2. Create a Room
```bash
transfera room create --name Alice --max 5
transfera room create --name "Dev Team" --max 10 --port 54321

# Create and enter the live interactive session immediately:
transfera room create --name Alice -i
```

**Flags:**
| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--name` | | `Host` | Display name of the room creator |
| `--max` | | `5` | Maximum participants allowed (2–100) |
| `--port` | | `0` (auto) | Custom room ID / port (49152–65535) |
| `--interactive` | `-i` | `false` | Enter interactive collaboration session immediately |

#### 3. Join a Room
```bash
transfera room join 52341 --name Bob

# Join and enter the live interactive session immediately:
transfera room join 52341 --name Bob -i
```

#### 4. Connect to an Active Room Session
```bash
transfera room session 52341 --user usr_abc123 --name Alice
```

#### 5. In-Room Commands (Interactive Session)
When inside an interactive room session, you can chat by typing regular text, or run slash commands:
- `/upload <file...>`: Share one or more files in the room with upload progress
- `/files`: List all shared files with file IDs, sizes, and uploaders
- `/download <file-id> [dir]`: Download a shared file to a directory
- `/members`: List all members currently in the room (with Host indicator)
- `/kick <user-id>`: Remove a member from the room (Host only)
- `/clear`: Clear terminal screen and redraw room header
- `/leave`: Gracefully leave the room and exit
- `/help`: Display in-room command reference

#### 6. Scriptable Subcommands
- **Send chat message**: `transfera room message 52341 "Hello team!" --user usr_abc123`
- **Upload files**: `transfera room upload 52341 doc.pdf archive.zip --user usr_abc123 --note "Project files"`
- **Download file**: `transfera room download 52341 fil_a1b2c3 -o ./downloads/`
- **Sync / view status**: `transfera room sync 52341`
- **Kick participant**: `transfera room remove 52341 usr_target --user usr_host`
- **Leave room**: `transfera room leave 52341 --user usr_abc123`

### `transfera install`

Install the CLI binary globally to your system so you can run `transfera` from any terminal.

```bash
transfera install           # Install globally to user PATH
transfera install --remove  # Uninstall and remove from PATH
```

- **Linux / macOS**: Copies binary to `~/.local/bin/transfera` and configures your shell (`.zshrc`, `.bashrc`, etc.)
- **Windows**: Copies binary to `%LOCALAPPDATA%\Programs\Transfera\transfera.exe` and updates User PATH via Registry

## Global Flags & Environment Variables

Available on all commands:

| Flag | Short | Default / Fallback | Description |
|------|-------|--------------------|-------------|
| `--api` | `-a` | `https://transfera-api.onrender.com` | API server URL (can also be set via `TRANSFERA_API_URL` or `NEXT_PUBLIC_API_BASE_URL`) |
| `--verbose` | `-V` | `false` | Enable verbose output (shows HTTP headers, timing) |

## Cross-Compilation

Build for any platform from any platform:

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o transfera-linux .

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o transfera-macos .

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o transfera-macos-arm .

# Windows
GOOS=windows GOARCH=amd64 go build -o transfera.exe .
```

## Architecture

```
CLI_go/
├── main.go                      # Entry point (calls cmd.Execute())
├── cmd/
│   ├── root.go                  # Root command, global flags, TUI trigger
│   ├── health.go                # Health check command
│   ├── upload.go                # Upload command
│   ├── download.go              # Download command
│   └── install.go               # Global PATH installer command
├── internal/
│   ├── api/
│   │   └── client.go            # HTTP client (upload, download, health)
│   ├── installer/
│   │   ├── installer.go         # Shared installer interface & helpers
│   │   ├── installer_unix.go    # Linux/macOS PATH installer (~/.local/bin)
│   │   └── installer_windows.go # Windows PATH installer (Registry)
│   ├── progress/
│   │   └── bar.go               # Terminal progress bars
│   ├── tui/
│   │   └── menu.go              # Interactive terminal menu interface
│   └── validation/
│       └── file.go              # File validation & path resolution
├── go.mod                       # Go module definition
└── README.md                    # This file
```

## Memory Usage

The CLI uses streaming (io.Pipe) for uploads and downloads. Memory usage stays flat regardless of file size:

| File Size | CLI Memory | Browser Memory |
|-----------|-----------|----------------|
| 10 MB     | ~10 MB    | ~40 MB         |
| 100 MB    | ~10 MB    | ~400 MB        |
| 500 MB    | ~10 MB    | ❌ may crash    |
