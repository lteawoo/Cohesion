CREATE TABLE IF NOT EXISTS space (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    space_name      TEXT NOT NULL,
    space_desc      TEXT,
    space_path      TEXT NOT NULL,
    icon            TEXT,
    space_category  TEXT,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_user_id TEXT,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_user_id TEXT
);
