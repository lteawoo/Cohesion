import pino from 'pino'
import path from 'path'
import fs from 'fs'

// 로그 디렉토리 경로
const LOG_DIR = process.env.LOG_DIR || path.join(process.cwd(), 'logs')

// 로그 디렉토리 생성
if (!fs.existsSync(LOG_DIR)) {
  fs.mkdirSync(LOG_DIR, { recursive: true })
}

// 로그 파일 경로 (날짜별)
const today = new Date().toISOString().split('T')[0]
const LOG_FILE = path.join(LOG_DIR, `cohesion-${today}.log`)

// 로그 파일 스트림 생성
const logFileStream = fs.createWriteStream(LOG_FILE, { flags: 'a' })

// 개발 환경 여부 확인
// 단독 실행 파일이거나 프로덕션 환경이면 pino-pretty 비활성화
const isStandalone = Bun.main.includes('~BUN') || Bun.main.endsWith('.exe')
const isDev = process.env.NODE_ENV !== 'production' && !isStandalone

// Pino 로거 설정
export const logger = isDev
  ? // 개발 환경: pino-pretty 사용
    pino(
      {
        level: process.env.LOG_LEVEL || 'info',
        transport: {
          target: 'pino-pretty',
          options: {
            colorize: true,
            translateTime: 'HH:MM:ss',
            ignore: 'pid,hostname',
          },
        },
      },
      pino.multistream([
        { stream: process.stdout },
        { stream: logFileStream },
      ])
    )
  : // 프로덕션/단독 실행 파일: 심플한 로깅
    pino(
      {
        level: process.env.LOG_LEVEL || 'info',
        timestamp: pino.stdTimeFunctions.isoTime,
      },
      pino.multistream([
        { stream: process.stdout },
        { stream: logFileStream },
      ])
    )

// 편의 함수들
export const log = {
  info: (msg: string, ...args: any[]) => logger.info(msg, ...args),
  error: (msg: string, ...args: any[]) => logger.error(msg, ...args),
  warn: (msg: string, ...args: any[]) => logger.warn(msg, ...args),
  debug: (msg: string, ...args: any[]) => logger.debug(msg, ...args),
}

// 로거 시작 메시지
logger.info(`Logger initialized. Log file: ${LOG_FILE}`)
