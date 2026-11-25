CREATE TABLE IF NOT EXISTS download_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    snapshot TEXT NOT NULL UNIQUE,
    owner INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    space_id INTEGER NOT NULL REFERENCES space(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    size INTEGER NOT NULL CHECK(size >= 0),
    expires INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS download_links_space_path ON download_links(space_id, path);
CREATE INDEX IF NOT EXISTS download_links_expires ON download_links(expires);
