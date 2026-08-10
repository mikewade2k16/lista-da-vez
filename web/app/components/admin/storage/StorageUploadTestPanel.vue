<script setup lang="ts">
import { useCoreAccountStore } from '../../../../layers/core/stores/account'
import CalendarMediaViewer from '~/components/calendar/CalendarMediaViewer.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import { useAuthStore } from '~/stores/auth'
import type { StorageObject, StorageSettings } from '~/types/storage'
import { getApiBase } from '~/utils/api-client'
import { formatBytes, type CalendarMediaItem } from '~/utils/calendar'

const props = defineProps<{
  settings: StorageSettings
  enabled: boolean
  initialized: boolean
  saving: boolean
}>()
const emit = defineEmits<{ uploaded: []; toggleUploads: [boolean] }>()

const auth = useAuthStore()
const accountStore = useCoreAccountStore()
const runtimeConfig = useRuntimeConfig()
const input = ref<HTMLInputElement | null>(null)
const selected = ref<File | null>(null)
const uploading = ref(false)
const progress = ref(0)
const phase = ref<'idle' | 'uploading' | 'reading' | 'ready'>('idle')
const errorMessage = ref('')
const uploadedObject = ref<StorageObject | null>(null)
const previewURL = ref('')
const viewerOpen = ref(false)
const dragActive = ref(false)

const category = computed(() => fileCategory(selected.value?.type || ''))
const selectedLimit = computed(() => limitForCategory(category.value))
const canUpload = computed(
  () =>
    props.enabled &&
    props.initialized &&
    props.settings.uploadsEnabled &&
    Boolean(accountStore.activeAccountId) &&
    Boolean(selected.value) &&
    !uploading.value &&
    !errorMessage.value,
)
const viewerItems = computed<CalendarMediaItem[]>(() => {
  const object = uploadedObject.value
  if (
    !object ||
    !previewURL.value ||
    !['image', 'video'].includes(fileCategory(object.contentType))
  ) {
    return []
  }
  return [
    {
      id: object.id,
      name: object.fileName,
      type: fileCategory(object.contentType) as 'image' | 'video',
      url: previewURL.value,
      contentType: object.contentType,
      sizeBytes: object.sizeBytes,
    },
  ]
})

function fileCategory(contentType: string): 'image' | 'video' | 'audio' | 'document' | '' {
  if (contentType.startsWith('image/')) return 'image'
  if (contentType.startsWith('video/')) return 'video'
  if (contentType.startsWith('audio/')) return 'audio'
  if (
    contentType === 'application/pdf' ||
    contentType === 'text/plain' ||
    contentType === 'text/csv' ||
    contentType.includes('officedocument')
  ) {
    return 'document'
  }
  return ''
}

function limitForCategory(value: ReturnType<typeof fileCategory>): number {
  switch (value) {
    case 'image':
      return props.settings.imageMaxBytes
    case 'video':
      return props.settings.videoMaxBytes
    case 'audio':
      return props.settings.audioMaxBytes
    case 'document':
      return props.settings.documentMaxBytes
    default:
      return 0
  }
}

function chooseFile(): void {
  errorMessage.value = ''
  input.value?.click()
}

function onFile(event: Event): void {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0] || null
  target.value = ''
  selectFile(file)
}

function selectFile(file: File | null): void {
  resetResult()
  selected.value = file
  if (!file) return
  const kind = fileCategory(file.type)
  if (!kind) {
    errorMessage.value =
      'Tipo nao permitido. Use imagem, video, audio, PDF, TXT, CSV, DOCX ou XLSX.'
    return
  }
  const limit = limitForCategory(kind)
  if (file.size > limit) {
    errorMessage.value = `${file.name} tem ${formatBytes(file.size)} e supera o limite de ${formatBytes(limit)} para ${kind}.`
  }
}

function onDrop(event: DragEvent): void {
  dragActive.value = false
  selectFile(event.dataTransfer?.files?.[0] || null)
}

