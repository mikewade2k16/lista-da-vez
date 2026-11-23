<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'

import type { CreateQrCodePayload, QrCodeItem, UpdateQrCodePayload } from '~/types/tools'
import { buildQrDataUrl, downloadQrImage } from '~/utils/qr-image'
import { checkQrContrast, qrContrastMessage } from '~/utils/qr-contrast'

interface ClientOption {
  label: string
  value: string
}

// Modal de criar/editar QR. Regras (ver tools/AGENT.md):
//   - nao existe QR escaneavel antes de salvar (so amostra de cores): o QR
//     sempre codifica o qrUrl rastreado que vem do banco;
//   - ao criar, o modal vira edicao com o QR real para baixar;
//   - o slug e fixo depois de criado;
//   - contraste baixo bloqueia o salvar (o back tambem recusa);
//   - a edicao envia so o que mudou (nao pisa num toggle feito na tabela).
const props = defineProps<{
  item: QrCodeItem | null
  canChooseClient: boolean
  clientOptions: ClientOption[]
  activeClientLabel: string
  activeClientId: string
  saving: boolean
  create: (payload: CreateQrCodePayload) => Promise<QrCodeItem | null>
  update: (id: string, payload: UpdateQrCodePayload) => Promise<QrCodeItem | null>
  errorText: () => string
}>()
const open = defineModel<boolean>('open', { required: true })

const current = ref<QrCodeItem | null>(null)
const previewImage = ref('')
const message = ref('')
const errorMessage = ref('')
const form = reactive({
  slug: '',
  targetUrl: '',
  fillColor: '#000000',
  backColor: '#ffffff',
  size: 220,
  isActive: true,
  clientId: '',
})

const mode = computed(() => (current.value ? 'edit' : 'create'))
const contrast = computed(() => checkQrContrast(form.fillColor, form.backColor))
const contrastMessage = computed(() => qrContrastMessage(contrast.value))

function normalizeUrlish(value: string) {
  const raw = String(value ?? '').trim()
  if (!raw) return ''
  return /^https?:\/\//i.test(raw) ? raw : `https://${raw}`
}

function normalizeSize(value: number) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return 220
  return Math.min(1000, Math.max(120, Math.floor(parsed)))
}

function load(item: QrCodeItem | null) {
  current.value = item ? { ...item } : null
  Object.assign(form, {
    slug: item?.slug ?? '',
    targetUrl: item?.targetUrl ?? '',
    fillColor: item?.fillColor ?? '#000000',
    backColor: item?.backColor ?? '#ffffff',
    size: item?.size ?? 220,
    isActive: item?.isActive ?? true,
    clientId: item?.accountId ?? (props.canChooseClient ? props.activeClientId : ''),
  })
  message.value = ''
  errorMessage.value = ''
}

watch(open, (value) => {
  if (value) load(props.item)
})

watch(
  () => [current.value?.qrUrl, form.fillColor, form.backColor, form.size, contrast.value.ok],
  async () => {
    const content = current.value?.qrUrl
    if (!import.meta.client || !content || !contrast.value.ok) {
      previewImage.value = ''
      return
    }
    try {
      previewImage.value = await buildQrDataUrl(
        content,
        form.fillColor,
        form.backColor,
        normalizeSize(form.size),
      )
    } catch {
      previewImage.value = ''
    }
  },
)

async function submit() {
  message.value = ''
  errorMessage.value = ''
  if (!normalizeUrlish(form.targetUrl)) {
    errorMessage.value = 'Informe a URL de destino.'
    return
  }
  if (!contrast.value.ok) {
    errorMessage.value = contrastMessage.value
    return
  }
  const size = normalizeSize(form.size)
  if (!current.value) {
    const created = await props.create({
      targetUrl: form.targetUrl,
      slug: form.slug,
      fillColor: form.fillColor,
      backColor: form.backColor,
      size,
      isActive: form.isActive,
      accountId: props.canChooseClient ? form.clientId : undefined,
    })
    if (!created) {
      errorMessage.value = props.errorText()
      return
    }
    load(created)
    message.value = 'QR criado. Baixe a imagem ao lado.'
    return
  }
  const original = current.value
  const changes: UpdateQrCodePayload = {}
  if (normalizeUrlish(form.targetUrl) !== original.targetUrl) changes.targetUrl = form.targetUrl
  if (form.fillColor !== original.fillColor) changes.fillColor = form.fillColor
  if (form.backColor !== original.backColor) changes.backColor = form.backColor
  if (size !== original.size) changes.size = size
  if (form.isActive !== original.isActive) changes.isActive = form.isActive
  if (!Object.keys(changes).length) {
    open.value = false
    return
  }
  const updated = await props.update(original.id, changes)
  if (!updated) {
    errorMessage.value = props.errorText()
    return
  }
  open.value = false
}
</script>

