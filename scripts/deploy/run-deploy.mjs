import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import {
  existsSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'
import { createDeployReport } from './deploy-report.mjs'
import { recordDeployHistory } from './deploy-record.mjs'

export async function runDeployment(
  executable,
  args,
  env,
  report,
  logError = console.error,
) {
  let interrupted = 0
  let spawnFailed = false
  const child = spawn(executable, args, {
    stdio: 'inherit',
    shell: false,
    env,
  })
  const interrupt = () => {
    interrupted = 130
    child.kill('SIGINT')
  }
  const terminate = () => {
    interrupted = 143
    child.kill('SIGTERM')
  }
  process.on('SIGINT', interrupt)
  process.on('SIGTERM', terminate)
  try {
    const code = await new Promise((resolve) => {
      child.once('error', (error) => {
        spawnFailed = true
        logError(
          error.code === 'ENOENT'
            ? `Instale ${executable} para executar os scripts de deploy. No Mac: ruby scripts/dev/setup-mac-tools.rb`
            : error.message,
        )
      })
      child.once('close', (status, signal) =>
        resolve(
          interrupted ||
            (spawnFailed ? 1 : (status ?? (signal === 'SIGINT' ? 130 : 1))),
        ),
      )
    })
    report.finish(code)
    return code
  } finally {
    process.off('SIGINT', interrupt)
    process.off('SIGTERM', terminate)
  }
}

// Impede dois deploys simultaneos do mesmo checkout (dois builds disputando
// memoria e duas publicacoes trocando IMAGE_TAG). Trava de processo vivo: se o
// PID registrado nao existe mais, a trava e considerada abandonada e substituida.
export function acquireDeployLock(
  path,
  pid = process.pid,
  isAlive = (candidate) => {
    try {
      process.kill(candidate, 0)
      return true
    } catch (error) {
      return error.code === 'EPERM'
    }
  },
) {
  mkdirSync(dirname(path), { recursive: true })
  for (let attempt = 0; attempt < 2; attempt++) {
    try {
      writeFileSync(path, `${pid}\n`, { flag: 'wx' })
      return () => {
        try {
          if (readFileSync(path, 'utf8').trim() === String(pid)) rmSync(path)
        } catch {
          /* Trava ja removida. */
        }
      }
    } catch (error) {
      if (error.code !== 'EEXIST') throw error
      const owner = Number.parseInt(readFileSync(path, 'utf8'), 10)
      if (Number.isInteger(owner) && owner > 0 && isAlive(owner)) {
        throw new Error(
          `Outro deploy ja esta em execucao (PID ${owner}). Aguarde-o terminar; nada foi iniciado.`,
        )
      }
      rmSync(path, { force: true })
    }
  }
  throw new Error('Nao foi possivel obter a trava de deploy.')
}

export function deploymentEnvironment(env, config, home = homedir()) {
  const result = { ...env }
  if (config.sshUser && !result.OMNI_DEPLOY_USER)
    result.OMNI_DEPLOY_USER = config.sshUser
  if (config.sshKeyPath && !result.OMNI_DEPLOY_KEY_PATH) {
    result.OMNI_DEPLOY_KEY_PATH = config.sshKeyPath.startsWith('~/')
      ? join(home, config.sshKeyPath.slice(2))
      : config.sshKeyPath
  }
  return result
}

const commands = {
  'fast-staging': ['deploy-fast.ps1', '-Environment', 'staging'],
  'fast-prod': [
    'deploy-fast.ps1',
    '-Environment',
    'prod',
    '-BackupDatabase',
    '-DeployOmnichannel',
  ],
  staging: ['staging-up.ps1'],
  'staging-down': ['staging-down.ps1'],
  prod: ['deploy-pull.ps1', '-Environment', 'prod', '-BackupDatabase'],
  promote: ['promote.ps1'],
}

export function deploymentCommand(
  name,
  extra = [],
  platform = process.platform,
) {
  const command = commands[name]
  if (!command) throw new Error(`Comando de deploy desconhecido: ${name}`)
  const [script, ...defaults] = command
  return {
    executable: platform === 'win32' ? 'powershell.exe' : 'pwsh',
    args: [
      '-NoProfile',
      '-ExecutionPolicy',
      'Bypass',
      '-File',
      fileURLToPath(new URL(script, import.meta.url)),
      ...defaults,
      ...extra,
    ],
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    const { executable, args } = deploymentCommand(
      process.argv[2],
      process.argv.slice(3),
    )
    if (
      process.argv.slice(3).some((arg) => ['--help', '-h', '-?'].includes(arg))
    ) {
      console.log('Uso: npm run deploy -- [opcoes PowerShell]')
      console.log('Opcoes comuns: -User <usuario> -KeyPath <chave> -Tag <tag>')
      console.log(
        'Deploy rapido: -Service both|api|web -TargetPlatform linux/amd64',
      )
      console.log(
        'Tempos: resumo no terminal e relatorios em .tmp/deploy/<execucao>/',
      )
      console.log(`Executor: ${executable}. Guia: docs/DEPLOY_VPS.md`)
      process.exit(0)
    }
    const configPath = join(homedir(), '.config', 'omni', 'deploy.json')
    const config = existsSync(configPath)
      ? JSON.parse(readFileSync(configPath, 'utf8'))
      : {}
    for (const key of ['sshUser', 'sshKeyPath']) {
      if (config[key] !== undefined && typeof config[key] !== 'string') {
        throw new Error(`Campo ${key} invalido em ${configPath}`)
      }
    }
    const runId =
      new Date().toISOString().replace(/[:.]/g, '-') + '-' + process.pid
    const directory = fileURLToPath(
      new URL(`../../.tmp/deploy/${runId}/`, import.meta.url),
    )
    const releaseLock = acquireDeployLock(
      fileURLToPath(new URL('../../.tmp/deploy/deploy.lock', import.meta.url)),
    )
    process.once('exit', releaseLock)
    const report = createDeployReport(directory, process.argv[2])
    const env = deploymentEnvironment(process.env, config)
    env.OMNI_DEPLOY_TIMING_FILE = report.eventsFile
    process.exitCode = await runDeployment(executable, args, env, report)
    // Historico de deploys (observabilidade): so avisa em falha, nunca muda o exit code.
    await recordDeployHistory({
      directory,
      env,
    }).catch(() => false)
  } catch (error) {
    console.error(error.message)
    process.exitCode = 1
  }
}
