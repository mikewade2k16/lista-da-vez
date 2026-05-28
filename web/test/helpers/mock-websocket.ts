type Listener = (event?: any) => void

export class MockWebSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  static instances: MockWebSocket[] = []

  readonly url: string
  readyState = MockWebSocket.CONNECTING
  sent: string[] = []

  private readonly listeners = new Map<string, Set<Listener>>()

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  static reset() {
    MockWebSocket.instances = []
  }

  addEventListener(type: string, listener: Listener) {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, new Set())
    }

    this.listeners.get(type)?.add(listener)
  }

  removeEventListener(type: string, listener: Listener) {
    this.listeners.get(type)?.delete(listener)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    this.emit('close', { code: 1000, reason: '', wasClean: true })
  }

  send(payload: string) {
    this.sent.push(String(payload))
  }

  open() {
    this.readyState = MockWebSocket.OPEN
    this.emit('open', {})
  }

  message(payload: unknown) {
    this.emit('message', {
      data: typeof payload === 'string' ? payload : JSON.stringify(payload),
    })
  }

  error() {
    this.emit('error', {})
  }

  private emit(type: string, event: any) {
    for (const listener of this.listeners.get(type) || []) {
      listener(event)
    }
  }
}
