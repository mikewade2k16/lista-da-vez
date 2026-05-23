import { describe, expect, it } from 'vitest'
import { sanitizeTaskContentHtml, stripHtmlToText } from './content'

describe('stripHtmlToText', () => {
  it('remove tags e colapsa espacos', () => {
    expect(stripHtmlToText('<p>  Ola&nbsp;&nbsp;<strong>mundo</strong> </p>')).toBe('Ola mundo')
  })
})

describe('sanitizeTaskContentHtml', () => {
  it('remove artefatos modal-editor salvos sozinhos', () => {
    expect(
      sanitizeTaskContentHtml(
        '<p> modal-editor-1779228624269  modal-editor-retry-1779228679082 </p>',
      ),
    ).toBe('')
  })

  it('preserva html valido do usuario', () => {
    expect(sanitizeTaskContentHtml('<p>Descricao valida</p>')).toBe('<p>Descricao valida</p>')
  })

  it('nao apaga conteudo misto com texto real', () => {
    expect(sanitizeTaskContentHtml('<p>modal-editor-123 contexto real</p>')).toBe(
      '<p>modal-editor-123 contexto real</p>',
    )
  })
})