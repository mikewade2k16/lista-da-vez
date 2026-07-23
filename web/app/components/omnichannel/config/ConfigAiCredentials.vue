<script setup lang="ts">
import { onMounted, ref } from 'vue'
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

defineProps<{ disabled?: boolean }>()
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
const nameDrafts = ref<Record<string, string>>({})
const rotateDrafts = ref<Record<string, string>>({})

async function load(): Promise<void> {
  loading.value = true
  try {
    credentials.value = await fetchAICredentials(api)
    nameDrafts.value = Object.fromEntries(credentials.value.map((item) => [item.id, item.name]))
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar as credenciais de IA.'))
  } finally {
    loading.value = false
  }
}

async function rename(item: OmniAICredential): Promise<void> {
  const value = (nameDrafts.value[item.id] || '').trim()
  if (!value || value === item.name || busy.value) return
  busy.value = true
  try {
    await updateAICredential(api, item.id, { name: value })
    ui.success('Nome da credencial atualizado.')
    await load()
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível renomear a credencial.'))
  } finally {
    busy.value = false
  }
}

async function create(): Promise<void> {
  if (!name.value.trim() || !apiKey.value.trim() || busy.value) return
  busy.value = true
  try {
    await createAICredential(api, {
      name: name.value.trim(),
      provider: provider.value,
      apiKey: apiKey.value.trim(),
    })
    name.value = ''
    apiKey.value = ''
    ui.success('Credencial salva no cofre da conta.')
    await load()
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar a credencial.'))
  } finally {
    busy.value = false
  }
}

async function rotate(item: OmniAICredential): Promise<void> {
  const value = (rotateDrafts.value[item.id] || '').trim()
  if (!value || busy.value) return
  busy.value = true
  try {
    await updateAICredential(api, item.id, { apiKey: value })
    rotateDrafts.value[item.id] = ''
    ui.success(`Chave “${item.name}” atualizada para todos os agentes vinculados.`)
    await load()
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível atualizar a chave.'))
  } finally {
    busy.value = false
  }
}

async function remove(item: OmniAICredential): Promise<void> {
  if (busy.value || !window.confirm(`Excluir a credencial “${item.name}”?`)) return
  busy.value = true
  try {
    await deleteAICredential(api, item.id)
    ui.success('Credencial excluída.')
    await load()
    emit('changed')
  } catch (error) {
    ui.error(
      getApiErrorMessage(
        error,
        'Não foi possível excluir. Remova primeiro os vínculos com versões e análises.',
      ),
    )
  } finally {
    busy.value = false
  }
}

async function importExisting(): Promise<void> {
  if (busy.value) return
  busy.value = true
  try {
    const result = await importLegacyAICredentials(api)
    ui.success(
      result.imported > 0
        ? `${result.imported} chave(s) existente(s) importada(s) para o cofre.`
        : 'As chaves existentes já estavam no cofre.',
    )
    await load()
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível importar as chaves existentes.'))
  } finally {
    busy.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="cfg-credentials">
    <div class="cfg-credentials__intro">
      <div>
        <h3>Chaves de API da conta</h3>
        <p class="calendar-config__hint">
          Dê um nome para cada chave e reutilize-a em vários agentes. O navegador só recebe o
          provedor e os quatro últimos caracteres.
        </p>
      </div>
      <AppPanelButton variant="secondary" :disabled="disabled || busy" @click="importExisting">
        Importar chaves existentes
      </AppPanelButton>
    </div>

    <details class="settings-collapse" open>
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
            placeholder="openai_mk"
            :disabled="disabled || busy"
          />
        </label>
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Provedor</span>
          <select v-model="provider" class="calendar-config__input" :disabled="disabled || busy">
            <option value="openai">OpenAI</option>
            <option value="gemini">Gemini</option>
            <option value="glm">GLM</option>
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
      <article v-for="item in credentials" :key="item.id" class="cfg-credentials__item">
        <div class="cfg-credentials__identity">
          <input
            v-model="nameDrafts[item.id]"
            class="calendar-config__input"
            aria-label="Nome da credencial"
            :disabled="disabled || busy"
          />
          <p class="calendar-config__hint">{{ item.provider }} ····{{ item.last4 }}</p>
        </div>
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
          Salvar nome
        </AppPanelButton>
        <input
          v-model="rotateDrafts[item.id]"
          type="password"
          autocomplete="off"
          class="calendar-config__input"
          placeholder="Nova chave para rotacionar"
          :disabled="disabled || busy"
        />
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
      </article>
    </template>
  </div>
</template>

<style scoped>
.cfg-credentials {
  display: grid;
  gap: 0.8rem;
}
.cfg-credentials__intro,
.cfg-credentials__item {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  justify-content: space-between;
}
.cfg-credentials__intro h3,
.cfg-credentials__intro p,
.cfg-credentials__item p {
  margin: 0;
}
.cfg-credentials__identity {
  display: grid;
  gap: 0.25rem;
  min-width: 12rem;
}
.cfg-credentials__form {
  display: grid;
  grid-template-columns: 1fr 0.8fr 1.5fr auto;
  align-items: end;
  gap: 0.7rem;
}
.cfg-credentials__item {
  padding: 0.75rem;
  border: 1px solid rgb(var(--border) / 0.65);
  border-radius: var(--radius-md);
}
.cfg-credentials__item .calendar-config__input {
  max-width: 20rem;
}
@media (max-width: 800px) {
  .cfg-credentials__intro,
  .cfg-credentials__item {
    align-items: stretch;
    flex-direction: column;
  }
  .cfg-credentials__form {
    grid-template-columns: 1fr;
  }
  .cfg-credentials__item .calendar-config__input {
    max-width: none;
  }
}
</style>
