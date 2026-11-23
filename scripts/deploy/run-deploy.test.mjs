import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  writeFileSync,
  rmSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  acquireDeployLock,
  deploymentCommand,
  deploymentEnvironment,
  runDeployment,
} from './run-deploy.mjs'
import { createDeployReport } from './deploy-report.mjs'
import { checkDocker } from './check-docker.mjs'

test('preflight Docker consulta o servidor com timeout e falha antes do build', () => {
  const healthy = (exe, args, options) => {
    assert.equal(exe, 'docker')
    assert.deepEqual(args, ['version', '--format', '{{json .Server}}'])
    assert.equal(options.timeout, 5000)
    assert.equal(options.killSignal, 'SIGKILL')
    return {
      status: 0,
      stdout: JSON.stringify({ Version: 'test', Os: 'linux' }),
    }
  }
  assert.equal(checkDocker(healthy), 'test')
  assert.throws(
    () => checkDocker(() => ({ error: { code: 'ETIMEDOUT' } })),
    /nao respondeu em 5s/,
  )
  assert.throws(
    () => checkDocker(() => ({ error: { code: 'ENOENT' } })),
    /nao encontrado/,
  )
  assert.throws(
    () => checkDocker(() => ({ status: 1, stdout: '' })),
    /indisponivel/,
  )
  assert.throws(
    () => checkDocker(() => ({ status: 0, stdout: 'broken' })),
    /resposta invalida/,
  )
  assert.throws(
    () => checkDocker(() => ({ status: 0, stdout: 'null' })),
    /Linux/,
  )
})

test('Mac e Windows escolhem o PowerShell correto e preservam argumentos com espacos', () => {
  const extra = ['-KeyPath', '/Users/test user/.ssh/key', '-User', 'deploy']
  for (const [platform, shell] of [
    ['darwin', 'pwsh'],
    ['win32', 'powershell.exe'],
  ]) {
    const plan = deploymentCommand('fast-prod', extra, platform)
    assert.equal(plan.executable, shell)
    assert.deepEqual(plan.args.slice(-4), extra)
    assert.ok(plan.args.includes('-BackupDatabase'))
    assert.ok(plan.args.includes('-DeployOmnichannel'))
    assert.ok(!plan.args.includes('-DeployAutomation'))
  }
})

test('todos os atalhos resolvem um script conhecido e comandos desconhecidos falham', () => {
  for (const name of [
    'fast-staging',
    'fast-prod',
    'staging',
    'staging-down',
    'prod',
    'promote',
  ]) {
    assert.ok(deploymentCommand(name).args.some((arg) => arg.endsWith('.ps1')))
  }
  assert.throws(() => deploymentCommand('../../other'), /desconhecido/)
})

test('ajuda funciona sem PowerShell e sem executar preflight, build ou deploy', () => {
  const launcher = fileURLToPath(new URL('./run-deploy.mjs', import.meta.url))
  const result = spawnSync(
    process.execPath,
    [launcher, 'fast-prod', '--help'],
    {
      encoding: 'utf8',
      env: { ...process.env, PATH: '' },
    },
  )
  assert.equal(result.status, 0, result.stderr)
  assert.match(result.stdout, /docs\/DEPLOY_VPS.md/)
})

test('configuracao local resolve a chave no home e respeita overrides do ambiente', () => {
  const config = { sshUser: 'operador', sshKeyPath: '~/.ssh/mac key' }
  const env = { PATH: '/bin' }
  assert.deepEqual(deploymentEnvironment(env, config, '/Users/test'), {
    PATH: '/bin',
    OMNI_DEPLOY_USER: 'operador',
    OMNI_DEPLOY_KEY_PATH: '/Users/test/.ssh/mac key',
  })
  assert.deepEqual(env, { PATH: '/bin' })
  assert.equal(
    deploymentEnvironment({ OMNI_DEPLOY_USER: 'outro' }, config)
      .OMNI_DEPLOY_USER,
    'outro',
  )
  assert.equal(
    deploymentEnvironment({ OMNI_DEPLOY_KEY_PATH: '/custom' }, config)
      .OMNI_DEPLOY_KEY_PATH,
    '/custom',
  )
  assert.deepEqual(deploymentEnvironment(env, {}), env)
})

