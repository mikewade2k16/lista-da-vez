// Build de producao do Nuxt fora do Docker Desktop, para o deploy rapido.
//
// Por que: dentro da VM do Docker Desktop (~6 GB) o build levava ~6,5 min e
// exigia pausar a stack local; nativo no host leva ~1,5 min. O `.output` do
// Nitro e JS puro (o smoke da imagem recusa modulos `.node`), entao o artefato
// compilado no host roda igual na imagem linux/amd64.
//
// Garantias de paridade com web/Dockerfile:
// - Node do host identico ao da imagem (lido de web/Dockerfile.prebuilt);
// - diretorio isolado em .tmp/web-build: nao toca o .nuxt/.output do dev;
// - dependencias por `npm ci` a partir do package-lock (reinstala quando o
//   lock, o Node ou a plataforma mudam), nunca o node_modules do dev;
// - mesmas exclusoes do web/.dockerignore (inclui .env*), e variaveis
//   NUXT_/NITRO_/VITE_/NODE_ do shell nao vazam para o build.
//
// Uso: node scripts/deploy/build-web-native.mjs [--heap-mb 6144]
// Saida: imprime na ultima linha o caminho absoluto da pasta .output.
import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { cpSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const repoDir = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..')
const webDir = join(repoDir, 'web')
const stageDir = join(repoDir, '.tmp', 'web-build')
const lockMarker = join(stageDir, 'node_modules', '.omni-lock-hash')

const heapIndex = process.argv.indexOf('--heap-mb')
const heapMb = heapIndex > -1 ? Number(process.argv[heapIndex + 1]) : 6144
if (!Number.isInteger(heapMb) || heapMb < 4096 || heapMb > 16384) {
  throw new Error('--heap-mb deve ser inteiro entre 4096 e 16384.')
}

// 1. Node do host precisa ser o mesmo da imagem de runtime.
const prebuilt = readFileSync(join(webDir, 'Dockerfile.prebuilt'), 'utf8')
const expectedNode = prebuilt.match(/^FROM\s+node:(\d+\.\d+\.\d+)/m)?.[1]
if (!expectedNode) throw new Error('Versao do Node nao encontrada em web/Dockerfile.prebuilt.')
if (process.versions.node !== expectedNode) {
  throw new Error(
    `Node do host ${process.versions.node} difere da imagem (${expectedNode}). ` +
      'Instale a mesma versao ou use -WebBuildMode docker.',
  )
}

// 2. Copia fontes para o diretorio isolado, preservando apenas node_modules.
const excludedNames = new Set(['node_modules', '.nuxt', '.output', 'dist', '.git', '.gitignore', 'coverage', '.DS_Store'])
const isExcluded = (name) => excludedNames.has(name) || name.startsWith('._') || name.startsWith('.env') || name.endsWith('.log')

mkdirSync(stageDir, { recursive: true })
for (const entry of readdirSync(stageDir)) {
  if (entry !== 'node_modules') rmSync(join(stageDir, entry), { recursive: true, force: true })
}
for (const entry of readdirSync(webDir)) {
  if (isExcluded(entry)) continue
  cpSync(join(webDir, entry), join(stageDir, entry), {
    recursive: true,
    filter: (source) => !isExcluded(source.split(/[\\/]/).pop()),
  })
}

// Ambiente limpo: o build no Docker nao ve variaveis do shell do desenvolvedor.
const buildEnv = Object.fromEntries(
  Object.entries(process.env).filter(([key]) => !/^(NUXT_|NITRO_|VITE_|NODE_|npm_|PORT$|HOST$)/i.test(key)),
)
const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm'
function run(args, extraEnv) {
  const result = spawnSync(npm, args, {
    cwd: stageDir,
    stdio: ['ignore', 'inherit', 'inherit'],
    env: { ...buildEnv, ...extraEnv },
    shell: process.platform === 'win32',
  })
  if (result.status !== 0) throw new Error(`npm ${args.join(' ')} falhou (exit ${result.status ?? result.signal}).`)
}

// 3. Dependencias reproduziveis a partir do lock.
const lockHash = createHash('sha256')
  .update(readFileSync(join(stageDir, 'package-lock.json')))
  .update(`${process.versions.node}|${process.platform}|${process.arch}`)
  .digest('hex')
const installedHash = existsSync(lockMarker) ? readFileSync(lockMarker, 'utf8').trim() : ''
if (installedHash !== lockHash) {
  console.log('==> npm ci no diretorio de build (lock, Node ou plataforma mudaram)')
  run(['ci', '--no-audit', '--no-fund'], { NODE_ENV: 'development' })
  writeFileSync(lockMarker, `${lockHash}\n`)
} else {
  console.log('==> Dependencias do build reaproveitadas (package-lock inalterado)')
}

// 4. Build de producao com as mesmas variaveis do web/Dockerfile.
run(['run', 'build'], { NODE_ENV: 'production', NODE_OPTIONS: `--max-old-space-size=${heapMb}` })

const outputDir = join(stageDir, '.output')
if (!existsSync(join(outputDir, 'server', 'index.mjs'))) {
  throw new Error('Build terminou sem .output/server/index.mjs.')
}
console.log(outputDir)
