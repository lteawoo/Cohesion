CREATE TABLE IF NOT EXISTS file_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    owner INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    space_id INTEGER NOT NULL REFERENCES space(id) ON DELETE CASCADE,
    root TEXT NOT NULL,
    folder TEXT NOT NULL,
    title TEXT NOT NULL,
    expires INTEGER NOT NULL,
    byte_limit INTEGER NOT NULL CHECK(byte_limit > 0),
    received INTEGER NOT NULL DEFAULT 0,
    received_files INTEGER NOT NULL DEFAULT 0,
    closed INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS file_requests_folder ON file_requests(owner,space_id,folder);
CREATE TABLE IF NOT EXISTS file_request_uploads (
    upload_id TEXT PRIMARY KEY REFERENCES upload_sessions(id) ON DELETE CASCADE,
    request_id INTEGER NOT NULL,
    size INTEGER NOT NULL CHECK(size >= 0)
);
CREATE INDEX IF NOT EXISTS file_request_uploads_request ON file_request_uploads(request_id);
