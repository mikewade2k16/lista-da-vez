import { createPinia, setActivePinia } from 'pinia'
import { effectScope, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AssistantChatSurface } from '~/domain/calendar/calendar-chat-api'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { useVoiceRecorder } from './useVoiceRecorder'

interface PendingRequest {
  path: string
  options: { signal?: AbortSignal }
  resolve: (value: unknown) => void
}

class FakeMediaRecorder {
  static autoStop = true
  static instances: FakeMediaRecorder[] = []

  static isTypeSupported() {
    return true
  }

  readonly mimeType = 'audio/webm'
  ondataavailable: ((event: { data: Blob }) => void) | null = null
  onstop: (() => void) | null = null

  constructor() {
    FakeMediaRecorder.instances.push(this)
  }

  start() {}
  pause() {}
  resume() {}

  stop() {
    if (FakeMediaRecorder.autoStop) queueMicrotask(() => this.finish())
  }

  finish(content = 'voice') {
    this.ondataavailable?.({ data: new Blob([content], { type: this.mimeType }) })
    this.onstop?.()
  }
}

const originalNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator')
const originalMediaRecorder = Object.getOwnPropertyDescriptor(globalThis, 'MediaRecorder')

function fetchMock() {
  return (globalThis as typeof globalThis & { $fetch: ReturnType<typeof vi.fn> }).$fetch
}

function installMediaMocks(): void {
  Object.defineProperty(globalThis, 'navigator', {
    configurable: true,
    value: {
      mediaDevices: {
        getUserMedia: vi.fn(async () => ({
          getTracks: () => [{ stop: vi.fn() }],
        })),
      },
    },
  })
  Object.defineProperty(globalThis, 'MediaRecorder', {
    configurable: true,
    value: FakeMediaRecorder,
  })
}

async function flushRecorderStop(): Promise<void> {
  await Promise.resolve()
  await Promise.resolve()
}

