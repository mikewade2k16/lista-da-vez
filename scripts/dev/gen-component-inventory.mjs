import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '..', '..')
const docsOutputPath = path.join(repoRoot, 'docs', 'COMPONENT_INVENTORY_AUTO.md')

const STATIC_ROOTS = [
  {
    label: 'web/app/components',
    dir: path.join(repoRoot, 'web', 'app', 'components'),
  },
  {
    label: 'web/app/features',
    dir: path.join(repoRoot, 'web', 'app', 'features'),
  },
]

const IMPORT_REGEX = /import\s+([\s\S]*?)\s+from\s+['"]([^'"]+)['"];?/g
const TIPTAP_REGEX = /@tiptap\/|\btiptap\b|\bEditorContent\b|\buseEditor\b/i
const PINIA_REGEX = /\bfrom\s+['"]pinia['"]|\bstoreToRefs\s*\(|\buse[A-Z][A-Za-z0-9_]*Store\s*\(/m
const SCOPED_STYLE_REGEX = /<style\b[^>]*\bscoped\b/i

function normalizePath(filePath) {
  return filePath.split(path.sep).join('/')
}

function countLines(content) {
  if (!content) {
    return 0
  }

  return content.split(/\r?\n/).length
}

async function pathExists(targetPath) {
  try {
    await fs.access(targetPath)
    return true
  } catch {
    return false
  }
}

async function collectLayerRoots() {
  const layersDir = path.join(repoRoot, 'web', 'layers')

  if (!(await pathExists(layersDir))) {
    return []
  }

  const entries = await fs.readdir(layersDir, { withFileTypes: true })

  return entries
    .filter((entry) => entry.isDirectory())
    .map((entry) => ({
      label: `web/layers/${entry.name}/components`,
      dir: path.join(layersDir, entry.name, 'components'),
    }))
    .sort((left, right) => left.label.localeCompare(right.label, 'en'))
}

async function walkVueFiles(dirPath) {
  const results = []
  const entries = await fs.readdir(dirPath, { withFileTypes: true })

  entries.sort((left, right) => left.name.localeCompare(right.name, 'en'))

  for (const entry of entries) {
    const fullPath = path.join(dirPath, entry.name)

    if (entry.isDirectory()) {
      results.push(...(await walkVueFiles(fullPath)))
      continue
    }

    if (entry.isFile() && entry.name.endsWith('.vue')) {
      results.push(fullPath)
    }
  }

  return results
}

function extractImportedBindings(specifier) {
  const normalized = String(specifier || '')
    .replace(/\btype\b/g, ' ')
    .replace(/\r?\n/g, ' ')
    .trim()

  if (!normalized) {
    return []
  }

  const collected = []
  const namedMatch = normalized.match(/\{([\s\S]*?)\}/)

  if (namedMatch) {
    for (const item of namedMatch[1].split(',')) {
      const candidate = item
        .split(' as ')[0]
        .trim()
      if (candidate) {
        collected.push(candidate)
      }
    }
  }

  const withoutNamed = normalized.replace(/\{[\s\S]*?\}/g, ' ').trim()
  for (const part of withoutNamed.split(',')) {
    const candidate = part
      .replace(/^\*\s+as\s+/, '')
      .trim()
    if (candidate) {
      collected.push(candidate)
    }
  }

  return [...new Set(collected)]
}

function extractComposableReferences(content) {
  const references = new Set()

  for (const match of content.matchAll(IMPORT_REGEX)) {
    const specifier = String(match[1] || '').trim()
    const source = normalizePath(String(match[2] || '').trim())

    if (!source.includes('composables')) {
      continue
    }

    const baseName = path.posix.basename(source).replace(/\.(vue|mjs|cjs|js|mts|cts|ts|tsx)$/, '')

    if (baseName && baseName !== 'index') {
      references.add(baseName)
      continue
    }

    for (const imported of extractImportedBindings(specifier)) {
      references.add(imported)
    }
  }

  return [...references].sort((left, right) => left.localeCompare(right, 'en'))
}

function formatBool(value) {
  return value ? 'yes' : 'no'
}

function formatCell(value) {
  return String(value || '').replace(/\|/g, '\\|')
}

function renderSummaryRows(sections) {
  const rows = sections.map((section) => {
    const records = section.records
    return `| ${formatCell(section.label)} | ${records.length} | ${records.reduce((sum, record) => sum + record.lineCount, 0)} | ${records.filter((record) => record.hasScopedStyle).length} | ${records.filter((record) => record.usesTiptap).length} | ${records.filter((record) => record.usesPinia).length} | ${records.filter((record) => record.composableRefs.length > 0).length} |`
  })

  const allRecords = sections.flatMap((section) => section.records)
  rows.push(
    `| TOTAL | ${allRecords.length} | ${allRecords.reduce((sum, record) => sum + record.lineCount, 0)} | ${allRecords.filter((record) => record.hasScopedStyle).length} | ${allRecords.filter((record) => record.usesTiptap).length} | ${allRecords.filter((record) => record.usesPinia).length} | ${allRecords.filter((record) => record.composableRefs.length > 0).length} |`,
  )

  return rows.join('\n')
}

function renderSection(section) {
  if (!section.exists) {
    return [`## ${section.label}`, '', 'Diretorio inexistente no momento.', ''].join('\n')
  }

  if (!section.records.length) {
    return [`## ${section.label}`, '', 'Nenhum componente `.vue` encontrado.', ''].join('\n')
  }

  const rows = section.records.map((record) => {
    const composables = record.composableRefs.length ? record.composableRefs.join(', ') : '-'
    return `| ${formatCell(record.relativePath)} | ${record.lineCount} | ${formatBool(record.hasScopedStyle)} | ${formatBool(record.usesTiptap)} | ${formatBool(record.usesPinia)} | ${formatCell(composables)} |`
  })

  return [
    `## ${section.label}`,
    '',
    `Componentes encontrados: ${section.records.length}`,
    '',
    '| Arquivo | Linhas | style scoped | TipTap | Pinia | Composables externos |',
    '| --- | ---: | --- | --- | --- | --- |',
    rows.join('\n'),
    '',
  ].join('\n')
}

function buildMarkdown(sections) {
  const generatedAt = new Date().toISOString()

  return [
    '# COMPONENT_INVENTORY_AUTO',
    '',
    '> Arquivo gerado automaticamente por `npm run inventory`. Nao editar manualmente.',
    `> Gerado em: ${generatedAt}`,
    '',
    '## Escopo',
    '',
    '- `web/app/components/`',
    '- `web/app/features/`',
    '- `web/layers/*/components/`',
    '',
    '## Resumo',
    '',
    '| Secao | Componentes | Linhas | Scoped | TipTap | Pinia | Com composables |',
    '| --- | ---: | ---: | ---: | ---: | ---: | ---: |',
    renderSummaryRows(sections),
    '',
    '## Regras de deteccao',
    '',
    '- `style scoped`: busca por bloco `<style scoped>`.',
    '- `TipTap`: busca por imports ou referencias `@tiptap/*`, `tiptap`, `EditorContent` ou `useEditor`.',
    '- `Pinia`: busca por import de `pinia`, `storeToRefs()` ou chamada `use*Store()`.',
    '- `Composables externos`: lista imports cujo caminho contem `composables`.',
    '',
    ...sections.map((section) => renderSection(section)),
  ].join('\n')
}

async function analyzeFile(filePath) {
  const content = await fs.readFile(filePath, 'utf8')

  return {
    relativePath: normalizePath(path.relative(repoRoot, filePath)),
    lineCount: countLines(content),
    hasScopedStyle: SCOPED_STYLE_REGEX.test(content),
    usesTiptap: TIPTAP_REGEX.test(content),
    usesPinia: PINIA_REGEX.test(content),
    composableRefs: extractComposableReferences(content),
  }
}

async function collectSections() {
  const roots = [...STATIC_ROOTS, ...(await collectLayerRoots())]
  const sections = []

  for (const root of roots) {
    const exists = await pathExists(root.dir)

    if (!exists) {
      sections.push({ ...root, exists: false, records: [] })
      continue
    }

    const files = await walkVueFiles(root.dir)
    const records = await Promise.all(files.map((filePath) => analyzeFile(filePath)))

    records.sort((left, right) => left.relativePath.localeCompare(right.relativePath, 'en'))
    sections.push({ ...root, exists: true, records })
  }

  return sections
}

async function main() {
  const sections = await collectSections()
  const markdown = buildMarkdown(sections)

  await fs.writeFile(docsOutputPath, markdown, 'utf8')

  const totalComponents = sections.reduce((sum, section) => sum + section.records.length, 0)
  console.log(`Generated docs/COMPONENT_INVENTORY_AUTO.md with ${totalComponents} Vue components.`)
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})