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

## Resume a browser upload

Files larger than 8 MiB upload in 4 MiB chunks. If your connection drops or you reload the page, reconnect and open the transfer center in the same browser tab. Choose **Resume upload** and select the original file. Cohesion checks its name, size and modification time, then continues from the last chunk the server confirmed. A session expires after 24 hours without a successful chunk. Choose **Cancel** to release it sooner.

## Collect files with a request link

1. Open the destination folder with a space account that can upload files. Choose **File request** (the link icon).
2. Set the expiry in days and total size limit in MiB. Create the link and copy the entire address, including the part after `#`. For people outside your network, open Cohesion at the HTTPS address they can reach before copying.
3. The recipient opens the link and selects a file on their phone or computer. They can submit without an account and see only their own transfer. Files with the same name get a numbered suffix.
4. After an interruption or reload, select **Resume upload** in the same tab and choose the original file. The server retains partial uploads for 24 hours after the last accepted chunk, up to the expiry or closure of the link.
5. Reopen **File request** in the folder to see received bytes or close the link. Closing stops pending submissions and keeps previously received files.

The destination filesystem must support hard links (for example APFS, ext4 or NTFS).

## Share a file for download

1. Sign in as a space manager with file read access. Open the file's menu, choose **Create download link**, and set an expiry (1–365 days).
2. Create the link and copy the full address, including the part after `#`. Use the HTTPS address your recipient can reach. Anyone with the link can download the shared copy until access ends.
3. The recipient opens the link on their phone or computer, checks the filename and size, and chooses **Download**. No account is required.
4. Choose **Manage download links** in the space's toolbar to close a link. You can still close it after moving or deleting the original. Expiry and closure stop new downloads. Removing the issuer's sharing permission also blocks access.

Each link stores a private copy on the server. Editing or replacing the source file leaves that copy at the version you shared. Create a new link to send an updated version.

## Good Fit For

- Personal file hubs at home
- Small teams that want self-hosted file access
- Users who want browser access and client-based sync side by side
- Simple internal sharing with clear permission boundaries

## License

Cohesion is licensed under `GNU Affero General Public License v3.0 (AGPL-3.0-only)`.