<template>
  <UModal
    v-model:open="open"
    :title="mode === 'create' ? 'Novo QR Code' : `QR ${current?.slug}`"
    :ui="{ content: 'max-w-3xl' }"
  >
    <template #body>
      <div class="tools-qr-form">
        <div class="tools-qr-form__fields">
          <div class="tools-qr-form__row">
            <UFormField
              :label="mode === 'create' ? 'Slug (opcional)' : 'Slug (fixo)'"
              :help="
                mode === 'create'
                  ? 'Depois de criado nao muda: o QR impresso depende dele.'
                  : 'Fixo para os QR ja impressos continuarem funcionando.'
              "
            >
              <UInput v-model="form.slug" placeholder="meu-qr" :disabled="mode === 'edit'" />
            </UFormField>
            <UFormField label="Tamanho (px)">
              <UInput v-model.number="form.size" type="number" :min="120" :max="1000" />
            </UFormField>
          </div>

          <UFormField label="Destino (URL)">
            <UInput v-model="form.targetUrl" placeholder="https://..." />
          </UFormField>

          <UFormField label="Conta dona">
            <USelect
              v-if="canChooseClient && mode === 'create'"
              v-model="form.clientId"
              :items="clientOptions"
              placeholder="Selecionar conta"
            />
            <UInput v-else :model-value="current?.clientName || activeClientLabel" disabled />
          </UFormField>

          <div class="tools-qr-form__row">
            <UFormField label="Cor do QR">
              <input v-model="form.fillColor" type="color" class="tools-qr-form__color" />
            </UFormField>
            <UFormField label="Cor de fundo">
              <input v-model="form.backColor" type="color" class="tools-qr-form__color" />
            </UFormField>
          </div>

          <USwitch
            v-model="form.isActive"
            size="sm"
            label="QR ativo"
            description="Desativado, quem escanear ve pagina nao encontrada."
          />

          <UAlert
            v-if="contrastMessage"
            color="warning"
            variant="soft"
            icon="i-lucide-triangle-alert"
            :description="contrastMessage"
          />
          <UAlert
            v-if="errorMessage"
            color="error"
            variant="soft"
            icon="i-lucide-alert-triangle"
            :description="errorMessage"
          />
          <UAlert
            v-if="message"
            color="success"
            variant="soft"
            icon="i-lucide-badge-check"
            :description="message"
          />
        </div>

        <aside class="tools-qr-form__preview">
          <div class="tools-qr-form__stage">
            <img v-if="previewImage" :src="previewImage" alt="Preview do QR" />
            <div v-else-if="mode === 'create'" class="tools-qr-form__sample">
              <div
                class="tools-qr-form__swatch"
                :style="{ background: form.backColor }"
                aria-hidden="true"
              >
                <span :style="{ background: form.fillColor }"></span>
              </div>
              <p>O QR e gerado ao salvar, com o link rastreado.</p>
            </div>
            <p v-else class="tools-qr-form__hint">Ajuste as cores para ver o QR.</p>
          </div>
          <p v-if="current" class="tools-qr-form__url">{{ current.qrUrl }}</p>
          <UButton
            v-if="current"
            block
            icon="i-lucide-download"
            label="Baixar QR salvo"
            color="neutral"
            variant="soft"
            @click="downloadQrImage(current)"
          />
        </aside>
      </div>
    </template>

    <template #footer>
      <div class="tools-qr-form__footer">
        <UButton label="Fechar" color="neutral" variant="ghost" @click="open = false" />
        <UButton
          :label="mode === 'create' ? 'Salvar QR' : 'Atualizar QR'"
          icon="i-lucide-save"
          color="primary"
          :loading="saving"
          :disabled="saving || !contrast.ok"
          @click="submit"
        />
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.tools-qr-form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 240px;
  gap: 1rem;
}

.tools-qr-form__fields {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-width: 0;
}

.tools-qr-form__row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 0.75rem;
}

.tools-qr-form__color {
  width: 100%;
  height: 2.25rem;
  padding: 0 0.2rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  cursor: pointer;
}

.tools-qr-form__preview {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.tools-qr-form__stage {
  display: grid;
  place-items: center;
  aspect-ratio: 1;
  padding: 0.5rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
}

.tools-qr-form__stage img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.tools-qr-form__sample {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  color: var(--text-muted);
  font-size: 0.76rem;
  text-align: center;
}

.tools-qr-form__sample p,
.tools-qr-form__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.76rem;
}

.tools-qr-form__swatch {
  display: grid;
  place-items: center;
  width: 5.5rem;
  height: 5.5rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}

.tools-qr-form__swatch span {
  width: 2.75rem;
  height: 2.75rem;
  border-radius: var(--radius-sm);
}

.tools-qr-form__url {
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--text-muted);
  font-size: 0.72rem;
}

.tools-qr-form__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
}

@media (max-width: 720px) {
  .tools-qr-form {
    grid-template-columns: 1fr;
  }

  .tools-qr-form__preview {
    order: -1;
  }

  .tools-qr-form__stage {
    max-width: 200px;
    margin: 0 auto;
    width: 100%;
  }
}
</style>
