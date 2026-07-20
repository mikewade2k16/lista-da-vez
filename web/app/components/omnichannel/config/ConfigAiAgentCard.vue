<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import ConfigAiAgentVersions from '~/components/omnichannel/config/ConfigAiAgentVersions.vue'
import ConfigAiAgentSimulator from '~/components/omnichannel/config/ConfigAiAgentSimulator.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  createAgentVersion,
  fetchAgent,
  fetchAgentVersions,
  publishAgentVersion,
  rollbackAgent,
  updateAgent,
} from '~/domain/omnichannel/config-api'
import type {
  OmniAgent,
  OmniAgentVersion,
  OmniAgentVersionInput,
} from '~/domain/omnichannel/config-types'

// Editor de UM agente: identidade (nome/ativo), chave do provider (write-only {set,last4}),
// versões (publish/rollback) e simulador. A chave crua nunca volta do back nem entra em log.
const props = defineProps<{ agent: OmniAgent; disabled?: boolean }>()
const emit = defineEmits<{ changed: [] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const local = ref<OmniAgent>(props.agent)
const versions = ref<OmniAgentVersion[]>([])
const draft = reactive({ name: '', enabled: true })
const keyDraft = ref('')
const saving = ref(false)

function hydrate(): void {
  local.value = props.agent
  draft.name = props.agent.name
  draft.enabled = props.agent.enabled
  keyDraft.value = ''
}
watch(() => props.agent, hydrate, { immediate: true })

async function reload(): Promise<void> {
  try {
    local.value = await fetchAgent(api, props.agent.id)
    draft.name = local.value.name
    draft.enabled = local.value.enabled
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível recarregar o agente.'))
  }
}

async function loadVersions(): Promise<void> {
  try {
    versions.value = await fetchAgentVersions(api, props.agent.id)
  } catch {
    versions.value = []
  }
}

async function saveSettings(): Promise<void> {
  if (!draft.name.trim()) {
    ui.error('O nome do agente não pode ficar vazio.')
    return
  }
  saving.value = true
  try {
    await updateAgent(api, props.agent.id, { name: draft.name.trim(), enabled: draft.enabled })
    ui.success('Agente atualizado.')
    await reload()
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar o agente.'))
  } finally {
    saving.value = false
  }
}

async function saveKey(): Promise<void> {
  const value = keyDraft.value.trim()
  if (!value) return
  saving.value = true
  try {
    local.value = await updateAgent(api, props.agent.id, { providerKey: value })
    keyDraft.value = ''
    ui.success('Chave do provider salva.')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar a chave.'))
  } finally {
    saving.value = false
  }
}

async function onCreateVersion(payload: OmniAgentVersionInput): Promise<void> {
  try {
    await createAgentVersion(api, props.agent.id, payload)
    ui.success('Rascunho de versão criado.')
    await loadVersions()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível criar a versão.'))
  }
}

async function onPublish(version: number): Promise<void> {
  const { confirmed } = await ui.confirm({
    title: `Publicar v${version}?`,
    message: 'A versão passa a valer no atendimento em runtime e fica imutável.',
    confirmLabel: 'Publicar',
    cancelLabel: 'Cancelar',
  })
  if (!confirmed) return
  try {
    local.value = await publishAgentVersion(api, props.agent.id, version)
    ui.success(`v${version} publicada.`)
    await loadVersions()
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível publicar a versão.'))
  }
}

async function onRollback(versionId: string): Promise<void> {
  try {
    local.value = await rollbackAgent(api, props.agent.id, versionId)
    ui.success('Versão ativa revertida.')
    await loadVersions()
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível reverter.'))
  }
}

onMounted(() => void loadVersions())
</script>

<template>
  <div class="cfg-agent">
    <div class="cfg-grid">
      <label class="cfg-field">
        <span class="cfg-field__label">Nome do agente *</span>
        <input v-model="draft.name" class="cfg-input" type="text" :disabled="disabled" />
      </label>
      <div class="cfg-field cfg-field--toggle">
        <span class="cfg-field__label">Estado</span>
        <AppToggleSwitch v-model="draft.enabled" :disabled="disabled" label="Habilitado" />
      </div>
    </div>
    <div class="cfg-agent__foot">
      <AppPanelButton variant="primary" :disabled="disabled || saving" @click="saveSettings">
        Salvar agente
      </AppPanelButton>
    </div>

    <section class="cfg-block">
      <div class="cfg-cred__head">
        <span class="cfg-field__label">Chave do provider (LLM)</span>
        <span class="cfg-cred__status" :class="local.providerKey.set ? 'is-set' : 'is-unset'">
          <template v-if="local.providerKey.set">
            configurada
            <template v-if="local.providerKey.last4">
              &bull;&bull;&bull;&bull;{{ local.providerKey.last4 }}
            </template>
          </template>
          <template v-else>não configurada</template>
        </span>
      </div>
      <p class="cfg-hint">
        A chave fica cifrada no servidor — nunca no navegador. Aqui só o status mascarado.
      </p>
      <div class="cfg-cred__row">
        <input
          v-model="keyDraft"
          class="cfg-input"
          type="password"
          autocomplete="off"
          :placeholder="
            local.providerKey.set ? 'Digite para trocar a chave' : 'Cole a chave da API'
          "
          :disabled="disabled || saving"
        />
        <AppPanelButton
          variant="ghost"
          :disabled="disabled || saving || !keyDraft.trim()"
          @click="saveKey"
        >
          Salvar chave
        </AppPanelButton>
      </div>
    </section>

    <section class="cfg-block">
      <ConfigAiAgentVersions
        :versions="versions"
        :active-version-id="local.activeVersionId"
        :disabled="disabled"
        @create="onCreateVersion"
        @publish="onPublish"
        @rollback="onRollback"
      />
    </section>

    <section class="cfg-block">
      <span class="cfg-field__label">Simulador</span>
      <ConfigAiAgentSimulator :agent-id="agent.id" :versions="versions" :disabled="disabled" />
    </section>
  </div>
</template>

<style scoped>
.cfg-agent {
  display: grid;
  gap: 0.85rem;
}

.cfg-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.75rem;
  align-items: end;
}

.cfg-field {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.cfg-field--toggle {
  gap: 0.5rem;
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

.cfg-agent__foot {
  display: flex;
  justify-content: flex-end;
}

.cfg-block {
  display: grid;
  gap: 0.5rem;
  padding-top: 0.7rem;
  border-top: 1px solid rgb(var(--border) / 0.6);
}

.cfg-cred__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.cfg-cred__status {
  font-size: 0.74rem;
  font-weight: 700;
}

.cfg-cred__status.is-set {
  color: rgb(var(--success));
}

.cfg-cred__status.is-unset {
  color: rgb(var(--muted));
}

.cfg-hint {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.cfg-cred__row {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}
</style>
