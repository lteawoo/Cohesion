# Cohesion

Cohesion is a self-hosted file hub for your PC or server.

Use it to browse files in the browser, share them through WebDAV or SFTP, and control access with spaces, accounts, roles, and permissions.

## Why Cohesion

- Keep your files under your control
- Manage files from a clean browser-based interface
- Connect with existing clients through WebDAV
- Add SFTP access when you need it
- Separate access by space, user, role, and permission

## What You Can Do

- Create space-based file areas
- Upload, download, copy, move, rename, and delete files
- Create folders and manage large file collections
- Switch between grid and table views
- Download multiple files as a ZIP
- View image thumbnails and file-type icons
- Manage accounts, roles, and permissions from the web UI

## Ways to Connect

- Web UI: `http://<host>:<port>`
- WebDAV: `http://<host>:<port>/dav`
- SFTP: `<host>:<sftp_port>` when enabled

## Quick Start

### Install with Homebrew

```bash
brew install lteawoo/cohesion/cohesion
brew services start cohesion
```

Then open `http://localhost:3000`.

### Or Download a Release

1. Download the latest package from [GitHub Releases](https://github.com/lteawoo/Cohesion/releases).
2. Extract the archive.
3. Run `cohesion` on macOS/Linux or `cohesion.exe` on Windows.
4. Open `http://localhost:3000` in your browser.

## Updating

### Homebrew Install

Use Homebrew to update Cohesion:

```bash
brew update
brew upgrade cohesion
```

Homebrew installs do not support in-app self-update.
In the app, update guidance appears in `Settings > About`.

### Direct Release Install

- macOS: Download and reinstall the latest release package from [GitHub Releases](https://github.com/lteawoo/Cohesion/releases).
- Linux: Use the in-app update flow when available, or replace the binary with the latest release package.
- Windows: Use the in-app update flow when available, or replace the binary with the latest release package.

## First Time Setup

1. Open Cohesion in your browser.
2. Complete the initial admin setup.
3. Create a space for your files.
4. Upload files or connect your WebDAV or SFTP client.

## Good Fit For

- Personal file hubs at home
- Small teams that want self-hosted file access
- Users who want browser access and client-based sync side by side
- Simple internal sharing with clear permission boundaries

## License

Cohesion is licensed under `GNU Affero General Public License v3.0 (AGPL-3.0-only)`.
