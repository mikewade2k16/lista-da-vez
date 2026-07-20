<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import ConfigQueueMembers from '~/components/omnichannel/config/ConfigQueueMembers.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  createQueue,
  fetchDepartments,
  fetchInstances,
  fetchQueues,
  updateQueue,
} from '~/domain/omnichannel/config-api'
import type {
  OmniAssignableUser,
  OmniDepartment,
  OmniQueue,
} from '~/domain/omnichannel/config-types'

// Aba de filas + membros (perm omnichannel.settings.manage). Cada fila pertence a um setor.
defineProps<{ canManage: boolean }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const departments = ref<OmniDepartment[]>([])
const queues = ref<OmniQueue[]>([])
const users = ref<OmniAssignableUser[]>([])
const loading = ref(true)
const busy = ref(false)

const form = reactive({ departmentId: '', name: '' })
const drafts = reactive<Record<string, { name: string; isDefault: boolean; isActive: boolean }>>({})

const departmentOptions = computed(() =>
  departments.value.map((d) => ({
    value: d.id,
    label: d.name,
    meta: d.isActive ? '' : 'desativado',
  })),
)

const createBlockedReason = computed(() => {
  if (!form.departmentId) return 'Escolha o setor da fila.'
  if (!form.name.trim()) return 'Informe o nome da fila.'
  return ''
})

function departmentName(id: string): string {
  return departments.value.find((d) => d.id === id)?.name || '—'
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const [deps, qs] = await Promise.all([fetchDepartments(api), fetchQueues(api)])
    departments.value = deps
    queues.value = qs
    for (const q of qs)
      drafts[q.id] = { name: q.name, isDefault: q.isDefault, isActive: q.isActive }
    // Pool de atendentes elegíveis para membros (users da gestão de instâncias). Tolerante
    // a falha: sem pool, o gerenciador de membros mostra estado vazio orientativo.
    try {
      users.value = (await fetchInstances(api)).users || []
    } catch {
      users.value = []
    }
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar as filas.'))
  } finally {
    loading.value = false
  }
}

async function create(): Promise<void> {
  if (createBlockedReason.value || busy.value) return
  busy.value = true
  try {
    await createQueue(api, { departmentId: form.departmentId, name: form.name.trim() })
    form.name = ''
    ui.success('Fila criada.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível criar a fila.'))
  } finally {
    busy.value = false
  }
}

async function save(id: string): Promise<void> {
  const d = drafts[id]
  if (!d || busy.value) return
  if (!d.name.trim()) {
    ui.error('O nome da fila não pode ficar vazio.')
    return
  }
  busy.value = true
  try {
    await updateQueue(api, id, {
      name: d.name.trim(),
      isDefault: d.isDefault,
      isActive: d.isActive,
    })
    ui.success('Fila atualizada.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar a fila.'))
  } finally {
    busy.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="cfg-tab">
    <p class="cfg-tab__lead">Filas recebem as conversas roteadas e têm atendentes (membros).</p>
    <p v-if="loading" class="cfg-tab__loading">Carregando filas…</p>

    <template v-else>
      <p v-if="departments.length === 0" class="cfg-empty">
        Cadastre um setor antes de criar filas (aba Setores).
      </p>

      <details v-else class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Adicionar fila</strong>
            <span class="settings-collapse__text">Nova fila num setor existente.</span>
          </div>
          <span class="settings-collapse__meta">novo</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body">
          <div class="cfg-grid">
            <AppSelectField
              class="cfg-field"
              label="Setor *"
              :model-value="form.departmentId"
              :options="departmentOptions"
              :disabled="!canManage"
              @update:model-value="form.departmentId = $event"
            />
            <label class="cfg-field">
              <span class="cfg-field__label">Nome da fila *</span>
              <input v-model="form.name" class="cfg-input" type="text" :disabled="!canManage" />
            </label>
          </div>
          <div class="cfg-tab__form-foot">
            <span v-if="createBlockedReason" class="cfg-tab__hint">{{ createBlockedReason }}</span>
            <AppPanelButton
              variant="primary"
              :disabled="!canManage || busy || !!createBlockedReason"
              @click="create"
            >
              Criar fila
            </AppPanelButton>
          </div>
        </div>
      </details>

      <p v-if="queues.length === 0 && departments.length > 0" class="cfg-empty">
        Nenhuma fila cadastrada ainda.
      </p>

      <details v-for="q in queues" :key="q.id" class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">{{ q.name }}</strong>
            <span class="settings-collapse__text">Setor: {{ departmentName(q.departmentId) }}</span>
          </div>
          <span class="settings-collapse__meta">
            {{ q.isActive ? 'ativa' : 'desativada' }}{{ q.isDefault ? ' · padrão' : '' }}
          </span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div v-if="drafts[q.id]" class="settings-collapse__body">
          <label class="cfg-field">
            <span class="cfg-field__label">Nome *</span>
            <input
              v-model="drafts[q.id].name"
              class="cfg-input"
              type="text"
              :disabled="!canManage"
            />
          </label>
          <div class="cfg-toggles">
            <AppToggleSwitch
              v-model="drafts[q.id].isDefault"
              :disabled="!canManage"
              label="Fila padrão"
            />
            <AppToggleSwitch v-model="drafts[q.id].isActive" :disabled="!canManage" label="Ativa" />
          </div>
          <div class="cfg-tab__form-foot">
            <AppPanelButton variant="primary" :disabled="!canManage || busy" @click="save(q.id)">
              Salvar fila
            </AppPanelButton>
          </div>
          <div class="cfg-tab__members">
            <ConfigQueueMembers :queue-id="q.id" :users="users" :disabled="!canManage" />
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

.cfg-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.75rem;
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
  color: rgb(var(--muted));
  font-size: 0.76rem;
  text-align: right;
}

.cfg-tab__members {
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid rgb(var(--border) / 0.6);
}
</style>