function resetResult(): void {
  if (previewURL.value) URL.revokeObjectURL(previewURL.value)
  previewURL.value = ''
  uploadedObject.value = null
  viewerOpen.value = false
  progress.value = 0
  phase.value = 'idle'
  errorMessage.value = ''
}

function upload(): void {
  const file = selected.value
  const accountID = accountStore.activeAccountId
  if (!file || !accountID || !canUpload.value) return

  uploading.value = true
  progress.value = 0
  phase.value = 'uploading'
  errorMessage.value = ''
  const form = new FormData()
  form.append('file', file)
  const xhr = new XMLHttpRequest()
  xhr.open('POST', `${getApiBase(runtimeConfig).replace(/\/$/, '')}/v1/storage/test-upload`)
  xhr.timeout = 15 * 60 * 1000
  if (auth.accessToken) xhr.setRequestHeader('Authorization', `Bearer ${auth.accessToken}`)
  xhr.setRequestHeader('X-Account-Id', accountID)
  xhr.setRequestHeader('Idempotency-Key', crypto.randomUUID())
  xhr.upload.onprogress = (event) => {
    if (event.lengthComputable) progress.value = Math.round((event.loaded / event.total) * 100)
  }
  xhr.upload.onload = () => {
    phase.value = 'reading'
  }
  xhr.onload = () => {
    void finishUpload(xhr, accountID)
  }
  xhr.onerror = () => finishWithError('A conexao com a API foi interrompida durante o envio.')
  xhr.ontimeout = () => finishWithError('O upload nao terminou dentro de 15 minutos.')
  xhr.onabort = () => finishWithError('O upload foi cancelado.')
  xhr.send(form)
}

async function finishUpload(xhr: XMLHttpRequest, accountID: string): Promise<void> {
  if (xhr.status < 200 || xhr.status >= 300) {
    let message = `O servidor recusou o upload (HTTP ${xhr.status}).`
    try {
      const body = JSON.parse(xhr.responseText) as { error?: { message?: string } }
      message = body.error?.message || message
    } catch {
      // Resposta nao-JSON: mantem a mensagem com status.
    }
    finishWithError(message)
    return
  }
  try {
    const body = JSON.parse(xhr.responseText) as { data?: StorageObject }
    if (!body.data?.id) throw new Error('Resposta de upload invalida.')
    uploadedObject.value = body.data
    phase.value = 'reading'
    await loadFromR2(body.data, accountID)
    progress.value = 100
    phase.value = 'ready'
    emit('uploaded')
  } catch (error) {
    finishWithError(
      error instanceof Error ? error.message : 'Nao foi possivel abrir o arquivo salvo.',
    )
    return
  }
  uploading.value = false
}

async function loadFromR2(object: StorageObject, accountID: string): Promise<void> {
  const response = await fetch(
    `${getApiBase(runtimeConfig).replace(/\/$/, '')}/v1/storage/objects/${encodeURIComponent(object.id)}/content`,
    {
      headers: {
        Authorization: `Bearer ${auth.accessToken || ''}`,
        'X-Account-Id': accountID,
      },
      cache: 'no-store',
    },
  )
  if (!response.ok)
    throw new Error(`Upload concluido, mas a leitura do R2 falhou (HTTP ${response.status}).`)
  previewURL.value = URL.createObjectURL(await response.blob())
}

function finishWithError(message: string): void {
  uploading.value = false
  phase.value = 'idle'
  errorMessage.value = message
}

onBeforeUnmount(() => {
  if (previewURL.value) URL.revokeObjectURL(previewURL.value)
})
</script>