function testReport(t) {
  const directory = mkdtempSync(join(tmpdir(), 'omni-deploy-report-'))
  t.after(() => rmSync(directory, { recursive: true, force: true }))
  return {
    directory,
    report: createDeployReport(directory, 'fast-prod', () => {}),
  }
}

test('relatorio preserva etapas, falha e etapa interrompida sem registrar credenciais', (t) => {
  const { directory, report } = testReport(t)
  const events = [
    { name: 'Build web', id: 'web', status: 'started', durationMs: 0 },
    { name: 'Build web', id: 'web', status: 'ok', durationMs: 1500 },
    { name: 'SFTP', id: 'sftp', status: 'started', durationMs: 0 },
  ]
  writeFileSync(
    report.eventsFile,
    events.map((event) => JSON.stringify(event)).join('\n') + '\n{"name":',
  )
  const result = report.finish(130)
  assert.equal(result.exitCode, 130)
  assert.equal(result.incompleteEvents, 1)
  assert.ok(result.durationMs >= 0)
  assert.deepEqual(
    result.stages.map(({ name, status }) => [name, status]),
    [
      ['Build web', 'ok'],
      ['SFTP', 'interrupted'],
    ],
  )
  assert.equal(result.stages[1].durationMs, null)
  assert.deepEqual(
    JSON.parse(readFileSync(join(directory, 'summary.json'), 'utf8')),
    result,
  )
  assert.ok(
    !readFileSync(join(directory, 'summary.json'), 'utf8').includes(
      'OMNI_DEPLOY_KEY_PATH',
    ),
  )
})

test('executor conserva exit code zero e nao zero e sempre encerra o relatorio', async (t) => {
  for (const code of [0, 7]) {
    const { directory, report } = testReport(t)
    assert.equal(
      await runDeployment(
        process.execPath,
        ['-e', `process.exit(${code})`],
        process.env,
        report,
      ),
      code,
    )
    assert.equal(
      JSON.parse(readFileSync(join(directory, 'summary.json'), 'utf8'))
        .exitCode,
      code,
    )
  }
})

test('executor ausente tambem produz relatorio de falha', async (t) => {
  const { directory, report } = testReport(t)
  assert.equal(
    await runDeployment(
      join(directory, 'missing-executable'),
      [],
      process.env,
      report,
      () => {},
    ),
    1,
  )
  assert.equal(
    JSON.parse(readFileSync(join(directory, 'summary.json'), 'utf8')).exitCode,
    1,
  )
})

test('interrupcao do launcher termina o filho e registra exit code 143', async (t) => {
  const { directory, report } = testReport(t)
  const timer = setTimeout(() => process.emit('SIGTERM'), 100)
  t.after(() => clearTimeout(timer))
  assert.equal(
    await runDeployment(
      process.execPath,
      ['-e', 'setInterval(() => {}, 1000)'],
      process.env,
      report,
    ),
    143,
  )
  assert.equal(
    JSON.parse(readFileSync(join(directory, 'summary.json'), 'utf8')).exitCode,
    143,
  )
})

