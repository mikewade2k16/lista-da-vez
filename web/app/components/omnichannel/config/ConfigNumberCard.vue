<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import ConfigNumberAccess from '~/components/omnichannel/config/ConfigNumberAccess.vue'
import ConfigNumberCredentials from '~/components/omnichannel/config/ConfigNumberCredentials.vue'
import ConfigNumberCapabilities from '~/components/omnichannel/config/ConfigNumberCapabilities.vue'
import ConfigNumberConnection from '~/components/omnichannel/config/ConfigNumberConnection.vue'
import {
  canResetInstanceHistory,
  useOmnichannelScopeInvalidation,
} from '~/composables/omnichannel/useOmnichannelScopeInvalidation'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { updateInstance } from '~/domain/omnichannel/config-api'
import { deleteInstance } from '~/domain/omnichannel/instance-admin-api'
import { OMNI_PROVIDER_LABEL } from '~/domain/omnichannel/config-types'
import type {
  OmniAssignableUser,
  OmniInstance,
  OmniProvider,
} from '~/domain/omnichannel/config-types'

// Editor de um número. O pareamento é a ação principal; edição, acesso e integração
// ficam recolhidos para não repetir os dados já presentes no resumo do card.
const props = defineProps<{
  instance: OmniInstance
  users: OmniAssignableUser[]
  reloadInstances: () => Promise<void>
  disabled?: boolean
}>()
const emit = defineEmits<{ changed: [] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)
const { accountLabel, isResettingInstance, requestInstanceHistoryReset } =
  useOmnichannelScopeInvalidation()

const draft = reactive({
  displayName: '',
  phoneNumber: '',
  queueLabel: '',
  isDefault: false,
})
const saving = ref(false)
const resolvedProvider = ref(props.instance.provider || '')

function hydrate(): void {
  draft.displayName = props.instance.displayName || ''
  draft.phoneNumber = props.instance.phoneNumber || ''
  draft.queueLabel = props.instance.queueLabel || ''
  draft.isDefault = props.instance.isDefault
  resolvedProvider.value = props.instance.provider || ''
}

watch(() => props.instance, hydrate, { immediate: true })

const providerLabel = computed(() => {
  const provider = (resolvedProvider.value || '') as OmniProvider
  return OMNI_PROVIDER_LABEL[provider] || resolvedProvider.value || 'provider não resolvido'
})

const credentialLabel = computed(() =>
  props.instance.hasEvolutionApiKey ? 'credencial configurada' : 'sem credencial própria',
)

const canResetHistory = computed(() => canResetInstanceHistory(props.instance))
const historyResetting = computed(() => isResettingInstance(props.instance.id))
const historyRiskTitleId = computed(() => `history-risk-title-${props.instance.id}`)

async function save(): Promise<void> {
  saving.value = true
  try {
    await updateInstance(api, props.instance.id, {
      displayName: draft.displayName.trim(),
      phoneNumber: draft.phoneNumber.trim(),
      queueLabel: draft.queueLabel.trim(),
      userScopePolicy: props.instance.userScopePolicy,
      isDefault: draft.isDefault,
    })
    ui.success('Configurações do número salvas.')
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar o número.'))
  } finally {
    saving.value = false
  }
}

async function remove(): Promise<void> {
  const confirmation = (await ui.confirm({
    title: 'Excluir número?',
    message:
      'O cadastro será removido. Se houver conversas vinculadas, a exclusão será bloqueada para preservar o histórico.',
    confirmLabel: 'Excluir',
    cancelLabel: 'Cancelar',
    danger: true,
  })) as { confirmed?: boolean }
  if (!confirmation.confirmed) return

  saving.value = true
  try {
    await deleteInstance(api, props.instance.id)
    ui.success('Cadastro do número excluído.')
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível excluir o número.'))
  } finally {
    saving.value = false
  }
}

async function resetVisibleHistory(): Promise<void> {
  await requestInstanceHistoryReset(props.instance, {
    rehydrate: props.reloadInstances,
  })
}
</script>

