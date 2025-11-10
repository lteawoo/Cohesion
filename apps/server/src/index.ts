import { serve } from '@hono/node-server'
import { Hono } from 'hono'
import { closeDatabase } from './db/index.js'

const app = new Hono()

app.get('/', (c) => {
  return c.text('Hello Hono!')
})

const port = 3000

const server = serve({
  fetch: app.fetch,
  port
}, (info) => {
  console.log(`Server is running on http://localhost:${info.port}`)
})

// Graceful shutdown
process.on('SIGINT', () => {
  console.log('[SIGINT]Shutting down server...')
  closeDatabase()
  server.close()
  process.exit(0)
})
process.on('SIGTERM', () => {
  console.log('[SIGTERM]Shutting down server...')
  closeDatabase()
  server.close((err) => {
    if (err) {
      console.error(err)
      process.exit(1)
    }
    process.exit(0)
  })
})
