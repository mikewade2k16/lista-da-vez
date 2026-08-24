<script setup lang="ts">
import { computed, onMounted, onScopeDispose, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  createAICredential,
  deleteAICredential,
  fetchAICredentials,
  importLegacyAICredentials,
  updateAICredential,
} from '~/domain/omnichannel/config-api'
import type { OmniAICredential } from '~/domain/omnichannel/config-types'

const props = defineProps<{
  disabled?: boolean
  accountId?: string
  credentialBasePath?: string
  allowedProviders?: OmniAICredential['provider'][]
}>()
const emit = defineEmits<{ changed: [] }>()
const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const credentials = ref<OmniAICredential[]>([])
const loading = ref(false)
const busy = ref(false)
const name = ref('')
const provider = ref<OmniAICredential['provider']>('openai')
const apiKey = ref('')
const providerOptions = computed(() => {
  const allowed: OmniAICredential['provider'][] = props.allowedProviders || [
    'openai',
    'gemini',
    'glm',
  ]
  const labels: Record<OmniAICredential['provider'], string> = {
    openai: 'OpenAI',
    anthropic: 'Claude (Anthropic)',
    gemini: 'Gemini',
    glm: 'GLM',
  }
  return allowed.map((value) => ({ value, label: labels[value] }))
})
const nameDrafts = ref<Record<string, string>>({})
const rotateDrafts = ref<Record<string, string>>({})
const contextController = new AbortController()
let disposed = false

function isAbortError(error: unknown): boolean {
  return (
    (error instanceof DOMException && error.name === 'AbortError') ||
    (error as { name?: string } | null)?.name === 'AbortError'
  )
}

function accountRequestOptions(): {
  basePath?: string
  headers?: Record<string, string>
  signal: AbortSignal
} {
  const accountId = String(props.accountId || '').trim()
  const basePath = String(props.credentialBasePath || '').trim()
  return {
    ...(basePath ? { basePath } : {}),
    ...(accountId ? { headers: { 'X-Account-Id': accountId } } : {}),
    signal: contextController.signal,
  }
}

async function load(options = accountRequestOptions()): Promise<void> {
  loading.value = true
  try {
    const loaded = await fetchAICredentials(api, options)
    if (disposed) return
    credentials.value = loaded
    nameDrafts.value = Object.fromEntries(credentials.value.map((item) => [item.id, item.name]))
  } catch (error) {
    if (disposed || isAbortError(error)) return
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar as credenciais de IA.'))
  } finally {
    if (!disposed) loading.value = false
  }
}

async function rename(item: OmniAICredential): Promise<void> {
  const value = (nameDrafts.value[item.id] || '').trim()
  if (item.readOnly || !value || value === item.name || busy.value || props.disabled) return
  const options = accountRequestOptions()
  busy.value = true
  try {
    await updateAICredential(api, item.id, { name: value }, options)
    if (disposed) return
    ui.success('Nome da credencial atualizado.')
    await load(options)
    emit('changed')
  } catch (error) {
    if (disposed || isAbortError(error)) return
    ui.error(getApiErrorMessage(error, 'Não foi possível renomear a credencial.'))
  } finally {
    if (!disposed) busy.value = false
  }
}

async function create(): Promise<void> {
  if (!name.value.trim() || !apiKey.value.trim() || busy.value || props.disabled) return
  const options = accountRequestOptions()
  busy.value = true
  try {
    await createAICredential(
      api,
      {
        name: name.value.trim(),
        provider: provider.value,
        apiKey: apiKey.value.trim(),
      },
      options,
    )
    if (disposed) return
    name.value = ''
    apiKey.value = ''
    ui.success('Credencial salva no cofre da conta.')
    await load(options)
    emit('changed')
  } catch (error) {
    if (disposed || isAbortError(error)) return
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar a credencial.'))
  } finally {
    if (!disposed) busy.value = false
  }
}

