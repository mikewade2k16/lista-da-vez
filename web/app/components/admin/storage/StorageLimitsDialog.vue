<script setup lang="ts">
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import StorageUploadTestPanel from '~/components/admin/storage/StorageUploadTestPanel.vue'
import type { StorageSettings, StorageSettingsInput } from '~/types/storage'
import {
  STORAGE_LIMITS,
  storageDraftToInput,
  storageSettingsToDraft,
  validateStorageLimitsDraft,
  type StorageLimitsDraft,
} from '~/domain/storage/limits'

const props = defineProps<{
  open: boolean
  settings: StorageSettings
  saving: boolean
  enabled: boolean
  initialized: boolean
}>()
const emit = defineEmits<{
  'update:open': [boolean]
  save: [StorageSettingsInput]
  uploaded: []
  toggleUploads: [boolean]
}>()

type DrawerMode = 'side' | 'center' | 'fullscreen'
type StorageTab = 'limits' | 'upload'
const mode = ref<DrawerMode>('side')
const activeTab = ref<StorageTab>('limits')
const draft = reactive<StorageLimitsDraft>(storageSettingsToDraft(props.settings))
const submitAttempted = ref(false)
const validationMessage = computed(() => validateStorageLimitsDraft(draft))
const fileFields: Array<{
  field: keyof Pick<
    StorageLimitsDraft,
    'imageMebibytes' | 'videoMebibytes' | 'audioMebibytes' | 'documentMebibytes'
  >
  label: string
  hint: string
  icon: string
}> = [
  {
    field: 'imageMebibytes',
    label: 'Imagens',
    hint: 'JPG, PNG, WebP, GIF e AVIF.',
    icon: 'i-lucide-image',
  },
  { field: 'videoMebibytes', label: 'Videos', hint: 'MP4, WebM e MOV.', icon: 'i-lucide-film' },
  {
    field: 'audioMebibytes',
    label: 'Audios',
    hint: 'MP3, M4A, WAV, OGG e WebM.',
    icon: 'i-lucide-audio-lines',
  },
  {
    field: 'documentMebibytes',
    label: 'Documentos',
    hint: 'PDF, TXT, CSV, DOCX e XLSX.',
    icon: 'i-lucide-file-text',
  },
]

watch(
  () => [props.open, props.settings] as const,
  ([open]) => {
    if (!open) return
    Object.assign(draft, storageSettingsToDraft(props.settings))
    submitAttempted.value = false
  },
)

function setNumber(
  field: Exclude<keyof StorageLimitsDraft, 'uploadsEnabled'>,
  value: unknown,
): void {
  draft[field] = Number(value)
}

function submit(): void {
  submitAttempted.value = true
  if (validationMessage.value) return
  emit('save', storageDraftToInput(draft))
}

function onDrawerModel(value: boolean): void {
  if (value || !props.open) return
  emit('update:open', false)
}
</script>

