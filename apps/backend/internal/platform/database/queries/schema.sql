CREATE TABLE IF NOT EXISTS space (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    space_name      TEXT NOT NULL,
    space_desc      TEXT,
    space_path      TEXT NOT NULL,
    icon            TEXT,
    space_category  TEXT,
    quota_bytes     INTEGER,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_user_id TEXT,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_user_id TEXT
);

CREATE TABLE IF NOT EXISTS trash_items (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    space_id      INTEGER NOT NULL,
    original_path TEXT NOT NULL,
    storage_path  TEXT NOT NULL,
    item_name     TEXT NOT NULL,
    is_dir        INTEGER NOT NULL DEFAULT 0,
    item_size     INTEGER NOT NULL DEFAULT 0,
    deleted_by    TEXT NOT NULL,
    deleted_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (space_id, storage_path),
    FOREIGN KEY (space_id) REFERENCES space(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_trash_items_space_deleted_at
    ON trash_items(space_id, deleted_at DESC);

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    nickname      TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'user',
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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

CREATE TABLE IF NOT EXISTS roles (
    name         TEXT PRIMARY KEY,
    description  TEXT,
    is_system    INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS permissions (
    key          TEXT PRIMARY KEY,
    description  TEXT NOT NULL,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_name      TEXT NOT NULL,
    permission_key TEXT NOT NULL,
    created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (role_name, permission_key),
    FOREIGN KEY (role_name) REFERENCES roles(name) ON DELETE CASCADE,
    FOREIGN KEY (permission_key) REFERENCES permissions(key) ON DELETE CASCADE
);

INSERT OR IGNORE INTO roles(name, description, is_system) VALUES
('admin', '관리자', 1),
('user', '일반 사용자', 1);

INSERT OR IGNORE INTO permissions(key, description) VALUES
('account.read', '계정 목록/조회'),
('account.write', '계정 생성/수정/삭제'),
('profile.read', '내 프로필 조회'),
('profile.write', '내 프로필 수정'),
('server.config.read', '서버 설정 조회'),
('server.config.write', '서버 설정 변경/재시작'),
('space.read', 'Space 조회'),
('space.write', 'Space 생성/삭제'),
('file.read', '파일 조회/다운로드'),
('file.write', '파일 업로드/수정/이동/삭제');

INSERT OR IGNORE INTO role_permissions(role_name, permission_key) VALUES
('admin', 'account.read'),
('admin', 'account.write'),
('admin', 'profile.read'),
('admin', 'profile.write'),
('admin', 'server.config.read'),
('admin', 'server.config.write'),
('admin', 'space.read'),
('admin', 'space.write'),
('admin', 'file.read'),
('admin', 'file.write'),
('user', 'profile.read'),
('user', 'profile.write'),
('user', 'space.read'),
('user', 'file.read'),
('user', 'file.write');
