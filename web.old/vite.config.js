import { defineConfig } from 'vite'
import path from 'path'
import fs from 'fs'
import { fileURLToPath } from 'url'

// __dirname not available in ESM; derive from import.meta.url.
const __dirname = path.dirname(fileURLToPath(import.meta.url))

// SDK lives in erp/web/sdk/ — serve it at /sdk/* in dev mode.
// Production: copy erp/web/sdk/ → awo/web/public/sdk/ before build.
const sdkDir = path.resolve(__dirname, '../../web/sdk')

// Locale files live in erp/web/utils/ — served at /locales/* in dev mode.
// Production: copy erp/web/utils/locale-en.js → awo/web/public/locales/en.js before build.
const utilsDir = path.resolve(__dirname, '../../web/utils')

function serveStaticFile(filePath, res, next) {
  if (!fs.existsSync(filePath)) { next(); return; }
  var ext = path.extname(filePath)
  var mime = ext === '.css' ? 'text/css'
           : ext === '.js'  ? 'application/javascript'
           : 'application/octet-stream'
  res.setHeader('Content-Type', mime)
  res.setHeader('Cache-Control', 'public, max-age=86400')
  fs.createReadStream(filePath).pipe(res)
}

function sdkDevPlugin() {
  return {
    name: 'awo-sdk',
    configureServer(server) {
      server.middlewares.use('/sdk', function (req, res, next) {
        serveStaticFile(path.join(sdkDir, req.url), res, next)
      })
      // /locales/en.js → erp/web/utils/locale-en.js
      server.middlewares.use('/locales', function (req, res, next) {
        var name = req.url.replace(/^\//, '') // e.g. "en.js"
        serveStaticFile(path.join(utilsDir, 'locale-' + name), res, next)
      })
    },
  }
}

export default defineConfig({
  root: '.',
  publicDir: 'public',
  plugins: [sdkDevPlugin()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
