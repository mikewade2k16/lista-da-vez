import { describe, expect, it } from 'vitest'

import { sanitizeText } from './sanitize'

describe('sanitizeText', () => {
  it('mascara e-mail, CPF e CNPJ', () => {
    const out = sanitizeText(
      'erro de ana@exemplo.com.br cpf 123.456.789-09 cnpj 12.345.678/0001-95',
    )
    expect(out).toBe('erro de [email] cpf [cpf] cnpj [cnpj]')
  })

  it('mascara sequencias de 13 a 19 digitos e CPF cru', () => {
    expect(sanitizeText('cartao 4111 1111 1111 1111 ok')).toBe('cartao [num] ok')
    expect(sanitizeText('doc 12345678909 fim')).toBe('doc [cpf] fim')
    expect(sanitizeText('pedido 12345')).toBe('pedido 12345')
  })

  it('mascara Bearer e JWT', () => {
    expect(sanitizeText('Authorization: Bearer abc.def-ghi')).toBe('Authorization: Bearer [token]')
    const jwt = 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.c2lnbmF0dXJl'
    expect(sanitizeText(`token ${jwt} fim`)).toBe('token [jwt] fim')
  })

  it('remove querystring', () => {
    expect(sanitizeText('GET https://x.com/a?token=1&b=2 falhou')).toBe(
      'GET https://x.com/a?[query] falhou',
    )
  })

  it('trunca no limite e tolera valores nao string', () => {
    expect(sanitizeText('a'.repeat(5000)).length).toBe(2000)
    expect(sanitizeText(undefined)).toBe('')
    expect(sanitizeText(42)).toBe('42')
  })
})