test(
  'tempos remotos preservam rollout e rollback sem acessar Docker nem VPS',
  { skip: process.platform === 'win32' },
  (t) => {
    for (const failure of ['none', 'up', 'http']) {
      const directory = mkdtempSync(join(tmpdir(), 'omni-deploy-rollout-'))
      t.after(() => rmSync(directory, { recursive: true, force: true }))
      const release = join(directory, '.deploy', 'test')
      mkdirSync(release, { recursive: true })
      writeFileSync(join(release, 'env.before'), 'IMAGE_TAG=old')
      writeFileSync(join(release, 'compose.before.yml'), 'old-compose')
      writeFileSync(join(directory, '.env.production'), 'IMAGE_TAG=new')
      writeFileSync(join(directory, 'docker-compose.prod.yml'), 'new-compose')
      const shellQuote = (value) => "'" + value.replaceAll("'", "'\"'\"'") + "'"
      const replacements = {
        '@REMOTE_PATH@': shellQuote(directory),
        '@ENV_FILE@': '.env.production',
        '@RELEASE_DIR@': '.deploy/test',
        '@REGISTRY_AUTH@': '',
        '@IMAGE_PULL_QUOTED@': shellQuote(
          'docker image inspect fake-image >/dev/null',
        ),
        '@FORCE_RECREATE@': '',
        '@PUBLIC_URL@': 'http://example.invalid',
      }
      let rollout = readFileSync(
        new URL('./rollout-app.sh', import.meta.url),
        'utf8',
      )
      for (const [key, value] of Object.entries(replacements))
        rollout = rollout.replaceAll(key, value)
      const stubs = `
docker() {
  if [[ "$FAILURE" == up && "$*" == *"up -d"* && ! -f "$ONCE" ]]; then touch "$ONCE"; return 9; fi
  return 0
}
curl() { if [[ "$FAILURE" == http ]]; then return 22; fi; return 0; }
export -f docker curl
`
      const result = spawnSync('bash', ['-s'], {
        input:
          stubs +
          readFileSync(new URL('./remote-timing.sh', import.meta.url), 'utf8') +
          '\n' +
          rollout,
        encoding: 'utf8',
        env: {
          ...process.env,
          FAILURE: failure,
          ONCE: join(directory, 'failed-once'),
        },
      })
      assert.equal(
        result.status,
        failure === 'none' ? 0 : failure === 'up' ? 9 : 22,
        result.stderr,
      )
      const expectedEnv = failure === 'none' ? 'IMAGE_TAG=new' : 'IMAGE_TAG=old'
      assert.equal(
        readFileSync(join(directory, '.env.production'), 'utf8'),
        expectedEnv,
      )
      assert.equal(
        readFileSync(join(directory, 'docker-compose.prod.yml'), 'utf8'),
        failure === 'none' ? 'new-compose' : 'old-compose',
      )
      const events = result.stdout
        .split('\n')
        .filter((line) => line.startsWith('OMNI_DEPLOY_STAGE '))
        .map((line) => JSON.parse(line.slice('OMNI_DEPLOY_STAGE '.length)))
      assert.ok(events.length > 0)
      assert.ok(
        events.every(
          (event) => Number.isFinite(event.durationMs) && event.durationMs >= 0,
        ),
      )
      if (failure === 'none') {
        assert.deepEqual(
          events
            .filter(({ status }) => status === 'ok')
            .map(({ name }) => name),
          [
            'remote.images.pull',
            'remote.containers.health',
            'remote.http.health',
            'remote.http.web',
          ],
        )
      } else {
        assert.equal(events.at(-1).status, 'failed')
      }
    }
  },
)

test('trava impede deploy simultaneo e substitui trava abandonada', () => {
  const directory = mkdtempSync(join(tmpdir(), 'omni-deploy-lock-'))
  const lock = join(directory, 'nested', 'deploy.lock')
  try {
    const release = acquireDeployLock(lock, 111, () => true)
    assert.throws(
      () => acquireDeployLock(lock, 222, () => true),
      /Outro deploy ja esta em execucao \(PID 111\)/,
    )
    release()
    const releaseSecond = acquireDeployLock(lock, 222, () => true)
    assert.equal(readFileSync(lock, 'utf8').trim(), '222')
    // Dono morto: a trava abandonada e substituida.
    const releaseThird = acquireDeployLock(lock, 333, () => false)
    assert.equal(readFileSync(lock, 'utf8').trim(), '333')
    releaseSecond() // nao remove trava de outro PID
    assert.equal(readFileSync(lock, 'utf8').trim(), '333')
    releaseThird()
    assert.throws(() => readFileSync(lock, 'utf8'), /ENOENT/)
  } finally {
    rmSync(directory, { recursive: true, force: true })
  }
})
