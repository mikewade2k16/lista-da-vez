<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import ConfigNumberCard from '~/components/omnichannel/config/ConfigNumberCard.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { createInstance, fetchInstances, updateInstance } from '~/domain/omnichannel/config-api'
import { updateChannelLimit } from '~/domain/omnichannel/instance-admin-api'
import { OMNI_PROVIDER_LABEL, OMNI_PROVIDER_OPTIONS } from '~/domain/omnichannel/config-types'
import type { OmniAssignableUser, OmniInstance } from '~/domain/omnichannel/config-types'

const props = defineProps<{ canManage: boolean }>()
const emit = defineEmits<{ changed: [] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const instances = ref<OmniInstance[]>([])
const users = ref<OmniAssignableUser[]>([])
const maxChannels = ref(0)
const currentChannels = ref(0)
const limitDraft = ref(1)
const loading = ref(true)
const creating = ref(false)
const savingLimit = ref(false)
const togglingId = ref('')
const openInstanceId = ref('')

const form = reactive({
  instanceName: '',
  displayName: '',
  phoneNumber: '',
  provider: 'evolution',
})

const isPlatformAdmin = computed(() => auth.role === 'platform_admin')
const atLimit = computed(() => maxChannels.value > 0 && currentChannels.value >= maxChannels.value)
const createBlockedReason = computed(() => {
  if (!form.instanceName.trim()) return 'Informe um identificador único.'
  if (atLimit.value)
    return `Limite de ${maxChannels.value} número(s) ativos atingido. Desative ou exclua um número, ou ajuste o limite.`
  return ''
})
const canCreate = computed(() => props.canManage && !creating.value && !createBlockedReason.value)

async function load(): Promise<void> {
  loading.value = true
  try {
    const view = await fetchInstances(api)
    instances.value = view.instances || []
    users.value = view.users || []
    maxChannels.value = view.maxChannels || 0
    currentChannels.value = view.currentChannels || 0
    limitDraft.value = Math.max(1, view.maxChannels || 1)
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar os números.'))
  } finally {
    loading.value = false
  }
}

async function submitCreate(): Promise<void> {
  if (!canCreate.value) return
  creating.value = true
  try {
    const created = await createInstance(api, {
      instanceName: form.instanceName.trim(),
      displayName: form.displayName.trim(),
      phoneNumber: form.phoneNumber.trim(),
      provider: form.provider,
      isActive: true,
    })
    form.instanceName = ''
    form.displayName = ''
    form.phoneNumber = ''
    openInstanceId.value = created.id
    ui.success('Número cadastrado. O painel de conexão foi aberto para gerar o QR.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível cadastrar o número.'))
  } finally {
    creating.value = false
  }
}

function fullInstancePatch(instance: OmniInstance, isActive: boolean) {
  return {
    instanceName: instance.instanceName,
    displayName: instance.displayName || '',
    phoneNumber: instance.phoneNumber || '',
    queueLabel: instance.queueLabel || '',
    responsibleUserId: instance.responsibleUserId || '',
    userScopePolicy: instance.userScopePolicy,
    isDefault: instance.isDefault,
    isActive,
  }
}

async function toggleInstance(instance: OmniInstance, isActive: boolean): Promise<void> {
  if (!props.canManage || togglingId.value) return
  if (!isActive) {
    const confirmation = (await ui.confirm({
      title: 'Desativar número?',
      message: 'Ele deixa de ocupar o limite da conta. As conversas existentes permanecem salvas.',
      confirmLabel: 'Desativar',
      cancelLabel: 'Cancelar',
    })) as { confirmed?: boolean }
    if (!confirmation.confirmed) return
  }

  togglingId.value = instance.id
  try {
    await updateInstance(api, instance.id, fullInstancePatch(instance, isActive))
    ui.success(isActive ? 'Número ativado.' : 'Número desativado e limite liberado.')
    await load()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível alterar o estado do número.'))
  } finally {
    togglingId.value = ''
  }
}

async function saveLimit(): Promise<void> {
  const value = Math.trunc(Number(limitDraft.value))
  if (!isPlatformAdmin.value || value < 1 || value > 100 || savingLimit.value) return
  savingLimit.value = true
  try {
    const result = await updateChannelLimit(api, value)
    maxChannels.value = result.maxChannels
    currentChannels.value = result.currentChannels
    ui.success('Limite de números atualizado para a conta ativa.')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível atualizar o limite de números.'))
  } finally {
    savingLimit.value = false
  }
}

function subtitleOf(instance: OmniInstance): string {
  const parts = [OMNI_PROVIDER_LABEL[instance.provider] || instance.provider]
  if (instance.phoneNumber) parts.push(instance.phoneNumber)
  if (instance.isDefault) parts.push('padrão')
  return parts.join(' · ')
}

function onToggleCard(instanceId: string, event: Event): void {
  const details = event.currentTarget as HTMLDetailsElement
  if (details.open) openInstanceId.value = instanceId
  else if (openInstanceId.value === instanceId) openInstanceId.value = ''
}

onMounted(() => void load())
</script>

