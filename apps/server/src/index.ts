import { serve } from '@hono/node-server'
import { Hono } from 'hono'
import { closeDatabase } from './db/index.js'
import { log } from './utils/logger.js'

const app = new Hono()

// 요청 로깅 미들웨어
app.use('*', async (c, next) => {
  const start = Date.now()
  await next()
  const ms = Date.now() - start
  log.info(`${c.req.method} ${c.req.path} - ${c.res.status} (${ms}ms)`)
})

app.get('/', (c) => {
  return c.text('Hello Hono!')
})

const port = 3000

const server = serve({
  fetch: app.fetch,
  port
}, (info) => {
  log.info(`Server started on http://localhost:${info.port}`)
})

// Graceful shutdown
process.on('SIGINT', () => {
  log.info('Received SIGINT, shutting down server...')
  closeDatabase()
  server.close()
  process.exit(0)
})
process.on('SIGTERM', () => {
  log.info('Received SIGTERM, shutting down server...')
  closeDatabase()
  server.close((err) => {
    if (err) {
      log.error(`Error during shutdown: ${err.message}`)
      process.exit(1)
    }
    process.exit(0)
  })
})

// 에러 핸들링
process.on('uncaughtException', (err) => {
  log.error(`Uncaught exception: ${err.message}`)
  log.error(err.stack || '')
  process.exit(1)
})

process.on('unhandledRejection', (reason) => {
  log.error(`Unhandled rejection: ${reason}`)
  process.exit(1)
})
