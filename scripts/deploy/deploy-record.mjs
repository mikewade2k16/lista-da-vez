// Historico de deploys: converte o summary.json de uma execucao em uma linha de
// obs.deploys (migration 0330) e grava por upsert no banco do ambiente alvo.
// Nunca le nem imprime segredos: o SQL/JSON segue pela stdin e, em prod/staging,
// as credenciais do Postgres vem do proprio container (variaveis do Compose remoto).
import { spawn, spawnSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { basename, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const REPO_ROOT = fileURLToPath(new URL('../../', import.meta.url))
const DOLLAR_TAG = '$omni_deploys$'

const ENVIRONMENT_BY_COMMAND = {
  'fast-prod': 'prod',
  prod: 'prod',
  promote: 'prod',
  'fast-staging': 'staging',
  staging: 'staging',
}

// Destinos remotos: mesmos host, usuario, caminho e env file de deploy-pull.ps1.
const REMOTE_TARGETS = {
  prod: {
    path: '/home/deploy/omni',
    envFile: '.env.production',
  },
  staging: {
    path: '/home/deploy/lista-atendimento-staging',
    envFile: '.env.staging',
  },
}
const REMOTE_HOST = '85.31.62.33'
const LOCAL_POSTGRES_CONTAINER = 'omni-postgres-1'
const DEFAULT_TIMEOUT_MS = 20000

export function deployEnvironment(command) {
  return ENVIRONMENT_BY_COMMAND[command] ?? ''
}

export function extractImageTag(stages) {
  for (const stage of stages ?? []) {
    const match = /IMAGE_TAG=([A-Za-z0-9][A-Za-z0-9_.-]{0,127})/.exec(
      stage?.name ?? '',
    )
    if (match) return match[1]
  }
  return ''
}

export function reduceStages(stages) {
  return (stages ?? []).map((stage) => ({
    name: String(stage.name ?? ''),
    depth: Number.isInteger(stage.depth) ? stage.depth : 0,
    status: String(stage.status ?? ''),
    durationMs: Number.isFinite(stage.durationMs) ? stage.durationMs : null,
  }))
}

export function deployStatus(summary) {
  if (!summary.finishedAt) return 'interrupted'
  if (summary.exitCode === 0) return 'success'
  if (Number.isInteger(summary.exitCode)) return 'failed'
  return 'unknown'
}

// meta: { gitSha, gitBranch, triggeredBy } (todos opcionais).
export function summaryToDeployRow(id, summary, meta = {}) {
  if (!id || typeof id !== 'string') throw new Error('id do deploy ausente')
  if (!summary?.command) throw new Error('summary sem command')
  if (Number.isNaN(Date.parse(summary.startedAt))) {
    throw new Error('summary sem startedAt valido')
  }
  const stages = reduceStages(summary.stages)
  return {
    id,
    command: String(summary.command),
    environment: deployEnvironment(summary.command),
    started_at: summary.startedAt,
    finished_at: summary.finishedAt ?? null,
    duration_ms: Number.isFinite(summary.durationMs)
      ? summary.durationMs
      : null,
    exit_code: Number.isInteger(summary.exitCode) ? summary.exitCode : null,
    status: deployStatus(summary),
    git_sha: meta.gitSha ?? '',
    git_branch: meta.gitBranch ?? '',
    image_tag: extractImageTag(summary.stages),
    triggered_by: meta.triggeredBy ?? '',
    stages,
  }
}

// Upsert por id. O JSON vai como literal dollar-quoted lido por
// jsonb_populate_recordset: nenhum valor e concatenado como SQL.
// Em conflito, campos de git/imagem ja preenchidos nao sao apagados por vazios.
export function buildUpsertSql(rows) {
  const json = JSON.stringify(rows)
  if (json.includes(DOLLAR_TAG)) {
    throw new Error('conteudo incompativel com o literal SQL')
  }
  const keep = (column) =>
    `${column} = coalesce(nullif(excluded.${column}, ''), obs.deploys.${column})`
  return `insert into obs.deploys
  (id, command, environment, started_at, finished_at, duration_ms, exit_code, status,
   git_sha, git_branch, image_tag, triggered_by, stages)
select id, command, environment, started_at, finished_at, duration_ms, exit_code, status,
       coalesce(git_sha, ''), coalesce(git_branch, ''), coalesce(image_tag, ''),
       coalesce(triggered_by, ''), coalesce(stages, '[]'::jsonb)
from jsonb_populate_recordset(null::obs.deploys, ${DOLLAR_TAG}${json}${DOLLAR_TAG}::jsonb)
on conflict (id) do update set
  command = excluded.command,
  environment = excluded.environment,
  started_at = excluded.started_at,
  finished_at = excluded.finished_at,
  duration_ms = excluded.duration_ms,
  exit_code = excluded.exit_code,
  status = excluded.status,
  ${keep('git_sha')},
  ${keep('git_branch')},
  ${keep('image_tag')},
  ${keep('triggered_by')},
  stages = excluded.stages;
`
}

export function gitMeta(cwd = REPO_ROOT, run = spawnSync) {
  const read = (args) => {
    const result = run('git', args, { cwd, encoding: 'utf8', timeout: 5000 })
    return result.status === 0 ? String(result.stdout).trim() : ''
  }
  return {
    gitSha: read(['rev-parse', 'HEAD']),
    gitBranch: read(['rev-parse', '--abbrev-ref', 'HEAD']),
    triggeredBy: process.env.OBS_DEPLOY_TRIGGERED_BY ?? '',
  }
}

function bashQuote(value) {
  return `'${String(value).replaceAll("'", `'"'"'`)}'`
}

// Comando remoto: psql roda DENTRO do container do postgres, que ja tem
// POSTGRES_USER/POSTGRES_DB do Compose remoto; nada de segredo sai da VPS.
export function remoteCommand(target) {
  const remote = REMOTE_TARGETS[target]
  if (!remote) throw new Error(`destino remoto desconhecido: ${target}`)
  return [
    `cd ${bashQuote(remote.path)}`,
    `docker compose --env-file ${bashQuote(remote.envFile)} -f docker-compose.prod.yml exec -T postgres sh -c ${bashQuote('psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -q')}`,
  ].join(' && ')
}

export function sshArguments(env = process.env, home = homedir()) {
  const key =
    env.OMNI_DEPLOY_KEY_PATH || join(home, '.ssh', 'gh_actions_omnichannel_vps')
  const args = [
    '-i',
    key,
    '-o',
    'StrictHostKeyChecking=accept-new',
    '-o',
    'BatchMode=yes',
    '-o',
    'ConnectTimeout=15',
    '-o',
    'ServerAliveInterval=10',
    '-o',
    'ServerAliveCountMax=3',
  ]
  // Reaproveita a conexao do deploy se ainda estiver viva, sem criar outra.
  if (process.platform !== 'win32') {
    args.push(
      '-o',
      'ControlMaster=no',
      '-o',
      `ControlPath=${join(home, '.ssh', 'omni-deploy-%C')}`,
    )
  }
  return args
}

// Monta executavel e argumentos de gravacao (a stdin recebe o SQL).
export function writerCommand(target, env = process.env) {
  if (target === 'local') {
    return {
      executable: 'docker',
      args: [
        'exec',
        '-i',
        LOCAL_POSTGRES_CONTAINER,
        'psql',
        '-U',
        'omni',
        '-d',
        'omni',
        '-v',
        'ON_ERROR_STOP=1',
        '-q',
      ],
    }
  }
  const user = env.OMNI_DEPLOY_USER || 'deploy'
  return {
    executable: 'ssh',
    args: [
      ...sshArguments(env),
      `${user}@${REMOTE_HOST}`,
      remoteCommand(target),
    ],
  }
}

function shortReason(text) {
  const line =
    String(text)
      .split('\n')
      .map((item) => item.trim())
      .find(Boolean) ?? ''
  return line.slice(0, 140)
}

// Executa o SQL no destino. Rejeita com motivo curto (sem imprimir a stdin).
export function runSql(
  target,
  sql,
  { env = process.env, timeoutMs = DEFAULT_TIMEOUT_MS, spawnFn = spawn } = {},
) {
  const { executable, args } = writerCommand(target, env)
  return new Promise((resolve, reject) => {
    let stdout = ''
    let stderr = ''
    let settled = false
    const child = spawnFn(executable, args, {
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: false,
    })
    const finish = (error, value) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      if (error) reject(error)
      else resolve(value)
    }
    const timer = setTimeout(() => {
      finish(new Error(`tempo esgotado (${Math.round(timeoutMs / 1000)}s)`))
      child.kill('SIGKILL')
    }, timeoutMs)
    child.stdout?.on('data', (chunk) => (stdout += chunk))
    child.stderr?.on('data', (chunk) => (stderr += chunk))
    child.once('error', (error) =>
      finish(
        new Error(
          error.code === 'ENOENT'
            ? `${executable} nao encontrado`
            : shortReason(error.message),
        ),
      ),
    )
    child.once('close', (status) =>
      status === 0
        ? finish(null, stdout)
        : finish(
            new Error(
              shortReason(stderr) || `${executable} terminou com ${status}`,
            ),
          ),
    )
    child.stdin?.on('error', () => {})
    child.stdin?.end(sql)
  })
}

export function upsertDeploys(rows, target, options) {
  return runSql(target, buildUpsertSql(rows), options)
}

// Gravacao automatica ao fim do deploy: nunca lanca, so avisa.
export async function recordDeployHistory({
  directory,
  summary = undefined,
  env = process.env,
  warn = console.warn,
  log = console.log,
  git = gitMeta,
  write = upsertDeploys,
}) {
  try {
    if (env.OBS_DEPLOY_RECORD === '0') return false
    const data =
      summary ??
      JSON.parse(readFileSync(join(directory, 'summary.json'), 'utf8'))
    const target = deployEnvironment(data.command)
    if (!target) return false
    const row = summaryToDeployRow(basename(directory), data, git())
    const timeoutMs =
      Number.parseInt(env.OBS_DEPLOY_RECORD_TIMEOUT_MS, 10) ||
      DEFAULT_TIMEOUT_MS
    await write([row], target, { env, timeoutMs })
    log(`[obs] historico de deploy gravado (${target}): ${row.id}`)
    return true
  } catch (error) {
    warn(
      `[obs] histórico de deploy não gravado: ${shortReason(error?.message ?? error)}`,
    )
    return false
  }
}
