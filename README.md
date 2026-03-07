# Cohesion

Cohesion is a self-hosted file service designed to make managing and sharing files on your PC or server simple.

## Overview

This project is built around the following goals:

- Keep installation and operation simple
- Browse and manage files directly in the browser
- Integrate with existing clients via protocols such as WebDAV and SFTP
- Enforce access control with account/role/permission policies

## Key Features

- Space-based virtual root
- Grid/Table file explorer views
- Upload, download, copy, move, delete, rename, and folder creation
- Multi-select and multi-download (ZIP)
- Image thumbnails and extension-based file icons
- WebDAV Basic Auth with Space permission checks
- Optional SFTP server with account authentication and permission checks
- JWT cookie authentication with initial admin setup flow
- Account/role/permission (RBAC) management UI

## Supported Protocols

- WEB: `http://<host>:<port>` (UI + API)
- WebDAV: `http://<host>:<port>/dav`
- SFTP: `<host>:<sftp_port>` (when enabled)

## Tech Stack

- Backend: Go (`net/http`, SQLite, WebDAV, SFTP)
- Frontend: React 19, Vite, Ant Design, Zustand
- Monorepo: Turborepo, pnpm workspace
- Release build: GoReleaser

## Quick Start

### Requirements

- Node.js `>= 24`
- pnpm `>= 10.24.0`
- Go `1.25.7`

### Install

```bash
pnpm install
```

### Run in Development

```bash
pnpm dev
```

- Frontend: `http://localhost:5173`
- Backend: `http://localhost:3000`
- Default SFTP port: `2222` (when enabled)

## Build

### Build Entire Workspace

```bash
pnpm build
```

### Build Individually

```bash
# frontend
pnpm -C apps/frontend build

# backend
cd apps/backend && go build -o cohesion
```

### Release Build (Artifacts)

```bash
pnpm release:check
pnpm release:snapshot
```

### Supported OS / Architecture (Release Artifacts)

| OS | Architecture | Artifact Format |
| --- | --- | --- |
| macOS | `amd64`, `arm64` | `.tar.gz` |
| Linux | `amd64`, `arm64` | `.tar.gz` |
| Windows | `amd64` | `.zip` |

Release artifacts are published on GitHub Releases:

- https://github.com/lteawoo/Cohesion/releases

### Homebrew Install

```bash
brew install lteawoo/cohesion/cohesion
brew services start cohesion
```

Homebrew installs should be updated with:

```bash
brew upgrade cohesion
```

### Run from Release Artifact

1. Download the artifact that matches your OS/architecture.
2. Extract it.
3. Run the binary.

macOS / Linux:

```bash
tar -xzf cohesion_<version>_<os>_<arch>.tar.gz
cd cohesion_<version>_<os>_<arch>
./cohesion
```

Windows (PowerShell):

```powershell
Expand-Archive .\cohesion_<version>_windows_amd64.zip .
.\cohesion.exe
```

Linux systemd service install:

```bash
tar -xzf cohesion_<version>_linux_<arch>.tar.gz
cd cohesion_<version>_linux_<arch>
sudo ./install.sh --user "$(id -un)"
sudo systemctl status cohesion --no-pager
```

The Linux installer:

- installs the release files into `/opt/cohesion`
- creates `/var/lib/cohesion/config`, `/var/lib/cohesion/data`, and `/var/lib/cohesion/secrets`
- writes runtime files under `/var/lib/cohesion/runtime`
- writes `/etc/systemd/system/cohesion.service`
- enables and starts the service by default

Use `sudo ./install.sh --help` to see optional flags such as `--skip-start`.

Linux native packages:

```bash
# Debian / Ubuntu
sudo dpkg -i ./cohesion_<version>_<arch>.deb
sudo systemctl enable --now cohesion

# RHEL / Fedora
sudo rpm -Uvh ./cohesion-<version>-1.<arch>.rpm
sudo systemctl enable --now cohesion
```

Native packages:

- install the `cohesion` binary into `/usr/bin`
- install the systemd service unit into the distribution-specific systemd directory
- run the service as the `cohesion` system user
- keep config/data/secrets under `/var/lib/cohesion`
- keep runtime files under `/var/lib/cohesion/runtime`

On first production run, Cohesion creates its operational files under the platform default state root when they are missing.

- macOS / Homebrew: `~/.cohesion`
- Windows: `%USERPROFILE%\\.cohesion`
- Linux: `/var/lib/cohesion`
- Config: `<state-root>/config/config.prod.yaml`
- Database: `<state-root>/data/cohesion.db`
- Secrets: `<state-root>/secrets/`
- Windows: the `.cohesion` root directory is marked hidden when the OS supports the hidden attribute.

### Upgrade Notes

- Homebrew installs do not support in-app self-update. Use `brew upgrade cohesion`.
- Linux package installs do not support in-app self-update. Upgrade with a newer `.deb`/`.rpm` package or your system package manager.
- macOS direct-download installs do not support in-app self-update. Reinstall the latest release or switch to the Homebrew install path.
- Linux direct-download installs should be reinstalled with the latest release archive unless the running account can update both the binary location and the configured state root.
- Linux systemd installs do not support in-app self-update. Download the latest release archive again and rerun `sudo ./install.sh --user <service-user>`.
- Stop the running process before replacing the binary.
- If you are upgrading from an older Linux production install that kept files under `~/.cohesion` or next to the binary, move them into `/var/lib/cohesion/` before starting the new build.
- If you are upgrading from an older macOS/Windows production install that kept files next to the binary, move them into `~/.cohesion/` before starting the new build.

## Environment Variables

- `COHESION_STATE_ROOT` (optional)
  - Overrides the production config/data/secrets root directory
  - Useful when Linux deployments need a different operational state root
- `COHESION_JWT_SECRET`
  - Recommended to be at least 32 characters in production
  - If not set, a random value is generated in `<state-root>/secrets/jwt_secret`
- `COHESION_JWT_SECRET_FILE` (optional)
- `COHESION_ADMIN_USER`, `COHESION_ADMIN_PASSWORD`, `COHESION_ADMIN_NICKNAME` (optional)
  - `COHESION_ADMIN_USER` and `COHESION_ADMIN_PASSWORD` must be set together
- `COHESION_SFTP_HOST_KEY_FILE` (optional)

## Security Recommendations

- The default transport is HTTP. For internet exposure, use a reverse proxy (Caddy/Nginx) with TLS.
- WebDAV should be used with both Basic Auth and Space permission checks.

## License

This project is licensed under `GNU Affero General Public License v3.0 (AGPL-3.0-only)`.

- Full text: `LICENSE`
- Reference: https://www.gnu.org/licenses/agpl-3.0.html
