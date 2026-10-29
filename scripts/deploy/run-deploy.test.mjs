import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { deploymentCommand, deploymentEnvironment } from './run-deploy.mjs'

test('Mac e Windows escolhem o PowerShell correto e preservam argumentos com espacos', () => {
  const extra = ['-KeyPath', '/Users/test user/.ssh/key', '-User', 'deploy']
  for (const [platform, shell] of [['darwin', 'pwsh'], ['win32', 'powershell.exe']]) {
    const plan = deploymentCommand('fast-prod', extra, platform)
    assert.equal(plan.executable, shell)
    assert.deepEqual(plan.args.slice(-4), extra)
    assert.ok(plan.args.includes('-BackupDatabase'))
    assert.ok(plan.args.includes('-DeployOmnichannel'))
    assert.ok(!plan.args.includes('-DeployAutomation'))
  }
})

test('todos os atalhos resolvem um script conhecido e comandos desconhecidos falham', () => {
  for (const name of ['fast-staging', 'fast-prod', 'staging', 'staging-down', 'prod', 'promote']) {
    assert.ok(deploymentCommand(name).args.some((arg) => arg.endsWith('.ps1')))
  }
  assert.throws(() => deploymentCommand('../../other'), /desconhecido/)
})

test('ajuda funciona sem PowerShell e sem executar preflight, build ou deploy', () => {
  const launcher = fileURLToPath(new URL('./run-deploy.mjs', import.meta.url))
  const result = spawnSync(process.execPath, [launcher, 'fast-prod', '--help'], {
    encoding: 'utf8', env: { ...process.env, PATH: '' },
  })
  assert.equal(result.status, 0, result.stderr)
  assert.match(result.stdout, /docs\/DEPLOY_VPS.md/)
})

test('configuracao local resolve a chave no home e respeita overrides do ambiente', () => {
  const config = { sshUser: 'operador', sshKeyPath: '~/.ssh/mac key' }
  const env = { PATH: '/bin' }
  assert.deepEqual(deploymentEnvironment(env, config, '/Users/test'), {
    PATH: '/bin', OMNI_DEPLOY_USER: 'operador', OMNI_DEPLOY_KEY_PATH: '/Users/test/.ssh/mac key',
  })
  assert.deepEqual(env, { PATH: '/bin' })
  assert.equal(deploymentEnvironment({ OMNI_DEPLOY_USER: 'outro' }, config).OMNI_DEPLOY_USER, 'outro')
  assert.equal(deploymentEnvironment({ OMNI_DEPLOY_KEY_PATH: '/custom' }, config).OMNI_DEPLOY_KEY_PATH, '/custom')
  assert.deepEqual(deploymentEnvironment(env, {}), env)
})
