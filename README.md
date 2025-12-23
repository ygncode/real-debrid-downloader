# RD Downloader - Real-Debrid Movie Downloader

A web application that lists movies from a directory and allows downloading new movies via Real-Debrid API with automatic subtitle fetching.

## Features

- List movies from a specified directory
- Add torrents via magnet links or .torrent files
- Select which files to download from torrents
- Real-time download progress tracking via SSE
- Automatic subtitle download using Subliminal CLI
- Clean, cinematic dark theme UI
- Password protection (optional)
- Delete files from collection

## Installation

### Prerequisites

- Go 1.21 or later
- Real-Debrid account and API key
- (Optional) Subliminal for subtitles: `pip install subliminal`

### Build from source

```bash
git clone https://github.com/ygncode/real-debrid-downloader.git
cd real-debrid-downloader
make build
```

### Download release

Download the latest release for your platform from the [Releases](https://github.com/ygncode/real-debrid-downloader/releases) page.

## Usage

```bash
# Using the binary
./bin/rd-downloader --path=/path/to/movies --api-key=YOUR_API_KEY

# Or with environment variable
export REALDEBRID_API_KEY=YOUR_API_KEY
./bin/rd-downloader --path=/path/to/movies

# With custom port
./bin/rd-downloader --path=/path/to/movies --api-key=YOUR_API_KEY --port=3000

# With password protection
./bin/rd-downloader --path=/path/to/movies --api-key=YOUR_API_KEY --password=MySecurePass123

# With custom subliminal path
./bin/rd-downloader --path=/path/to/movies --api-key=YOUR_API_KEY --subliminal-path=/home/user/miniconda3/bin/subliminal
```

### Command Line Options

| Flag | Description | Default |
|------|-------------|---------|
| `--path`, `-p` | Path to movies directory (required) | - |
| `--api-key` | Real-Debrid API key | `$REALDEBRID_API_KEY` |
| `--port` | Web server port | 8080 |
| `--password` | Password to protect web interface | - |
| `--subliminal-path` | Custom path to subliminal binary | auto-detect |

## How It Works

1. **Add Torrent**: Paste a magnet link or upload a .torrent file
2. **Select Files**: Choose which files from the torrent to download
3. **Download**: Real-Debrid processes the torrent, then files are downloaded to your movies folder
4. **Subtitles**: English subtitles are automatically downloaded for video files (optional)

## Tech Stack

- **Backend**: Go with Gin framework
- **Frontend**: htmx + vanilla JavaScript
- **Database**: SQLite (via GORM)
- **Styling**: Custom CSS with cinematic dark theme

## Development

```bash
# Install dependencies
make deps

# Run in development mode
make dev PATH=/path/to/movies API_KEY=YOUR_KEY

# Build
make build

# Build for all platforms
make build-all
```

## License

MIT
