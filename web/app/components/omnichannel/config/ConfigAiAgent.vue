<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import ConfigAiAgentCard from '~/components/omnichannel/config/ConfigAiAgentCard.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { createAgent, fetchAgents } from '~/domain/omnichannel/config-api'
import type { OmniAgent } from '~/domain/omnichannel/config-types'

// Aba do agente de IA (perm omnichannel.agents.manage). Editor + publish/rollback + simulador.
defineProps<{ canManage: boolean }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const agents = ref<OmniAgent[]>([])
const loading = ref(true)
const busy = ref(false)
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

async function create(): Promise<void> {
  const name = newName.value.trim()
  if (!name || busy.value) return
  busy.value = true
  try {
    await createAgent(api, { name, enabled: false })
    newName.value = ''
    ui.success('Agente criado (desabilitado). Crie e publique uma versão para ativar.')
    await load()
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
      O agente sugere; o motor de roteamento decide. Edite versões e publique quando validar no
      simulador. Só a versão publicada vale em runtime.
    </p>
    <p v-if="loading" class="cfg-tab__loading">Carregando agentes…</p>

    <template v-else>
      <details class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Adicionar agente</strong>
            <span class="settings-collapse__text">
              Novo agente de triagem (nasce desabilitado).
            </span>
          </div>
          <span class="settings-collapse__meta">novo</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body">
          <label class="cfg-field">
            <span class="cfg-field__label">Nome do agente *</span>
            <input v-model="newName" class="cfg-input" type="text" :disabled="!canManage" />
          </label>
          <div class="cfg-tab__form-foot">
            <span v-if="!newName.trim()" class="cfg-tab__hint">Informe o nome do agente.</span>
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

      <p v-if="agents.length === 0" class="cfg-empty">Nenhum agente cadastrado ainda.</p>

      <details v-for="agent in agents" :key="agent.id" class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">{{ agent.name }}</strong>
            <span class="settings-collapse__text">{{ agent.slug }}</span>
          </div>
          <span class="settings-collapse__meta">
            {{ agent.enabled ? 'habilitado' : 'desabilitado' }}
          </span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body">
          <ConfigAiAgentCard :agent="agent" :disabled="!canManage" @changed="load" />
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
}
</style>
