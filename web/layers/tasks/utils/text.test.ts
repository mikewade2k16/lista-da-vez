import { describe, expect, it } from 'vitest'
import { clampText, normalizeText } from './text'

describe('normalizeText (usar em flush/autosave)', () => {
  it('faz trim e colapsa whitespaces internos', () => {
    expect(normalizeText('  hello   world  ')).toBe('hello world')
  })

  it('clampa pelo tamanho maximo', () => {
    expect(normalizeText('abcdef', 3)).toBe('abc')
  })

  it('aceita null/undefined sem panicar', () => {
    expect(normalizeText(null)).toBe('')
    expect(normalizeText(undefined)).toBe('')
  })

  it('converte numeros para string', () => {
    expect(normalizeText(42)).toBe('42')
  })
})

describe('clampText (usar em @update:model-value de inputs controlados)', () => {
  it('PRESERVA espaco no final — chave do bugfix T7.2', () => {
    // E' o caso que estourava: usuario digita "abc " e o cursor pulava porque normalizeText
    // trimava. clampText DEVE manter o espaco trailing intacto.
    expect(clampText('abc ')).toBe('abc ')
    expect(clampText('palavra duas ')).toBe('palavra duas ')
  })

  it('preserva multiplos espacos internos (sem colapso)', () => {
    expect(clampText('a  b   c')).toBe('a  b   c')
  })

  it('clampa pelo tamanho mas nao trima', () => {
    expect(clampText('   hello   ', 8)).toBe('   hello')
  })

  it('aceita null/undefined sem panicar', () => {
    expect(clampText(null)).toBe('')
    expect(clampText(undefined)).toBe('')
  })

  it('diferenca essencial entre os dois helpers', () => {
    // T7.2: normalizeText e' destrutivo para digitacao em curso; clampText preserva.
    const partial = 'palavra '
    expect(normalizeText(partial)).toBe('palavra') // -> input "salta"
    expect(clampText(partial)).toBe('palavra ')    // -> usuario continua digitando
  })
})
