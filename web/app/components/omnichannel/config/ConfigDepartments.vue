<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  createDepartment,
  fetchDepartments,
  updateDepartment,
} from '~/domain/omnichannel/config-api'
import type { OmniDepartment } from '~/domain/omnichannel/config-types'

// Aba de setores (perm omnichannel.settings.manage). Remover = soft (isActive=false): a
// conversa que já está na fila continua visível. Accordion: "Adicionar setor" + um por setor.
defineProps<{ canManage: boolean }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const departments = ref<OmniDepartment[]>([])
const loading = ref(true)
const busy = ref(false)
const newName = ref('')
const drafts = reactive<Record<string, { name: string; isDefault: boolean; isActive: boolean }>>({})

async function load(): Promise<void> {
  loading.value = true
  try {
    departments.value = await fetchDepartments(api)
    for (const d of departments.value) {
      drafts[d.id] = { name: d.name, isDefault: d.isDefault, isActive: d.isActive }
    }
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar os setores.'))
  } finally {
    loading.value = false
  }
}

async function create(): Promise<void> {
  const name = newName.value.trim()
  if (!name || busy.value) return
  busy.value = true
  try {
    await createDepartment(api, { name })
    newName.value = ''
    ui.success('Setor criado.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível criar o setor.'))
  } finally {
    busy.value = false
  }
}

async function save(id: string): Promise<void> {
  const d = drafts[id]
  if (!d || busy.value) return
  if (!d.name.trim()) {
    ui.error('O nome do setor não pode ficar vazio.')
    return
  }
  busy.value = true
  try {
    await updateDepartment(api, id, {
      name: d.name.trim(),
      isDefault: d.isDefault,
      isActive: d.isActive,
    })
    ui.success('Setor atualizado.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar o setor.'))
  } finally {
    busy.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="cfg-tab">
    <p class="cfg-tab__lead">Setores agrupam as filas de atendimento da conta.</p>
    <p v-if="loading" class="cfg-tab__loading">Carregando setores…</p>

    <template v-else>
      <details class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Adicionar setor</strong>
            <span class="settings-collapse__text">Cria um novo setor na conta.</span>
          </div>
          <span class="settings-collapse__meta">novo</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body">
          <label class="cfg-field">
            <span class="cfg-field__label">Nome do setor *</span>
            <input v-model="newName" class="cfg-input" type="text" :disabled="!canManage" />
          </label>
          <div class="cfg-tab__form-foot">
            <span v-if="!newName.trim()" class="cfg-tab__hint">Informe o nome do setor.</span>
            <AppPanelButton
              variant="primary"
              :disabled="!canManage || busy || !newName.trim()"
              @click="create"
            >
              Criar setor
            </AppPanelButton>
          </div>
        </div>
      </details>

      <p v-if="departments.length === 0" class="cfg-empty">Nenhum setor cadastrado ainda.</p>

      <details v-for="d in departments" :key="d.id" class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">{{ d.name }}</strong>
            <span class="settings-collapse__text">{{ d.slug }}</span>
          </div>
          <span class="settings-collapse__meta">
            {{ d.isActive ? 'ativo' : 'desativado' }}{{ d.isDefault ? ' · padrão' : '' }}
          </span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div v-if="drafts[d.id]" class="settings-collapse__body">
          <label class="cfg-field">
            <span class="cfg-field__label">Nome *</span>
            <input
              v-model="drafts[d.id].name"
              class="cfg-input"
              type="text"
              :disabled="!canManage"
            />
          </label>
          <div class="cfg-toggles">
            <AppToggleSwitch
              v-model="drafts[d.id].isDefault"
              :disabled="!canManage"
              label="Setor padrão"
            />
            <AppToggleSwitch v-model="drafts[d.id].isActive" :disabled="!canManage" label="Ativo" />
          </div>
          <p class="cfg-tab__hint">Desativar mantém as conversas já recebidas visíveis no inbox.</p>
          <div class="cfg-tab__form-foot">
            <AppPanelButton variant="primary" :disabled="!canManage || busy" @click="save(d.id)">
              Salvar setor
            </AppPanelButton>
          </div>
        </div>
      </details>
    </template>
  </div>
</template>

<style scoped>
.cfg-tab {
  display: grid;
  gap: 0.75rem;
}

.cfg-tab__lead,
.cfg-tab__loading,
.cfg-empty {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.82rem;
}

.cfg-field {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.cfg-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: rgb(var(--muted));
}

.cfg-input {
  min-height: 36px;
  padding: 0 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.cfg-input:focus {
  outline: none;
  border-color: rgb(var(--primary) / 0.6);
}

.cfg-input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.cfg-toggles {
  display: flex;
  flex-wrap: wrap;
  gap: 1.25rem;
  margin-top: 0.5rem;
}

.cfg-tab__form-foot {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 0.5rem;
}

.cfg-tab__hint {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.76rem;
}
</style>
