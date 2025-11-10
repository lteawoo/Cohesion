import Database from 'better-sqlite3'
import path from 'path'
import { fileURLToPath } from 'url'
import fs from 'fs'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

// 데이터 디렉토리 경로
const DATA_DIR = path.join(__dirname, '../../data')
const DB_PATH = path.join(DATA_DIR, 'cohesion.db')

// 데이터 디렉토리 생성
if (!fs.existsSync(DATA_DIR)) {
  fs.mkdirSync(DATA_DIR, { recursive: true })
}

export const db = new Database(DB_PATH)

// WAL 모드 활성화 (동시성 향상)
db.pragma('journal_mode = WAL')
db.pragma('foreign_keys = ON') // 외래 키 제약 활성화

console.log('Database connected at', DB_PATH)

export function initDatabase() {
  // TODO users and auth tables
}

export function closeDatabase() {
  db.close()
}