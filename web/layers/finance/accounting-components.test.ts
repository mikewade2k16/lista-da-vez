import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { compileScript, parse } from '@vue/compiler-sfc'
import { expect, it } from 'vitest'
const directory = new URL('./components/accounting/', import.meta.url)
const files = readdirSync(directory)
  .filter((file) => file.endsWith('.vue'))
  .map((file) => fileURLToPath(new URL(file, directory)))
files.push(
  fileURLToPath(new URL('../../app/components/calendar/CalendarChatPanel.vue', import.meta.url)),
)
files.push(
  fileURLToPath(new URL('../../app/components/assistant/OmniAssistantHost.vue', import.meta.url)),
)
it.each(files)('compiles the integrated component and template: %s', (filename) => {
  const { descriptor, errors } = parse(readFileSync(filename, 'utf8'), { filename })
  expect(errors).toEqual([])
  expect(() => compileScript(descriptor, { id: filename, inlineTemplate: true })).not.toThrow()
})
