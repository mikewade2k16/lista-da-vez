import { ref } from 'vue'
import { beforeEach, vi } from 'vitest'

class MemoryStorage {
  private readonly values = new Map<string, string>()

  get length() {
    return this.values.size
  }

  clear() {
    this.values.clear()
  }

  getItem(key: string) {
    return this.values.has(key) ? this.values.get(key) || null : null
  }

  key(index: number) {
    return [...this.values.keys()][index] || null
  }

  removeItem(key: string) {
    this.values.delete(key)
  }

  setItem(key: string, value: string) {
    this.values.set(String(key), String(value))
  }
}

const cookieJar = new Map<string, ReturnType<typeof ref>>()
const localStorageMock = new MemoryStorage()
const sessionStorageMock = new MemoryStorage()

const runtimeConfig = {
  apiInternalBase: 'http://localhost:8080',
  public: {
    apiBase: 'http://localhost:8080',
    apiWsBase: 'ws://localhost:8080',
  },
}

const globalObject = globalThis as typeof globalThis & {
  $fetch: ReturnType<typeof vi.fn>
  localStorage: MemoryStorage
  sessionStorage: MemoryStorage
  useCookie: ReturnType<typeof vi.fn>
  useRuntimeConfig: ReturnType<typeof vi.fn>
  window: typeof globalThis
}

Object.defineProperty(globalObject, 'window', {
  value: globalObject,
  configurable: true,
})

globalObject.localStorage = localStorageMock
globalObject.sessionStorage = sessionStorageMock
globalObject.window.localStorage = localStorageMock
globalObject.window.sessionStorage = sessionStorageMock

globalObject.$fetch = vi.fn()
globalObject.useRuntimeConfig = vi.fn(() => runtimeConfig)
globalObject.useCookie = vi.fn((key: string, options: { default?: unknown } = {}) => {
  if (!cookieJar.has(key)) {
    const defaultValue =
      typeof options.default === 'function' ? options.default() : options.default ?? null
    cookieJar.set(key, ref(defaultValue))
  }

  return cookieJar.get(key)
})

beforeEach(() => {
  cookieJar.clear()
  localStorageMock.clear()
  sessionStorageMock.clear()
  globalObject.$fetch.mockReset()
  globalObject.useCookie.mockClear()
  globalObject.useRuntimeConfig.mockClear()
})