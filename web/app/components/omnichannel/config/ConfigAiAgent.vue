<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import ConfigAiAgentCard from '~/components/omnichannel/config/ConfigAiAgentCard.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { createAgent, fetchAgents, updateAgent } from '~/domain/omnichannel/config-api'
import type { AutomationProfile } from '~/domain/omnichannel/automation-api'
import type { OmniAgent } from '~/domain/omnichannel/config-types'

const props = defineProps<{ canManage: boolean; profiles?: AutomationProfile[] }>()
const emit = defineEmits<{ changed: [] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const agents = ref<OmniAgent[]>([])
const loading = ref(true)
const busy = ref(false)
const togglingId = ref('')
const newName = ref('')

async function load(): Promise<void> {
  loading.value = true
  try {
    agents.value = await fetchAgents(api)
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar os agentes.'))
  } finally {
    loading.value = false
  }
}

async function toggleAgent(agent: OmniAgent, enabled: boolean): Promise<void> {
  if (!props.canManage || togglingId.value) return
  togglingId.value = agent.id
  try {
    await updateAgent(api, agent.id, { enabled })
    ui.success(enabled ? 'Agente ativado.' : 'Agente desativado.')
    await load()
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível alterar o estado do agente.'))
  } finally {
    togglingId.value = ''
  }
}

async function onCardChanged(): Promise<void> {
  await load()
  emit('changed')
}

async function create(): Promise<void> {
  const name = newName.value.trim()
  if (!name || busy.value) return
  busy.value = true
  try {
    await createAgent(api, { name, enabled: false })
    newName.value = ''
    ui.success('Agente criado. Configure chave, modelo e prompt antes de ativar.')
    await load()
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível criar o agente.'))
  } finally {
    busy.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="cfg-tab">
    <p class="cfg-tab__lead">
      Cada agente mantém seu próprio prompt, modelo, escopo de clientes e chave de API.
    </p>
    <p v-if="loading" class="cfg-tab__loading">Carregando agentes…</p>

    <template v-else>
      <p v-if="agents.length === 0" class="cfg-empty">
        Nenhum agente cadastrado. Crie o primeiro agente abaixo.
      </p>

      <details v-for="agent in agents" :key="agent.id" class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">{{ agent.name }}</strong>
            <span class="settings-collapse__text">
              {{ agent.activeVersionId ? 'versão publicada' : 'sem versão publicada' }}
            </span>
          </div>
          <span class="cfg-agent-toggle" @click.stop @keydown.stop>
            <AppToggleSwitch
              :model-value="agent.enabled"
              :disabled="!canManage || togglingId === agent.id"
              :label="agent.enabled ? 'Ativo' : 'Inativo'"
              compact
              @update:model-value="toggleAgent(agent, $event)"
            />
          </span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body">
          <ConfigAiAgentCard
            :agent="agent"
            :profiles="profiles || []"
            :disabled="!canManage"
            @changed="onCardChanged"
          />
        </div>
      </details>

      <details class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Adicionar agente</strong>
            <span class="settings-collapse__text">Crie outro perfil de atendimento.</span>
          </div>
          <span class="settings-collapse__meta">novo</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body">
          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Nome do agente</span>
            <input
              v-model="newName"
              class="calendar-config__input"
              type="text"
              :disabled="!canManage"
            />
          </label>
          <div class="calendar-config__section-actions">
            <AppPanelButton
              variant="primary"
              :disabled="!canManage || busy || !newName.trim()"
              @click="create"
            >
              Criar agente
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
  line-height: 1.4;
}

.cfg-agent-toggle {
  display: inline-flex;
  align-items: center;
  margin-left: auto;
}
</style>
