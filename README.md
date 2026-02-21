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

## Environment Variables

- `COHESION_JWT_SECRET`
  - Recommended to be at least 32 characters in production
  - If not set, a random value is generated in a local secret file
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
