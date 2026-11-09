import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { compileScript, parse } from '@vue/compiler-sfc'
import { expect, it } from 'vitest'

const directory = new URL('./components/fiscal/', import.meta.url)
const files = readdirSync(directory)
  .filter((file) => file.endsWith('.vue'))
  .map((file) => new URL(file, directory))
files.push(new URL('../../app/components/calendar/CalendarChatPanel.vue', import.meta.url))
files.push(new URL('../../app/components/calendar/CalendarChatComposer.vue', import.meta.url))

it.each(files.map((file) => [fileURLToPath(file)]))(
  'compiles script and template: %s',
  (filename) => {
    const { descriptor, errors } = parse(readFileSync(filename, 'utf8'), { filename })
    expect(errors).toEqual([])
    expect(() => compileScript(descriptor, { id: filename, inlineTemplate: true })).not.toThrow()
  },
)