async function rotate(item: OmniAICredential): Promise<void> {
  const value = (rotateDrafts.value[item.id] || '').trim()
  if (item.readOnly || !value || busy.value || props.disabled) return
  const options = accountRequestOptions()
  busy.value = true
  try {
    await updateAICredential(api, item.id, { apiKey: value }, options)
    if (disposed) return
    rotateDrafts.value[item.id] = ''
    ui.success(`Chave “${item.name}” atualizada para todos os agentes vinculados.`)
    await load(options)
    emit('changed')
  } catch (error) {
    if (disposed || isAbortError(error)) return
    ui.error(getApiErrorMessage(error, 'Não foi possível atualizar a chave.'))
  } finally {
    if (!disposed) busy.value = false
  }
}

async function remove(item: OmniAICredential): Promise<void> {
  if (
    item.readOnly ||
    busy.value ||
    props.disabled ||
    !window.confirm(`Excluir a credencial “${item.name}”?`)
  )
    return
  const options = accountRequestOptions()
  busy.value = true
  try {
    await deleteAICredential(api, item.id, options)
    if (disposed) return
    ui.success('Credencial excluída.')
    await load(options)
    emit('changed')
  } catch (error) {
    if (disposed || isAbortError(error)) return
    ui.error(
      getApiErrorMessage(
        error,
        'Não foi possível excluir. Remova primeiro os vínculos com versões e análises.',
      ),
    )
  } finally {
    if (!disposed) busy.value = false
  }
}

async function importExisting(): Promise<void> {
  if (busy.value || props.disabled) return
  const options = accountRequestOptions()
  busy.value = true
  try {
    const result = await importLegacyAICredentials(api, options)
    if (disposed) return
    ui.success(
      result.imported > 0
        ? `${result.imported} chave(s) existente(s) importada(s) para o cofre.`
        : 'As chaves existentes já estavam no cofre.',
    )
    await load(options)
    emit('changed')
  } catch (error) {
    if (disposed || isAbortError(error)) return
    ui.error(getApiErrorMessage(error, 'Não foi possível importar as chaves existentes.'))
  } finally {
    if (!disposed) busy.value = false
  }
}

onMounted(() => void load())
watch(
  providerOptions,
  (options) => {
    if (!options.some((option) => option.value === provider.value) && options[0]) {
      provider.value = options[0].value
    }
  },
  { immediate: true },
)
onScopeDispose(() => {
  disposed = true
  contextController.abort()
})
</script>

