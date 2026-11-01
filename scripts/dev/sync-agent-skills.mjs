import { cp, mkdir, readdir, readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const root = fileURLToPath(new URL('../../', import.meta.url))
const source = path.join(root, '.claude/skills')
const target = path.join(root, '.agents/skills')
const check = process.argv.includes('--check')

async function files(directory) {
  const result = []
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const name = path.join(directory, entry.name)
    if (entry.isDirectory()) result.push(...await files(name))
    else if (entry.isFile()) result.push(name)
    else throw new Error(`Tipo de arquivo inesperado: ${name}`)
  }
  return result
}

if (!check) {
  await mkdir(target, { recursive: true })
  await cp(source, target, { recursive: true })
}
const originals = await files(source)
const copies = await files(target)
if (originals.length !== copies.length) throw new Error('Inventários de skills divergentes')
for (const original of originals) {
  const relative = path.relative(source, original)
  if (!(await readFile(original)).equals(await readFile(path.join(target, relative)))) {
    throw new Error(`Skill divergente: ${relative}. Execute npm run agents:sync`)
  }
}
process.stdout.write(`${originals.length} arquivos de skills sincronizados e verificados.\n`)