<template>
  <section class="storage-test">
    <div class="storage-test__intro">
      <div>
        <h4>Teste completo do R2</h4>
        <p>Envia um arquivo, confirma a gravacao e le o mesmo objeto privado para o preview.</p>
      </div>
      <UBadge color="neutral" variant="soft">1 Classe A + 1 Classe B</UBadge>
    </div>

    <UAlert
      v-if="!enabled || !initialized"
      color="warning"
      variant="soft"
      icon="i-lucide-cloud-cog"
      title="Valide a conexao primeiro"
      description="O bucket precisa estar conectado antes do primeiro upload."
    />

    <div class="storage-test__mode">
      <div>
        <strong>
          {{
            settings.uploadsEnabled
              ? 'Destino ativo: Cloudflare R2'
              : 'Destino ativo: storage local'
          }}
        </strong>
        <span>
          {{
            settings.uploadsEnabled
              ? 'Calendar e Tasks enviam novos arquivos ao R2.'
              : 'Calendar e Tasks preservam o fluxo local anterior.'
          }}
        </span>
      </div>
      <div class="storage-test__mode-switch">
        <AppToggleSwitch
          :model-value="settings.uploadsEnabled"
          :label="`Upload para R2: ${settings.uploadsEnabled ? 'ativado' : 'desativado'}`"
          compact
          :disabled="saving || (!settings.uploadsEnabled && (!enabled || !initialized))"
          @update:model-value="emit('toggleUploads', $event)"
        />
        <UIcon v-if="saving" name="i-lucide-loader-circle" class="animate-spin" />
      </div>
    </div>

    <button
      type="button"
      class="storage-test__drop"
      :class="{ 'storage-test__drop--active': dragActive }"
      :disabled="uploading"
      @click="chooseFile"
      @dragenter.prevent="dragActive = true"
      @dragover.prevent="dragActive = true"
      @dragleave.prevent="dragActive = false"
      @drop.prevent="onDrop"
    >
      <UIcon name="i-lucide-cloud-upload" aria-hidden="true" />
      <span>{{ selected?.name || 'Escolher arquivo para o teste' }}</span>
      <small v-if="selected">
        {{ formatBytes(selected.size) }} · limite {{ formatBytes(selectedLimit) }}
      </small>
      <small v-else>Imagem, video, audio ou documento permitido</small>
    </button>

    <input
      ref="input"
      class="storage-test__input"
      type="file"
      accept="image/jpeg,image/png,image/gif,image/webp,image/avif,video/mp4,video/webm,video/quicktime,audio/mpeg,audio/mp4,audio/wav,audio/ogg,audio/webm,application/pdf,text/plain,text/csv,.docx,.xlsx"
      @change="onFile"
    />

    <div v-if="uploading || phase === 'ready'" class="storage-test__progress" aria-live="polite">
      <div>
        <span>
          {{
            phase === 'uploading'
              ? 'Enviando arquivo'
              : phase === 'reading'
                ? 'Lendo do R2 para validar'
                : 'Upload e leitura concluidos'
          }}
        </span>
        <strong>{{ progress }}%</strong>
      </div>
      <progress :value="progress" max="100"></progress>
    </div>

    <p v-if="errorMessage" class="storage-test__error" role="alert">{{ errorMessage }}</p>

    <div v-if="uploadedObject && previewURL" class="storage-test__preview">
      <button
        v-if="viewerItems.length"
        type="button"
        class="storage-test__media"
        @click="viewerOpen = true"
      >
        <img v-if="category === 'image'" :src="previewURL" :alt="uploadedObject.fileName" />
        <video v-else :src="previewURL" muted preload="metadata"></video>
        <span>
          <UIcon name="i-lucide-expand" />
          Abrir preview
        </span>
      </button>
      <audio v-else-if="category === 'audio'" :src="previewURL" controls></audio>
      <iframe
        v-else-if="uploadedObject.contentType === 'application/pdf'"
        :src="previewURL"
        :title="uploadedObject.fileName"
      ></iframe>
      <div v-else class="storage-test__document">
        <UIcon name="i-lucide-file-check-2" />
        <span>{{ uploadedObject.fileName }}</span>
        <a :href="previewURL" :download="uploadedObject.fileName">Baixar para conferir</a>
      </div>
      <p>
        <UIcon name="i-lucide-circle-check" />
        Salvo e relido do R2 ·
        {{ formatBytes(uploadedObject.sizeBytes) }}
      </p>
    </div>

    <div class="storage-test__actions">
      <UButton
        :label="settings.uploadsEnabled ? 'Enviar teste ao R2' : 'Ative o R2 acima para enviar'"
        icon="i-lucide-upload"
        color="primary"
        :loading="uploading"
        :disabled="!canUpload"
        @click="upload"
      />
    </div>

    <CalendarMediaViewer
      v-if="viewerOpen && viewerItems.length"
      :items="viewerItems"
      :start-index="0"
      @close="viewerOpen = false"
    />
  </section>
