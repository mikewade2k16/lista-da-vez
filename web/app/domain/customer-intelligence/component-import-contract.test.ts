import { readdirSync, readFileSync } from 'node:fs'
import { basename, dirname, extname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const appRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const componentRoot = join(appRoot, 'components/customer-intelligence')
const pageRoot = join(appRoot, 'pages/inteligencia-clientes')

function vueFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    return entry.isDirectory() ? vueFiles(path) : extname(entry.name) === '.vue' ? [path] : []
  })
}

describe('customer intelligence component import contract', () => {
  it('explicitly imports components stored in nested directories', () => {
    const componentFiles = vueFiles(componentRoot)
    const nestedComponents = new Map(
      componentFiles
        .filter((path) => dirname(path) !== componentRoot)
        .map((path) => [basename(path, '.vue'), path]),
    )
    const missing: string[] = []

    for (const path of [...componentFiles, ...vueFiles(pageRoot)]) {
      const source = readFileSync(path, 'utf8')
      const imported = new Set(
        Array.from(source.matchAll(/import\s+([A-Z][A-Za-z0-9]+)\s+from/g), (match) => match[1]),
      )
      const tags = new Set(
        Array.from(source.matchAll(/<([A-Z][A-Za-z0-9]+)(?:\s|\/|>)/g), (match) => match[1]),
      )
      const selfName = basename(path, '.vue')

      for (const tag of tags) {
        if (!nestedComponents.has(tag) || tag === selfName || imported.has(tag)) continue
        missing.push(`${path.replace(appRoot, 'app')} -> ${tag}`)
      }
    }

    expect(missing.sort()).toEqual([])
  })
})