<template>
  <div class="cfg-tab">
    <div class="cfg-tab__lead">
      <span>Conecte o WhatsApp usado no primeiro atendimento.</span>
      <strong>{{ currentChannels }}/{{ maxChannels || '∞' }} ativos</strong>
    </div>

    <details v-if="isPlatformAdmin" class="calendar-config__collapse">
      <summary class="calendar-config__collapse-head">Limite de números da conta</summary>
      <div class="calendar-config__collapse-body cfg-limit">
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Máximo de números ativos</span>
          <input
            v-model.number="limitDraft"
            class="calendar-config__input"
            type="number"
            min="1"
            max="100"
            :disabled="savingLimit"
          />
          <span class="calendar-config__hint">
            Somente o platform admin altera este teto. Números inativos não ocupam limite.
          </span>
        </label>
        <AppPanelButton
          variant="secondary"
          :disabled="
            savingLimit || limitDraft < currentChannels || limitDraft < 1 || limitDraft > 100
          "
          @click="saveLimit"
        >
          Salvar limite
        </AppPanelButton>
      </div>
    </details>

    <p v-if="loading" class="cfg-tab__loading">Carregando números…</p>

    <template v-else>
      <details class="settings-collapse" :open="instances.length === 0">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Adicionar número</strong>
            <span class="settings-collapse__text">Cadastre uma conexão Evolution e gere o QR.</span>
          </div>
          <span class="settings-collapse__meta">novo</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body cfg-create">
          <div class="calendar-config__grid2">
            <label class="calendar-config__field">
              <span class="calendar-config__field-label">Identificador único *</span>
              <input
                v-model="form.instanceName"
                class="calendar-config__input"
                type="text"
                placeholder="Ex.: whatsapp-comercial"
                :disabled="!canManage"
              />
            </label>
            <AppSelectField
              label="Provider *"
              :model-value="form.provider"
              :options="OMNI_PROVIDER_OPTIONS"
              :disabled="!canManage"
              @update:model-value="form.provider = $event"
            />
          </div>

          <details class="cfg-optional">
            <summary>Identificação opcional</summary>
            <div class="calendar-config__grid2 cfg-optional__body">
              <label class="calendar-config__field">
                <span class="calendar-config__field-label">Nome de exibição</span>
                <input
                  v-model="form.displayName"
                  class="calendar-config__input"
                  type="text"
                  :disabled="!canManage"
                />
              </label>
              <label class="calendar-config__field">
                <span class="calendar-config__field-label">Telefone</span>
                <input
                  v-model="form.phoneNumber"
                  class="calendar-config__input"
                  type="text"
                  placeholder="Somente dígitos com DDD"
                  :disabled="!canManage"
                />
              </label>
            </div>
          </details>

          <div class="cfg-tab__form-foot">
            <span v-if="createBlockedReason" class="cfg-tab__hint">{{ createBlockedReason }}</span>
            <AppPanelButton variant="primary" :disabled="!canCreate" @click="submitCreate">
              Cadastrar número
            </AppPanelButton>
          </div>
        </div>
      </details>

      <p v-if="instances.length === 0" class="cfg-empty">
        Nenhum número cadastrado. Adicione um para gerar o QR de conexão.
      </p>

      <details
        v-for="instance in instances"
        :key="instance.id"
        class="settings-collapse"
        :open="openInstanceId === instance.id"
        @toggle="onToggleCard(instance.id, $event)"
      >
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">
              {{ instance.displayName || instance.instanceName }}
            </strong>
            <span class="settings-collapse__text">{{ subtitleOf(instance) }}</span>
          </div>
          <span class="cfg-number-toggle" @click.stop @keydown.stop>
            <AppToggleSwitch
              :model-value="instance.isActive"
              :disabled="!canManage || togglingId === instance.id"
              :label="instance.isActive ? 'Ativo' : 'Inativo'"
              compact
              @update:model-value="toggleInstance(instance, $event)"
            />
          </span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body">
          <ConfigNumberCard
            :instance="instance"
            :users="users"
            :disabled="!canManage"
            @changed="load"
          />
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
.cfg-limit,
.cfg-tab__form-foot,
.cfg-number-toggle {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.cfg-tab__lead,
.cfg-tab__form-foot,
.cfg-limit {
  justify-content: space-between;
}

.cfg-tab__lead,
.cfg-tab__loading,
.cfg-empty,
.cfg-tab__hint {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.82rem;
  line-height: 1.4;
}

.cfg-tab__lead strong {
  flex: none;
  color: rgb(var(--text));
}

.cfg-create {
  display: grid;
  gap: 0.75rem;
}

.cfg-limit .calendar-config__field {
  flex: 1;
}

.cfg-limit .calendar-config__input {
  max-width: 140px;
}

.cfg-optional {
  border-top: 1px solid var(--line-soft);
}

.cfg-optional summary {
  padding-top: 0.65rem;
  color: rgb(var(--muted));
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
}

.cfg-optional__body {
  padding-top: 0.7rem;
}

.cfg-tab__hint {
  text-align: right;
}

@media (max-width: 640px) {
  .cfg-tab__lead,
  .cfg-tab__form-foot,
  .cfg-limit {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