<template>
  <div class="cfg-credentials">
    <div class="cfg-credentials__intro">
      <div>
        <h3>Chaves de API da conta</h3>
        <p class="calendar-config__hint">
          Cadastre quantas chaves quiser — por exemplo “OpenAI principal” e “OpenAI reserva” — e
          alterne a credencial ativa no Assistente. O navegador só recebe o provedor e os quatro
          últimos caracteres.
        </p>
      </div>
      <AppPanelButton variant="secondary" :disabled="disabled || busy" @click="importExisting">
        Importar chaves existentes
      </AppPanelButton>
    </div>

    <details class="settings-collapse">
      <summary class="settings-collapse__summary">
        <strong class="settings-collapse__title">Nova credencial</strong>
        <span class="material-icons-round settings-collapse__icon">expand_more</span>
      </summary>
      <div class="settings-collapse__body cfg-credentials__form">
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Nome</span>
          <input
            v-model="name"
            class="calendar-config__input"
            placeholder="Ex.: OpenAI principal"
            :disabled="disabled || busy"
          />
        </label>
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Provedor</span>
          <select v-model="provider" class="calendar-config__input" :disabled="disabled || busy">
            <option v-for="option in providerOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>
        <label class="calendar-config__field cfg-credentials__key">
          <span class="calendar-config__field-label">Chave da API</span>
          <input
            v-model="apiKey"
            type="password"
            autocomplete="off"
            class="calendar-config__input"
            :disabled="disabled || busy"
          />
        </label>
        <AppPanelButton
          variant="primary"
          :disabled="disabled || busy || !name.trim() || !apiKey.trim()"
          @click="create"
        >
          Salvar credencial
        </AppPanelButton>
      </div>
    </details>

    <p v-if="loading" class="calendar-config__hint">Carregando credenciais…</p>
    <p v-else-if="credentials.length === 0" class="calendar-config__hint">
      Nenhuma credencial cadastrada.
    </p>
    <template v-else>
      <details v-for="item in credentials" :key="item.id" class="settings-collapse">
        <summary class="settings-collapse__summary">
          <span class="cfg-credentials__summary-copy">
            <strong class="settings-collapse__title">{{ item.name }}</strong>
            <span class="calendar-config__hint">{{ item.provider }} ····{{ item.last4 }}</span>
          </span>
          <span class="material-icons-round settings-collapse__icon">expand_more</span>
        </summary>

        <div class="settings-collapse__body cfg-credentials__item">
          <span v-if="item.readOnly" class="cfg-credentials__shared-badge">
            Compartilhada por {{ item.ownerName || 'agência' }}
          </span>

          <template v-if="!item.readOnly">
            <label class="calendar-config__field">
              <span class="calendar-config__field-label">Apelido</span>
              <input
                v-model="nameDrafts[item.id]"
                class="calendar-config__input"
                aria-label="Nome da credencial"
                :disabled="disabled || busy"
              />
            </label>
            <AppPanelButton
              variant="secondary"
              :disabled="
                disabled ||
                busy ||
                !nameDrafts[item.id]?.trim() ||
                nameDrafts[item.id]?.trim() === item.name
              "
              @click="rename(item)"
            >
              Salvar apelido
            </AppPanelButton>

            <label class="calendar-config__field">
              <span class="calendar-config__field-label">Nova chave</span>
              <input
                v-model="rotateDrafts[item.id]"
                type="password"
                autocomplete="off"
                class="calendar-config__input"
                placeholder="Cole somente para rotacionar"
                :disabled="disabled || busy"
              />
            </label>
            <div class="cfg-credentials__actions">
              <AppPanelButton
                variant="secondary"
                :disabled="disabled || busy || !rotateDrafts[item.id]?.trim()"
                @click="rotate(item)"
              >
                Atualizar chave
              </AppPanelButton>
              <AppPanelButton variant="ghost" :disabled="disabled || busy" @click="remove(item)">
                Excluir
              </AppPanelButton>
            </div>
          </template>

          <p v-else class="cfg-credentials__shared-note">
            Disponível para seleção nesta conta; o segredo só pode ser alterado na agência de
            origem.
          </p>
        </div>
      </details>
    </template>
  </div>
</template>

<style scoped>
.cfg-credentials {
  display: grid;
  gap: 0.8rem;
  min-width: 0;
  max-width: 100%;
}
.cfg-credentials__intro {
  display: flex;
  align-items: flex-start;
  gap: 0.7rem;
  justify-content: space-between;
  flex-wrap: wrap;
  min-width: 0;
}
.cfg-credentials__intro h3,
.cfg-credentials__intro p,
.cfg-credentials__item p {
  margin: 0;
}
.cfg-credentials__summary-copy {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
}
.cfg-credentials__shared-badge {
  width: fit-content;
  padding: 0.18rem 0.45rem;
  border: 1px solid rgb(var(--primary) / 0.4);
  border-radius: 999px;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  font-size: 0.68rem;
  font-weight: 700;
}
.cfg-credentials__shared-note {
  max-width: 22rem;
  color: var(--text-muted);
  font-size: 0.74rem;
  line-height: 1.45;
}
.cfg-credentials__form {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.7rem;
  min-width: 0;
}
.cfg-credentials__item {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0.65rem;
  min-width: 0;
  max-width: 100%;
}
.cfg-credentials__item .calendar-config__input {
  width: 100%;
  max-width: 100%;
}
.cfg-credentials__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
@media (max-width: 800px) {
  .cfg-credentials__intro {
    display: grid;
  }
  .cfg-credentials__intro :deep(button) {
    width: 100%;
  }
}
</style>