<template>
  <OmniEntityDrawer
    v-model:mode="mode"
    :model-value="open"
    title="Configurar Storage R2"
    subtitle="Limites globais, limites por tipo e teste controlado de upload."
    @update:model-value="onDrawerModel"
  >
    <div class="calendar-config-drawer storage-config">
      <nav class="calendar-config__tabs" aria-label="Secoes da configuracao do storage">
        <button
          type="button"
          class="calendar-config__tab"
          :class="{ 'is-active': activeTab === 'limits' }"
          @click="activeTab = 'limits'"
        >
          <UIcon name="i-lucide-shield-check" aria-hidden="true" />
          <span>Limites</span>
        </button>
        <button
          type="button"
          class="calendar-config__tab"
          :class="{ 'is-active': activeTab === 'upload' }"
          @click="activeTab = 'upload'"
        >
          <UIcon name="i-lucide-cloud-upload" aria-hidden="true" />
          <span>Teste de upload</span>
        </button>
      </nav>

      <div class="calendar-config-drawer__panel">
        <section v-show="activeTab === 'limits'" class="storage-config__limits">
          <div class="storage-config__section-heading">
            <div>
              <h4>Franquia mensal</h4>
              <p>Reservas conservadoras bloqueiam a operacao antes de ultrapassar estes tetos.</p>
            </div>
          </div>

          <div class="storage-config__field">
            <div class="storage-config__field-heading">
              <div>
                <label>Início do ciclo de faturamento</label>
                <p>Use o mesmo dia exibido no painel de faturamento da Cloudflare.</p>
              </div>
              <UInput
                :model-value="draft.billingCycleDay"
                type="number"
                :min="STORAGE_LIMITS.billingCycleDay.min"
                :max="STORAGE_LIMITS.billingCycleDay.max"
                :step="STORAGE_LIMITS.billingCycleDay.step"
                class="storage-config__input"
                @update:model-value="setNumber('billingCycleDay', $event)"
              >
                <template #trailing><span>dia</span></template>
              </UInput>
            </div>
            <USlider
              :model-value="draft.billingCycleDay"
              :min="STORAGE_LIMITS.billingCycleDay.min"
              :max="STORAGE_LIMITS.billingCycleDay.max"
              :step="STORAGE_LIMITS.billingCycleDay.step"
              @update:model-value="setNumber('billingCycleDay', $event)"
            />
          </div>

          <div class="storage-config__field">
            <div class="storage-config__field-heading">
              <div>
                <label>Armazenamento</label>
                <p>Teto oficial: 10 GB-mes.</p>
              </div>
              <UInput
                :model-value="draft.storageGigabytes"
                type="number"
                :min="STORAGE_LIMITS.storageGigabytes.min"
                :max="STORAGE_LIMITS.storageGigabytes.max"
                :step="STORAGE_LIMITS.storageGigabytes.step"
                class="storage-config__input"
                @update:model-value="setNumber('storageGigabytes', $event)"
              >
                <template #trailing><span>GB</span></template>
              </UInput>
            </div>
            <USlider
              :model-value="draft.storageGigabytes"
              :min="STORAGE_LIMITS.storageGigabytes.min"
              :max="STORAGE_LIMITS.storageGigabytes.max"
              :step="STORAGE_LIMITS.storageGigabytes.step"
              @update:model-value="setNumber('storageGigabytes', $event)"
            />
          </div>

          <div class="storage-config__budget-grid">
            <div class="storage-config__field">
              <div class="storage-config__field-heading">
                <div>
                  <label>Operacoes Classe A</label>
                  <p>Gravacoes e listagens · teto 1 milhao/mes.</p>
                </div>
                <UInput
                  :model-value="draft.classARequests"
                  type="number"
                  :min="STORAGE_LIMITS.classARequests.min"
                  :max="STORAGE_LIMITS.classARequests.max"
                  class="storage-config__input"
                  @update:model-value="setNumber('classARequests', $event)"
                />
              </div>
              <USlider
                :model-value="draft.classARequests"
                :min="STORAGE_LIMITS.classARequests.min"
                :max="STORAGE_LIMITS.classARequests.max"
                :step="STORAGE_LIMITS.classARequests.step"
                @update:model-value="setNumber('classARequests', $event)"
              />
            </div>
            <div class="storage-config__field">
              <div class="storage-config__field-heading">
                <div>
                  <label>Operacoes Classe B</label>
                  <p>Leituras · teto 10 milhoes/mes.</p>
                </div>
                <UInput
                  :model-value="draft.classBRequests"
                  type="number"
                  :min="STORAGE_LIMITS.classBRequests.min"
                  :max="STORAGE_LIMITS.classBRequests.max"
                  class="storage-config__input"
                  @update:model-value="setNumber('classBRequests', $event)"
                />
              </div>
              <USlider
                :model-value="draft.classBRequests"
                :min="STORAGE_LIMITS.classBRequests.min"
                :max="STORAGE_LIMITS.classBRequests.max"
                :step="STORAGE_LIMITS.classBRequests.step"
                @update:model-value="setNumber('classBRequests', $event)"
              />
            </div>
          </div>

          <div class="storage-config__section-heading storage-config__section-heading--types">
            <div>
              <h4>Limite por tipo de arquivo</h4>
              <p>O backend detecta e valida o MIME antes de reservar espaco e gravar no R2.</p>
            </div>
            <UBadge color="neutral" variant="soft">Padrao inicial 25 MiB</UBadge>
          </div>

          <div class="storage-config__type-grid">
            <div v-for="item in fileFields" :key="item.field" class="storage-config__field">
              <div class="storage-config__field-heading">
                <div class="storage-config__type-copy">
                  <UIcon :name="item.icon" aria-hidden="true" />
                  <div>
                    <label>{{ item.label }}</label>
                    <p>{{ item.hint }}</p>
                  </div>
                </div>
                <UInput
                  :model-value="draft[item.field]"
                  type="number"
                  :min="STORAGE_LIMITS.fileMebibytes.min"
                  :max="STORAGE_LIMITS.fileMebibytes.max"
                  :step="STORAGE_LIMITS.fileMebibytes.step"
                  class="storage-config__input"
                  @update:model-value="setNumber(item.field, $event)"
                >
                  <template #trailing><span>MiB</span></template>
                </UInput>
              </div>
              <USlider
                :model-value="draft[item.field]"
                :min="STORAGE_LIMITS.fileMebibytes.min"
                :max="STORAGE_LIMITS.fileMebibytes.max"
                :step="STORAGE_LIMITS.fileMebibytes.step"
                @update:model-value="setNumber(item.field, $event)"
              />
            </div>
          </div>

          <UAlert
            v-if="submitAttempted && validationMessage"
            color="error"
            variant="soft"
            icon="i-lucide-alert-triangle"
            title="Revise os limites"
            :description="validationMessage"
          />
        </section>

        <StorageUploadTestPanel
          v-if="activeTab === 'upload'"
          :settings="settings"
          :enabled="enabled"
          :initialized="initialized"
          :saving="saving"
          @uploaded="emit('uploaded')"
          @toggle-uploads="emit('toggleUploads', $event)"
        />
      </div>
    </div>

    <template v-if="activeTab === 'limits'" #footer>
      <AppPanelButton variant="primary" :disabled="saving" @click="submit">
        {{ saving ? 'Salvando...' : 'Salvar limites' }}
      </AppPanelButton>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.storage-config__limits,
