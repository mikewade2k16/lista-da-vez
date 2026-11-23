import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

export function checkDocker(run = spawnSync, timeoutMs = 5000) {
  const result = run('docker', ['version', '--format', '{{json .Server}}'], {
    encoding: 'utf8',
    timeout: timeoutMs,
    killSignal: 'SIGKILL',
    windowsHide: true,
  })
  if (result.error?.code === 'ETIMEDOUT') {
    throw new Error(
      `Docker engine local nao respondeu em ${timeoutMs / 1000}s. Verifique o Docker Desktop antes de repetir; nenhum build ou envio foi iniciado.`,
    )
  }
  if (result.error?.code === 'ENOENT')
    throw new Error(
      'docker nao encontrado no PATH. Verifique a instalacao do Docker Desktop.',
    )
  if (result.error || result.status !== 0) {
    throw new Error(
      'Docker engine local indisponivel. Verifique o Docker Desktop e o contexto Docker antes de repetir o deploy.',
    )
  }
  let server
  try {
    server = JSON.parse(result.stdout)
  } catch {
    throw new Error(
      'Docker retornou uma resposta invalida ao consultar o engine local.',
    )
  }
  if (!server?.Version || server.Os !== 'linux') {
    throw new Error(
      'O deploy exige um Docker engine Linux disponivel; verifique o contexto Docker ativo.',
    )
  }
  return server.Version
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    console.log('Verificando Docker engine local (limite: 5s)...')
    console.log(`Docker engine Linux: OK (${checkDocker()})`)
  } catch (error) {
    console.error(error.message)
    process.exitCode = 1
  }
}
