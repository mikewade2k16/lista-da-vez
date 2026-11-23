import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

export function createDeployReport(directory, command, log = console.log) {
  mkdirSync(directory, { recursive: true, mode: 0o700 })
  const eventsFile = join(directory, 'timings.jsonl')
  writeFileSync(eventsFile, '', { mode: 0o600 })
  const startedAt = new Date().toISOString()
  const started = process.hrtime.bigint()
  return {
    eventsFile,
    finish(exitCode) {
      const durationMs = Number(process.hrtime.bigint() - started) / 1e6
      const events = []
      let incompleteEvents = 0
      for (const line of readFileSync(eventsFile, 'utf8')
        .split('\n')
        .filter(Boolean)) {
        try {
          events.push(JSON.parse(line.replace(/^\uFEFF/, '')))
        } catch {
          incompleteEvents++
        }
      }
      const stages = events.filter((event) => event.status !== 'started')
      for (const event of events.filter(
        (event) => event.status === 'started',
      )) {
        if (
          !stages.some((stage) =>
            event.id ? stage.id === event.id : stage.name === event.name,
          )
        ) {
          stages.push({ ...event, status: 'interrupted', durationMs: null })
        }
      }
      const report = {
        command,
        startedAt,
        finishedAt: new Date().toISOString(),
        exitCode,
        durationMs,
        incompleteEvents,
        stages,
      }
      const summary = join(directory, 'summary.json')
      writeFileSync(summary, JSON.stringify(report, null, 2) + '\n', {
        mode: 0o600,
      })
      log(
        `\n[tempo] TOTAL | ${exitCode === 0 ? 'ok' : 'failed'} | ${(durationMs / 1000).toFixed(1)}s`,
      )
      for (const stage of stages) {
        log(
          `[tempo] ${'  '.repeat(stage.depth ?? 0)}${stage.name}: ${stage.durationMs === null ? 'interrompida' : (stage.durationMs / 1000).toFixed(1) + 's'} (${stage.status})`,
        )
      }
      log(`[tempo] Relatorio: ${summary}`)
      return report
    },
  }
}