.storage-config__field,
.storage-config__section-heading {
  display: flex;
}

.storage-config__limits,
.storage-config__field {
  flex-direction: column;
}

.storage-config__limits {
  gap: 1rem;
}

.storage-config__section-heading {
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.storage-config__section-heading--types {
  margin-top: 0.3rem;
  padding-top: 1rem;
  border-top: 1px solid rgb(var(--border));
}

.storage-config h4,
.storage-config p {
  margin: 0;
}

.storage-config h4,
.storage-config label {
  color: rgb(var(--text));
  font-weight: 750;
}

.storage-config h4 {
  font-size: 0.95rem;
}

.storage-config p,
.storage-config__input span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.storage-config__section-heading p {
  margin-top: 0.25rem;
  font-size: 0.78rem;
}

.storage-config__budget-grid,
.storage-config__type-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.storage-config__field {
  gap: 0.65rem;
  padding: 0.85rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface-2) / 0.38);
}

.storage-config__field-heading,
.storage-config__type-copy {
  display: flex;
  align-items: center;
  gap: 0.7rem;
}

.storage-config__field-heading {
  justify-content: space-between;
}

.storage-config__type-copy > :first-child {
  flex: 0 0 auto;
  color: rgb(var(--primary));
}

.storage-config__input {
  width: 9.2rem;
  flex: 0 0 auto;
}

@media (max-width: 720px) {
  .storage-config__budget-grid,
  .storage-config__type-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .storage-config__field-heading {
    align-items: stretch;
    flex-direction: column;
  }

  .storage-config__input {
    width: 100%;
  }
}
</style>
