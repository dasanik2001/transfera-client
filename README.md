// Uses Socket Programming
// Uses HTTP library


## Server (C++ API)

Build and run:

```bash
cd server
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j
./scripts/run.sh
```

`run.sh` stops any old process on port 8080 before starting (multiple stale servers will break uploads).

If port 8080 is stuck: `lsof -ti :8080 | xargs kill -9`

Default port is **8080** (override with `PORT=9000 ./scripts/run.sh`).

### API endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Health check |
| POST | `/api/upload` | Multipart: `file` (required), `maxDownloads` (optional, 1–100, default **1**) → `{ "port", "maxDownloads" }` |
| GET | `/api/download/:port` | P2P transfer; response includes `X-Downloads-Remaining` |

CORS is enabled for browser requests from the frontend.

### Product behavior (summary)

| Feature | Detail |
|---------|--------|
| **P2P download** | Zero-RAM chunked streaming via `set_content_provider` (64 KB buffer); supports HTTP Range Requests (`206 Partial Content`) for resumable downloads |
| **Max downloads** | Uploader sets N (default 1); invite removed after N successful downloads |
| **Upload limits** | UI: max **100 MB**, **all file types** (audio, video, archives, documents) |
| **Logging** | `[INFO]` / `[ERROR]` to stdout; `TRANSFERA_LOG_HTTP`, `TRANSFERA_LOG_VERBOSE` |

See **[client/public/manual/features.html](client/public/manual/features.html)** (deployed with the app).

## Client (Next.js)

```bash
cd client
cp .env.local.example .env.local   # optional; defaults to http://127.0.0.1:8080
npm install
npm run dev
```

1. Start the C++ server (`./scripts/run.sh`).
2. Start the client (`npm run dev` → http://localhost:3000).
3. Set **Max downloads (P2P)** on the Share tab (default 1), upload, share invite code, download until quota is used.

Production default API: **https://transfera-api.onrender.com** (Render). See **[docs/render-deploy.md](docs/render-deploy.md)**.

```bash
curl https://transfera-api.onrender.com/api/health
```

## CLI (Go)

A fast command-line client for file sharing — transfer photos, videos, and documents between laptops and desktops with ~128KB memory usage. Connects to the official cloud server (`https://transfera-api.onrender.com`) by default.

```bash
cd CLI_go
go mod tidy
go build -o transfera.exe .   # Windows
go build -o transfera .       # Linux/macOS
```

**Upload** a file (e.g. photo from your laptop) and get an invite code:
```bash
./transfera upload photo.jpg
./transfera upload video.mp4 --max-downloads 5
```

**Download** on another machine (e.g. desktop) using the invite code:
```bash
./transfera download 52341
./transfera download 52341 -o ~/Pictures/
```

**Health check**:
```bash
./transfera health
```

**Collaborative Rooms** (Create or Join multi-user room for group file sharing and chat):
```bash
./transfera room create --name Alice -i   # Create & enter live room
./transfera room join 52341 --name Bob -i  # Join & enter live room
```

Or run `./transfera` with no arguments for the interactive terminal menu!

See **[cli/README.md](cli/README.md)** for full documentation.

## Deploy on Linode (optional self-host)

See **[docs/linode-deploy.md](docs/linode-deploy.md)** — client and C++ API on separate ports.

## Technical manual (GitHub Pages)

Full documentation (features, P2P, threads, operations). **Mobile:** use the ☰ Menu button to open the sidebar.

- **Manual:** https://dasanik2001.github.io/transfera-client/manual/
- **Live app:** https://dasanik2001.github.io/transfera-client/
- **Features chapter:** https://dasanik2001.github.io/transfera-client/manual/features.html

Source: `client/public/manual/` (deployed with the client static build).