<template>
  <div class="cfg-card">
    <section class="cfg-card__connection">
      <div class="cfg-card__section-head">
        <div>
          <strong>Conexão do WhatsApp</strong>
          <span>Use o celular para parear ou conferir a sessão.</span>
        </div>
      </div>
      <ConfigNumberConnection
        :instance-name="instance.instanceName"
        :disabled="disabled"
        @provider-resolved="resolvedProvider = $event"
      />
    </section>

    <details class="cfg-card__details">
      <summary>
        <span>
          <strong>Configurações do número</strong>
          <small>{{ providerLabel }} · {{ credentialLabel }}</small>
        </span>
        <span class="material-icons-round" aria-hidden="true">expand_more</span>
      </summary>

      <div class="cfg-card__details-body">
        <div class="cfg-grid">
          <label class="cfg-field">
            <span class="cfg-field__label">Nome de exibição</span>
            <input v-model="draft.displayName" class="cfg-input" type="text" :disabled="disabled" />
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Telefone</span>
            <input
              v-model="draft.phoneNumber"
              class="cfg-input"
              type="text"
              placeholder="Somente dígitos com DDD"
              :disabled="disabled"
            />
          </label>
          <label class="cfg-field">
            <span class="cfg-field__label">Rótulo da fila</span>
            <input v-model="draft.queueLabel" class="cfg-input" type="text" :disabled="disabled" />
          </label>
        </div>

        <AppToggleSwitch
          v-model="draft.isDefault"
          :disabled="disabled"
          label="Usar como número padrão"
        />

        <ConfigNumberAccess
          :instance-id="instance.id"
          :users="users"
          :reload-instances="reloadInstances"
          :disabled="disabled"
          @changed="emit('changed')"
        />

        <details class="cfg-card__subdetails">
          <summary>
            <span>Credencial e capacidades</span>
            <small>detalhes técnicos</small>
          </summary>
          <div class="cfg-card__subdetails-body cfg-card__technical">
            <ConfigNumberCredentials
              :instance-name="instance.instanceName"
              :initial-set="instance.hasEvolutionApiKey"
              :disabled="disabled"
              @saved="emit('changed')"
            />
            <div class="cfg-card__capabilities">
              <span class="cfg-field__label">Capacidades confirmadas</span>
              <ConfigNumberCapabilities :instance-id="instance.id" />
            </div>
          </div>
        </details>

        <section
          v-if="canResetHistory"
          class="cfg-card__risk"
          :aria-labelledby="historyRiskTitleId"
        >
          <div>
            <strong :id="historyRiskTitleId">Zona de risco</strong>
            <p>
              Oculta o histórico anterior somente desta conexão. A sessão não será desconectada e os
              contatos serão preservados.
            </p>
          </div>
          <dl class="cfg-card__risk-details">
            <div>
              <dt>Conta</dt>
              <dd>{{ accountLabel }}</dd>
            </div>
            <div>
              <dt>instanceName</dt>
              <dd>{{ instance.instanceName }}</dd>
            </div>
            <div>
              <dt>Nome de exibição</dt>
              <dd>{{ instance.displayName || 'não informado' }}</dd>
            </div>
            <div>
              <dt>Telefone</dt>
              <dd>{{ instance.phoneNumber || 'não informado' }}</dd>
            </div>
            <div>
              <dt>Provider</dt>
              <dd>{{ instance.provider }}</dd>
            </div>
          </dl>
          <AppPanelButton
            variant="danger"
            :disabled="historyResetting"
            @click="resetVisibleHistory"
          >
            {{
              historyResetting ? 'Limpando histórico…' : 'Limpar histórico visível desta conexão'
            }}
          </AppPanelButton>
        </section>

        <div class="cfg-card__actions">
          <AppPanelButton variant="danger" :disabled="disabled || saving" @click="remove">
            Excluir número
          </AppPanelButton>
          <AppPanelButton variant="primary" :disabled="disabled || saving" @click="save">
            {{ saving ? 'Salvando…' : 'Salvar alterações' }}
          </AppPanelButton>
        </div>
      </div>
    </details>
  </div>
</template>

<style scoped>
.cfg-card {
  display: grid;
  gap: 0.75rem;
}

.cfg-card__connection,
.cfg-card__details-body {
  display: grid;
  gap: 0.75rem;
}

.cfg-card__section-head > div {
  display: grid;
  gap: 0.12rem;
}

.cfg-card__section-head strong {
  color: rgb(var(--text));
  font-size: 0.84rem;
}

.cfg-card__section-head span {
  color: rgb(var(--muted));
  font-size: 0.74rem;
}

.cfg-card__details,
.cfg-card__subdetails {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}

.cfg-card__details > summary,
.cfg-card__subdetails > summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.7rem 0.75rem;
  color: rgb(var(--text));
  cursor: pointer;
}

.cfg-card__details > summary > span:first-child {
  display: grid;
  gap: 0.12rem;
}

.cfg-card__details summary small,
.cfg-card__subdetails summary small {
  color: rgb(var(--muted));
  font-size: 0.7rem;
  font-weight: 500;
}

.cfg-card__details-body {
  padding: 0 0.75rem 0.75rem;
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
  color: rgb(var(--muted));
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
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

.cfg-card__subdetails-body {
  padding: 0 0.75rem 0.75rem;
}

.cfg-card__technical {
  display: grid;
  gap: 0.75rem;
}

.cfg-card__capabilities {
  display: grid;
  gap: 0.5rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--line-soft);
}

.cfg-card__actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}

.cfg-card__risk {
  display: grid;
  gap: 0.7rem;
  padding: 0.75rem;
  border: 1px solid rgb(var(--danger) / 0.45);
  border-radius: var(--radius-sm);
  background: rgb(var(--danger) / 0.06);
}

.cfg-card__risk strong,
.cfg-card__risk dd {
  color: rgb(var(--text));
}

.cfg-card__risk p,
.cfg-card__risk dl {
  margin: 0.2rem 0 0;
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.cfg-card__risk-details {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 0.5rem;
}

.cfg-card__risk-details div {
  min-width: 0;
}

.cfg-card__risk dt {
  font-weight: 700;
}

.cfg-card__risk dd {
  margin: 0.1rem 0 0;
  overflow-wrap: anywhere;
}

@media (max-width: 640px) {
  .cfg-card__actions {
    justify-content: stretch;
  }

  .cfg-card__actions > * {
    flex: 1;
  }
}
</style>
