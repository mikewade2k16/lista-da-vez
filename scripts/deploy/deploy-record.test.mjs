import assert from 'node:assert/strict'
import { EventEmitter } from 'node:events'
import { test } from 'node:test'
import {
  buildUpsertSql,
  deployEnvironment,
  extractImageTag,
  recordDeployHistory,
  remoteCommand,
  runSql,
  summaryToDeployRow,
  writerCommand,
} from './deploy-record.mjs'

const summary = {
  command: 'fast-prod',
  startedAt: '2026-11-23T14:11:53.645Z',
  finishedAt: '2026-11-23T14:15:35.184Z',
  exitCode: 0,
  durationMs: 221533.9,
  stages: [
    {
      name: 'Preflight SSH',
      status: 'ok',
      durationMs: 1770.9,
      id: 'x',
      depth: 0,
      timestamp: 't',
    },
    {
      name: 'Gravando IMAGE_TAG=local-20261123-111158 em .env.production',
      status: 'ok',
      durationMs: 900,
      depth: 1,
    },
    { name: 'Interrompida', status: 'interrupted', durationMs: null },
  ],
}

test('status e ambiente derivados do resumo', () => {
  const row = summaryToDeployRow('run-1', summary, {
    gitSha: 'abc',
    gitBranch: 'main',
  })
  assert.equal(row.status, 'success')
  assert.equal(row.environment, 'prod')
  assert.equal(row.git_sha, 'abc')
  assert.equal(row.image_tag, 'local-20261123-111158')
  assert.deepEqual(row.stages[0], {
    name: 'Preflight SSH',
    depth: 0,
    status: 'ok',
    durationMs: 1770.9,
  })
  assert.deepEqual(Object.keys(row.stages[2]), [
    'name',
    'depth',
    'status',
    'durationMs',
  ])
  assert.equal(row.stages[2].durationMs, null)

  assert.equal(
    summaryToDeployRow('r', { ...summary, exitCode: 1 }).status,
    'failed',
  )
  assert.equal(
    summaryToDeployRow('r', { ...summary, finishedAt: undefined }).status,
    'interrupted',
  )
  assert.equal(
    summaryToDeployRow('r', { ...summary, exitCode: undefined }).status,
    'unknown',
  )
  assert.equal(
    summaryToDeployRow('r', { ...summary, command: 'fast-staging' })
      .environment,
    'staging',
  )
  assert.equal(
    summaryToDeployRow('r', { ...summary, command: 'staging-down' })
      .environment,
    '',
  )
  assert.throws(
    () => summaryToDeployRow('r', { ...summary, startedAt: 'x' }),
    /startedAt/,
  )
  assert.equal(deployEnvironment('prod'), 'prod')
  assert.equal(extractImageTag([{ name: 'nada' }]), '')
})

test('upsert usa jsonb_populate_recordset e recusa o delimitador no conteudo', () => {
  const row = summaryToDeployRow("o'brien", {
    ...summary,
    command: 'fast-prod',
  })
  const sql = buildUpsertSql([row])
  assert.match(
    sql,
    /jsonb_populate_recordset\(null::obs\.deploys, \$omni_deploys\$/,
  )
  assert.match(sql, /on conflict \(id\) do update/)
  assert.ok(sql.includes(`"id":"o'brien"`))
  assert.throws(
    () => buildUpsertSql([{ ...row, id: 'a$omni_deploys$b' }]),
    /incompativel/,
  )
})

test('destinos: local por docker exec e remoto por ssh sem segredos', () => {
  const local = writerCommand('local')
  assert.equal(local.executable, 'docker')
  assert.deepEqual(local.args.slice(0, 5), [
    'exec',
    '-i',
    'omni-postgres-1',
    'psql',
    '-U',
  ])
  const remote = writerCommand('prod', {
    OMNI_DEPLOY_USER: 'ana',
    OMNI_DEPLOY_KEY_PATH: '/k',
  })
  assert.equal(remote.executable, 'ssh')
  assert.ok(remote.args.includes('ana@85.31.62.33'))
  assert.ok(remote.args.includes('/k'))
  const command = remote.args.at(-1)
  assert.match(command, /cd '\/home\/deploy\/lista-atendimento'/)
  assert.match(command, /--env-file '\.env\.production'/)
  assert.match(command, /exec -T postgres sh -c/)
  assert.match(command, /\$POSTGRES_USER/)
  assert.match(
    remoteCommand('staging'),
    /lista-atendimento-staging.*\.env\.staging/,
  )
  assert.throws(() => remoteCommand('local'), /desconhecido/)
})

function fakeSpawn({ status = 0, stderr = '', hang = false } = {}) {
  const calls = []
  const fn = (executable, args) => {
    const child = new EventEmitter()
    child.stdout = new EventEmitter()
    child.stderr = new EventEmitter()
    child.stdin = new EventEmitter()
    child.stdin.end = (sql) => {
      calls.push({ executable, args, sql })
      if (hang) return
      if (stderr) child.stderr.emit('data', stderr)
      setImmediate(() => child.emit('close', status))
    }
    child.kill = () => child.emit('close', null)
    return child
  }
  return { fn, calls }
}

test('runSql envia o SQL pela stdin e resume erros', async () => {
  const ok = fakeSpawn()
  await runSql('local', 'select 1', { spawnFn: ok.fn })
  assert.equal(ok.calls[0].sql, 'select 1')
  const bad = fakeSpawn({
    status: 1,
    stderr: 'ERROR: relation "obs.deploys" does not exist\nLINE 1',
  })
  await assert.rejects(
    runSql('local', 'x', { spawnFn: bad.fn }),
    /does not exist/,
  )
  const slow = fakeSpawn({ hang: true })
  await assert.rejects(
    runSql('prod', 'x', { spawnFn: slow.fn, timeoutMs: 10 }),
    /tempo esgotado/,
  )
})

test('gravacao automatica nunca lanca e respeita OBS_DEPLOY_RECORD=0', async () => {
  const warnings = []
  const writes = []
  const base = {
    directory: '/tmp/2026-run-1',
    summary,
    warn: (message) => warnings.push(message),
    log: () => {},
    git: () => ({ gitSha: 'abc', gitBranch: 'main' }),
    write: async (rows, target) => writes.push({ rows, target }),
  }
  assert.equal(await recordDeployHistory({ ...base, env: {} }), true)
  assert.equal(writes[0].target, 'prod')
  assert.equal(writes[0].rows[0].id, '2026-run-1')
  assert.equal(writes[0].rows[0].git_sha, 'abc')

  assert.equal(
    await recordDeployHistory({ ...base, env: { OBS_DEPLOY_RECORD: '0' } }),
    false,
  )
  assert.equal(writes.length, 1)

  assert.equal(
    await recordDeployHistory({
      ...base,
      summary: { ...summary, command: 'staging-down' },
      env: {},
    }),
    false,
  )
  assert.equal(writes.length, 1)

  const failing = await recordDeployHistory({
    ...base,
    env: {},
    write: async () => {
      throw new Error('ssh: connect to host timed out\nmais')
    },
  })
  assert.equal(failing, false)
  assert.deepEqual(warnings, [
    '[obs] histórico de deploy não gravado: ssh: connect to host timed out',
  ])

  assert.equal(
    await recordDeployHistory({
      ...base,
      summary: undefined,
      directory: '/nao/existe',
      env: {},
    }),
    false,
  )
  assert.equal(warnings.length, 2)
})
