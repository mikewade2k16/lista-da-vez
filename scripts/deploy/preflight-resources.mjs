// Preflight de recursos LOCAIS do deploy rapido. Roda antes de validacao, build
// e qualquer acao remota: falha cedo, com motivo acionavel, em vez de quebrar no
// meio (build web sem memoria, Docker sem disco, porta do tunel ocupada...).
//
// Uso: node preflight-resources.mjs [--web-build native|docker] [--heap-mb 6144]
//        [--tunnel-port 5055] [--registry-image registry:2.8.3]
// Nao altera nada; so le informacoes do host e do Docker.
import { spawnSync } from 'node:child_process'
import { readFileSync, statfsSync } from 'node:fs'
import { cpus, freemem, loadavg, totalmem } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createConnection } from 'node:net'

const GB = 1024 ** 3
const repoDir = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..')

function option(name, fallback) {
  const index = process.argv.indexOf(name)
  return index > -1 ? process.argv[index + 1] : fallback
}

// Memoria realmente disponivel. No macOS os.freemem() ignora paginas inativas
// e purgeable que o kernel libera sob demanda, subestimando muito.
export function availableMemoryBytes(platform = process.platform, readVmStat = () => run('vm_stat', [])) {
  if (platform === 'darwin') {
    const output = readVmStat()
    const pageSize = Number(output.match(/page size of (\d+) bytes/)?.[1] ?? 4096)
    const pages = (label) => Number(output.match(new RegExp(`Pages ${label}:\\s+(\\d+)`))?.[1] ?? 0)
    return (pages('free') + pages('inactive') + pages('speculative') + pages('purgeable')) * pageSize
  }
  if (platform === 'linux') {
    const kb = Number(readFileSync('/proc/meminfo', 'utf8').match(/MemAvailable:\s+(\d+)/)?.[1] ?? 0)
    if (kb) return kb * 1024
  }
  return freemem()
}

function run(command, args, timeout = 10000) {
  const result = spawnSync(command, args, { encoding: 'utf8', timeout, killSignal: 'SIGKILL', windowsHide: true })
  if (result.error?.code === 'ETIMEDOUT') throw new Error(`${command} nao respondeu em ${timeout / 1000}s.`)
  if (result.error || result.status !== 0) {
    throw new Error(`${command} ${args[0] ?? ''} falhou: ${(result.stderr || result.error?.message || '').trim()}`)
  }
  return result.stdout
}

function portIsFree(port) {
  return new Promise((resolvePort) => {
    const socket = createConnection({ host: '127.0.0.1', port })
    socket.once('connect', () => { socket.destroy(); resolvePort(false) })
    socket.once('error', () => resolvePort(true))
    socket.setTimeout(1000, () => { socket.destroy(); resolvePort(true) })
  })
}