describe('useVoiceRecorder context isolation', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    FakeMediaRecorder.autoStop = true
    FakeMediaRecorder.instances = []
    installMediaMocks()

    const auth = useAuthStore()
    auth.accessToken = 'test-token'
    auth.user = { id: 'user-1', name: 'Agency Owner' }
    auth.principal = { role: 'owner', permissions: [], permissionsResolved: true }
    auth.hydrated = true

    useCoreAccountStore().activeAccountId = 'account-a'
  })

  afterEach(() => {
    if (originalNavigator) Object.defineProperty(globalThis, 'navigator', originalNavigator)
    else Reflect.deleteProperty(globalThis, 'navigator')
    if (originalMediaRecorder) {
      Object.defineProperty(globalThis, 'MediaRecorder', originalMediaRecorder)
    } else {
      Reflect.deleteProperty(globalThis, 'MediaRecorder')
    }
  })

  it('aborts and discards a transcription completed after an account switch', async () => {
    const pending: PendingRequest[] = []
    fetchMock().mockImplementation(
      (path: string, options: PendingRequest['options']) =>
        new Promise((resolve) => pending.push({ path, options, resolve })),
    )
    const surface = ref<AssistantChatSurface>('calendar')
    const scope = effectScope()
    const voice = scope.run(() => useVoiceRecorder(() => surface.value))!

    expect(await voice.start()).toBe(true)
    const result = voice.stopAndTranscribe()
    await flushRecorderStop()

    expect(pending).toHaveLength(1)
    expect(pending[0]?.path).toContain('surface=calendar')
    expect(voice.state.value).toBe('transcribing')

    useCoreAccountStore().activeAccountId = 'account-b'

    expect(pending[0]?.options.signal?.aborted).toBe(true)
    expect(voice.state.value).toBe('idle')
    pending[0]?.resolve({ text: 'segredo da conta A' })

    await expect(result).resolves.toBe('')
    expect(voice.errorMessage.value).toBe('')
    scope.stop()
  })

  it('keeps the request bound to its starting surface and rejects a stale response', async () => {
    const pending: PendingRequest[] = []
    fetchMock().mockImplementation(
      (path: string, options: PendingRequest['options']) =>
        new Promise((resolve) => pending.push({ path, options, resolve })),
    )
    const surface = ref<AssistantChatSurface>('calendar')
    const scope = effectScope()
    const voice = scope.run(() => useVoiceRecorder(() => surface.value))!

    expect(await voice.start()).toBe(true)
    const result = voice.stopAndTranscribe()
    await flushRecorderStop()
    expect(pending[0]?.path).toContain('surface=calendar')

    surface.value = 'meta_ads'

    expect(pending[0]?.options.signal?.aborted).toBe(true)
    pending[0]?.resolve({ text: 'resposta tardia do Calendar' })
    await expect(result).resolves.toBe('')
    expect(voice.state.value).toBe('idle')
    scope.stop()
  })

  it('ignores a delayed stop from the cancelled recording after a new account starts recording', async () => {
    FakeMediaRecorder.autoStop = false
    const pending: PendingRequest[] = []
    fetchMock().mockImplementation(
      (path: string, options: PendingRequest['options']) =>
        new Promise((resolve) => pending.push({ path, options, resolve })),
    )
    const surface = ref<AssistantChatSurface>('calendar')
    const scope = effectScope()
    const voice = scope.run(() => useVoiceRecorder(() => surface.value))!

    expect(await voice.start()).toBe(true)
    const firstRecorder = FakeMediaRecorder.instances[0]!

    useCoreAccountStore().activeAccountId = 'account-b'
    expect(voice.state.value).toBe('idle')
    expect(await voice.start()).toBe(true)
    const secondRecorder = FakeMediaRecorder.instances[1]!

    firstRecorder.finish('audio antigo da conta A')
    expect(voice.state.value).toBe('recording')

    const result = voice.stopAndTranscribe()
    secondRecorder.finish('audio novo da conta B')
    await flushRecorderStop()
    expect(pending).toHaveLength(1)
    expect(pending[0]?.path).toContain('surface=calendar')

    pending[0]?.resolve({ text: 'texto da conta B' })
    await expect(result).resolves.toBe('texto da conta B')
    expect(voice.state.value).toBe('idle')
    scope.stop()
  })

  it('single-flights rapid stop gestures before MediaRecorder emits onstop', async () => {
    FakeMediaRecorder.autoStop = false
    const pending: PendingRequest[] = []
    fetchMock().mockImplementation(
      (path: string, options: PendingRequest['options']) =>
        new Promise((resolve) => pending.push({ path, options, resolve })),
    )
    const surface = ref<AssistantChatSurface>('calendar')
    const scope = effectScope()
    const voice = scope.run(() => useVoiceRecorder(() => surface.value))!

    expect(await voice.start()).toBe(true)
    const capture = FakeMediaRecorder.instances[0]!

    const first = voice.stopAndTranscribe()
    const second = voice.stopAndTranscribe()

    expect(voice.state.value).toBe('transcribing')
    await expect(second).resolves.toBe('')

    capture.finish()
    await flushRecorderStop()
    expect(pending).toHaveLength(1)
    pending[0]?.resolve({ text: 'uma unica transcricao' })

    await expect(first).resolves.toBe('uma unica transcricao')
    expect(voice.state.value).toBe('idle')
    scope.stop()
  })

  it('single-flights permission prompts and discards a stream granted after cancellation', async () => {
    let resolvePermission: ((stream: MediaStream) => void) | null = null
    const stoppedTrack = vi.fn()
    const getUserMedia = vi.fn(
      () =>
        new Promise<MediaStream>((resolve) => {
          resolvePermission = resolve
        }),
    )
    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      value: { mediaDevices: { getUserMedia } },
    })
    const surface = ref<AssistantChatSurface>('calendar')
    const scope = effectScope()
    const voice = scope.run(() => useVoiceRecorder(() => surface.value))!

    const first = voice.start()
    await expect(voice.start()).resolves.toBe(false)
    expect(getUserMedia).toHaveBeenCalledTimes(1)

    voice.cancel()
    resolvePermission?.({
      getTracks: () => [{ stop: stoppedTrack }],
    } as unknown as MediaStream)

    await expect(first).resolves.toBe(false)
    expect(stoppedTrack).toHaveBeenCalledOnce()
    expect(FakeMediaRecorder.instances).toHaveLength(0)
    expect(voice.state.value).toBe('idle')
    scope.stop()
  })
})
