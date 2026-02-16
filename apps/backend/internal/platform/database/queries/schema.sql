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

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    nickname      TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'user',
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (role IN ('admin', 'user'))
);

CREATE TABLE IF NOT EXISTS user_space_permissions (
    user_id     INTEGER NOT NULL,
    space_id    INTEGER NOT NULL,
    permission  TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, space_id),
    CHECK (permission IN ('read', 'write', 'manage')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (space_id) REFERENCES space(id) ON DELETE CASCADE
);
