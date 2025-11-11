import { Database } from 'bun:sqlite'
import path from 'path'
import fs from 'fs'
import { log } from '../utils/logger.js'

// 데이터 디렉토리 경로 (실행 파일 위치 기준)
const DATA_DIR = process.env.DATA_DIR || path.join(process.cwd(), 'data')
const DB_PATH = path.join(DATA_DIR, 'cohesion.db')

// 데이터 디렉토리 생성
if (!fs.existsSync(DATA_DIR)) {
  fs.mkdirSync(DATA_DIR, { recursive: true })
  log.info(`Created data directory: ${DATA_DIR}`)
}

export const db = new Database(DB_PATH)

// WAL 모드 활성화 (동시성 향상)
db.exec('PRAGMA journal_mode = WAL')
db.exec('PRAGMA foreign_keys = ON') // 외래 키 제약 활성화

log.info(`Database connected at ${DB_PATH}`)

export function initDatabase() {
  // TODO users and auth tables
  log.info('Database initialized')
}

export function closeDatabase() {
  db.close()
  log.info('Database connection closed')
}