export async function checkResources({
  webBuild = 'native',
  heapMb = 6144,
  tunnelPort = 0,
  registryImage = 'registry:2.8.3',
  registryContainer = 'omni-deploy-registry',
} = {}) {
  const failures = []
  const warnings = []
  const info = []

  // 1. Node do host = Node da imagem (so importa quando o web compila no host).
  if (webBuild === 'native') {
    const dockerfile = readFileSync(join(repoDir, 'web', 'Dockerfile.prebuilt'), 'utf8')
    const expected = dockerfile.match(/^FROM\s+node:(\d+\.\d+\.\d+)/m)?.[1]
    if (process.versions.node !== expected) {
      failures.push(`Node do host ${process.versions.node} difere da imagem web (${expected}). Instale ${expected} ou use -WebBuildMode docker.`)
    } else info.push(`Node ${expected} (igual a imagem)`)
  }

  // 2. Memoria do host. Pico medido do build nativo: ~4,3 GB de RSS.
  const available = availableMemoryBytes()
  // Override consciente (como OMNI_WEB_BUILD_MIN_FREE_MB do disco): abaixo do pico
  // de ~4,3 GB o build usa swap e fica mais lento, mas conclui.
  const memOverrideGb = Number(process.env.OMNI_WEB_BUILD_MIN_FREE_MEM_GB || '')
  const defaultRequiredGb = webBuild === 'native' ? Math.max(4.5, heapMb / 1024 * 0.75) : 1
  const requiredGb = memOverrideGb > 0 ? memOverrideGb : defaultRequiredGb
  if (memOverrideGb > 0) warnings.push(`Minimo de memoria reduzido por OMNI_WEB_BUILD_MIN_FREE_MEM_GB=${memOverrideGb} (padrao ${defaultRequiredGb.toFixed(1)} GB): o build pode usar swap.`)
  info.push(`Memoria disponivel no host: ${(available / GB).toFixed(1)} de ${(totalmem() / GB).toFixed(1)} GB`)
  if (available < requiredGb * GB) {
    failures.push(`Memoria livre insuficiente: ${(available / GB).toFixed(1)} GB (minimo ${requiredGb.toFixed(1)} GB para o build web). Feche aplicativos pesados ou pare servicos locais que nao estejam em uso.`)
  } else if (available < (requiredGb + 1.5) * GB) {
    warnings.push(`Memoria apertada (${(available / GB).toFixed(1)} GB livres): o build pode ficar lento por swap.`)
  }

  // 3. Carga de CPU do host (aviso: build continua correto, so mais lento).
  const [load] = loadavg()
  if (process.platform !== 'win32' && load > cpus().length * 2) {
    warnings.push(`Host com carga alta (load ${load.toFixed(1)} para ${cpus().length} CPUs): o build vai demorar mais. Verifique processos pesados no Monitor de Atividade.`)
  }

  // 4. Disco do host (staging do build web, node_modules e pacote SFTP de fallback).
  const disk = statfsSync(repoDir)
  const freeDisk = disk.bavail * disk.bsize
  info.push(`Disco livre no host: ${(freeDisk / GB).toFixed(1)} GB`)
  if (freeDisk < 5 * GB) failures.push(`Menos de 5 GB livres no disco do host (${(freeDisk / GB).toFixed(1)} GB).`)

  // 5. Docker: memoria da VM e disco da VM (builds e imagens moram la).
  const dockerInfo = JSON.parse(run('docker', ['info', '--format', '{{json .}}']))
  const dockerMem = Number(dockerInfo.MemTotal ?? 0)
  info.push(`Docker: ${dockerInfo.ServerVersion}, ${(dockerMem / GB).toFixed(1)} GB de RAM na VM, ${dockerInfo.ContainersRunning} containers ativos`)
  if (webBuild === 'docker' && dockerMem < 5.5 * GB) {
    failures.push(`O build web em Docker precisa de ~6 GB na VM do Docker; ha ${(dockerMem / GB).toFixed(1)} GB. Use -WebBuildMode native.`)
  }
  const hasRegistryImage = spawnSync('docker', ['image', 'inspect', registryImage], { stdio: 'ignore', timeout: 10000 }).status === 0
  if (hasRegistryImage) {
    const df = run('docker', ['run', '--rm', '--network', 'none', '--entrypoint', 'df', registryImage, '-Pk', '/'], 30000)
    const freeVmKb = Number(df.trim().split('\n').pop().split(/\s+/)[3])
    info.push(`Disco livre na VM do Docker: ${(freeVmKb / 1024 / 1024).toFixed(1)} GB`)
    if (freeVmKb * 1024 < 5 * GB) {
      failures.push(`Menos de 5 GB livres no disco da VM do Docker. Libere com: docker builder prune --filter until=168h (nao apaga volumes).`)
    }
  } else if (tunnelPort) {
    info.push(`Imagem ${registryImage} ausente: sera baixada uma unica vez para o transporte por tunel.`)
  }

  // 6. Porta do registry local do tunel: livre ou ja usada pelo nosso container.
  if (tunnelPort) {
    const owner = spawnSync('docker', ['ps', '-q', '--filter', `name=^${registryContainer}$`], { encoding: 'utf8', timeout: 10000 }).stdout?.trim()
    if (!owner && !(await portIsFree(tunnelPort))) {
      failures.push(`Porta local ${tunnelPort} ocupada por outro processo; o registry do tunel precisa dela. Libere-a ou use -TunnelPort <porta>.`)
    }
  }

  return { failures, warnings, info }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    const result = await checkResources({
      webBuild: option('--web-build', 'native'),
      heapMb: Number(option('--heap-mb', '6144')),
      tunnelPort: Number(option('--tunnel-port', '0')),
      registryImage: option('--registry-image', 'registry:2.8.3'),
    })
    for (const line of result.info) console.log(`  ${line}`)
    for (const line of result.warnings) console.warn(`  AVISO: ${line}`)
    for (const line of result.failures) console.error(`  ERRO: ${line}`)
    if (result.failures.length) {
      console.error('Preflight de recursos locais falhou; nada foi iniciado.')
      process.exitCode = 1
    } else console.log('Preflight de recursos locais: OK')
  } catch (error) {
    console.error(`Preflight de recursos locais falhou: ${error.message}`)
    process.exitCode = 1
  }
}
