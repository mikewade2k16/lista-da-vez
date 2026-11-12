import type { InjectionKey } from 'vue'
import { createApiRequest } from '~/utils/api-client'
export interface ScriptImages {
  display: (html: string) => Promise<string>
  persist: (html: string) => string
  upload: (file: File) => Promise<string>
  dispose: () => void
}
export const scriptImagesKey: InjectionKey<ScriptImages> = Symbol('script-images')
export function useScriptImages(accountId: { value: string }): ScriptImages {
  const auth = useAuthStore()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const urls = new Map<string, string>()
  const pending = new Map<string, Promise<string>>()
  let generation = 0
  async function imageUrl(path: string) {
    if (urls.has(path)) return urls.get(path)!
    if (pending.has(path)) return pending.get(path)!
    const current = generation
    const scope = accountId.value
    const request = (async () => {
      const blob = (await api(path, {
        headers: { 'X-Account-Id': scope },
        responseType: 'blob',
      })) as Blob
      if (current !== generation || scope !== accountId.value) return ''
      const url = URL.createObjectURL(blob)
      urls.set(path, url)
      return url
    })()
    pending.set(path, request)
    try {
      return await request
    } finally {
      pending.delete(path)
    }
  }
  async function display(html: string) {
    const paths = [...new Set(html.match(/\/v1\/tools\/scripts\/images\/[a-f\d-]{36}/gi) || [])]
    let rendered = html
    for (const path of paths) rendered = rendered.split(path).join(await imageUrl(path))
    return rendered
  }
  function persist(html: string) {
    let saved = html
    for (const [path, url] of urls) saved = saved.split(url).join(path)
    return saved
  }
  async function upload(file: File) {
    const scope = accountId.value
    const body = new FormData()
    body.append('file', file)
    const result = (await api('/v1/tools/scripts/images', {
      method: 'POST',
      body,
      headers: { 'X-Account-Id': scope },
    })) as { data: { url: string } }
    if (scope !== accountId.value) throw new Error('A conta ativa mudou durante o envio.')
    return imageUrl(result.data.url)
  }
  function dispose() {
    generation++
    urls.forEach((url) => URL.revokeObjectURL(url))
    urls.clear()
    pending.clear()
  }
  return { display, persist, upload, dispose }
}
