import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { existsSync, readFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'

export function deploymentEnvironment(env, config, home = homedir()) {
  const result = { ...env }
  if (config.sshUser && !result.OMNI_DEPLOY_USER) result.OMNI_DEPLOY_USER = config.sshUser
  if (config.sshKeyPath && !result.OMNI_DEPLOY_KEY_PATH) {
    result.OMNI_DEPLOY_KEY_PATH = config.sshKeyPath.startsWith('~/')
      ? join(home, config.sshKeyPath.slice(2))
      : config.sshKeyPath
  }
  return result
}

const commands = {
  'fast-staging': ['deploy-fast.ps1', '-Environment', 'staging'],
  'fast-prod': ['deploy-fast.ps1', '-Environment', 'prod', '-BackupDatabase', '-DeployOmnichannel'],
  staging: ['staging-up.ps1'],
  'staging-down': ['staging-down.ps1'],
  prod: ['deploy-pull.ps1', '-Environment', 'prod', '-BackupDatabase'],
  promote: ['promote.ps1'],
}

export function deploymentCommand(name, extra = [], platform = process.platform) {
  const command = commands[name]
  if (!command) throw new Error(`Comando de deploy desconhecido: ${name}`)
  const [script, ...defaults] = command
  return {
    executable: platform === 'win32' ? 'powershell.exe' : 'pwsh',
    args: ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', fileURLToPath(new URL(script, import.meta.url)), ...defaults, ...extra],
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    const { executable, args } = deploymentCommand(process.argv[2], process.argv.slice(3))
    if (process.argv.slice(3).some((arg) => ['--help', '-h', '-?'].includes(arg))) {
      console.log('Uso: npm run deploy -- [opcoes PowerShell]')
      console.log('Opcoes comuns: -User <usuario> -KeyPath <chave> -Tag <tag>')
      console.log('Deploy rapido: -Service both|api|web -TargetPlatform linux/amd64')
      console.log(`Executor: ${executable}. Guia: docs/DEPLOY_VPS.md`)
      process.exit(0)
    }
    const configPath = join(homedir(), '.config', 'omni', 'deploy.json')
    const config = existsSync(configPath) ? JSON.parse(readFileSync(configPath, 'utf8')) : {}
    for (const key of ['sshUser', 'sshKeyPath']) {
      if (config[key] !== undefined && typeof config[key] !== 'string') {
        throw new Error(`Campo ${key} invalido em ${configPath}`)
      }
    }
    const result = spawnSync(executable, args, {
      stdio: 'inherit', shell: false, env: deploymentEnvironment(process.env, config),
    })
    if (result.error) {
      console.error(result.error.code === 'ENOENT'
        ? `Instale ${executable} para executar os scripts de deploy. No Mac: ruby scripts/dev/setup-mac-tools.rb`
        : result.error.message)
      process.exitCode = 1
    } else {
      process.exitCode = result.status ?? 1
    }
  } catch (error) {
    console.error(error.message)
    process.exitCode = 1
  }
}