</template>

<style scoped>
.storage-test,
.storage-test__intro,
.storage-test__progress,
.storage-test__preview,
.storage-test__document {
  display: flex;
}

.storage-test {
  flex-direction: column;
  gap: 1rem;
}

.storage-test__intro {
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.storage-test h4,
.storage-test p {
  margin: 0;
}

.storage-test h4 {
  color: rgb(var(--text));
  font-size: 0.95rem;
}

.storage-test__intro p {
  margin-top: 0.25rem;
  color: rgb(var(--muted));
  font-size: 0.78rem;
}

.storage-test__drop {
  min-height: 11rem;
  display: grid;
  place-items: center;
  align-content: center;
  gap: 0.45rem;
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface-2) / 0.45);
  color: rgb(var(--text));
  cursor: pointer;
}

.storage-test__drop:hover:not(:disabled) {
  border-color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.06);
}

.storage-test__drop--active {
  border-color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.09);
}

.storage-test__mode {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.8rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface-2) / 0.45);
}

.storage-test__mode > div {
  display: grid;
  gap: 0.2rem;
}

.storage-test__mode > .storage-test__mode-switch {
  display: inline-flex;
  grid-auto-flow: column;
  align-items: center;
  gap: 0.4rem;
}

.storage-test__mode strong {
  color: rgb(var(--text));
  font-size: 0.8rem;
}

.storage-test__mode span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.storage-test__drop > :first-child {
  width: 1.8rem;
  height: 1.8rem;
  color: rgb(var(--primary));
}

.storage-test__drop small {
  color: rgb(var(--muted));
}

.storage-test__input {
  display: none;
}

.storage-test__progress {
  flex-direction: column;
  gap: 0.45rem;
}

.storage-test__progress > div {
  display: flex;
  justify-content: space-between;
  color: rgb(var(--muted));
  font-size: 0.78rem;
}

.storage-test__progress progress {
  width: 100%;
  height: 0.5rem;
  accent-color: rgb(var(--primary));
}

.storage-test__error {
  color: rgb(var(--error));
  font-size: 0.78rem;
}

.storage-test__preview {
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.75rem;
  border: 1px solid rgb(var(--success) / 0.35);
  border-radius: var(--radius-md);
  background: rgb(var(--success) / 0.05);
}

.storage-test__preview > p {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  color: rgb(var(--success));
  font-size: 0.76rem;
  font-weight: 700;
}

.storage-test__media {
  position: relative;
  min-height: 14rem;
  overflow: hidden;
  border: 0;
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  cursor: pointer;
}

.storage-test__media img,
.storage-test__media video {
  width: 100%;
  height: 18rem;
  object-fit: contain;
}

.storage-test__media span {
  position: absolute;
  right: 0.6rem;
  bottom: 0.6rem;
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.3rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--surface) / 0.9);
  color: rgb(var(--text));
  font-size: 0.72rem;
}

.storage-test__preview audio,
.storage-test__preview iframe {
  width: 100%;
}

.storage-test__preview iframe {
  min-height: 24rem;
  border: 0;
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
}

.storage-test__document {
  align-items: center;
  gap: 0.5rem;
  color: rgb(var(--text));
}

.storage-test__document a {
  margin-left: auto;
  color: rgb(var(--primary));
  font-weight: 700;
}

.storage-test__actions {
  display: flex;
  justify-content: flex-end;
}
</style>
