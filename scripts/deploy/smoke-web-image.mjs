import { spawn } from 'node:child_process'
import { readdirSync } from 'node:fs'
import { join } from 'node:path'

function nativeModules(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    return entry.isDirectory() ? nativeModules(path) : entry.name.endsWith('.node') ? [path] : []
  })
}

if (nativeModules('.output').length) throw new Error('Bundle contem modulos nativos; revisar compatibilidade da arquitetura antes de publicar.')
const server = spawn(process.execPath, ['.output/server/index.mjs'], {
  env: { ...process.env, PORT: '13003', NITRO_PORT: '13003', HOST: '127.0.0.1', NITRO_HOST: '127.0.0.1' },
  stdio: 'ignore',
})
try {
  let ready = false
  for (let attempt = 0; attempt < 100; attempt++) {
    if (server.exitCode !== null) throw new Error(`Servidor encerrou antes de responder: ${server.exitCode}`)
    try {
      const response = await fetch('http://127.0.0.1:13003', { signal: AbortSignal.timeout(1000) })
      if (response.status === 200) { ready = true; break }
    } catch { /* Aguarda apenas a inicializacao do servidor isolado. */ }
    await new Promise((resolve) => setTimeout(resolve, 100))
  }
  if (!ready) throw new Error('A imagem web nao respondeu HTTP 200.')
  console.log(`Smoke da imagem web OK: ${process.arch}, HTTP 200, bundle sem modulos nativos.`)
} finally { server.kill('SIGTERM') }
