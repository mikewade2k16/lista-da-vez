<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import ConfigNumberCard from '~/components/omnichannel/config/ConfigNumberCard.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { createInstance, fetchInstances } from '~/domain/omnichannel/config-api'
import { OMNI_PROVIDER_OPTIONS } from '~/domain/omnichannel/config-types'
import type { OmniAssignableUser, OmniInstance } from '~/domain/omnichannel/config-types'

// Aba de números/instâncias/providers (perm omnichannel.instances.manage). Accordion:
// um bloco "Adicionar número" + um bloco por número, todos fechados por padrão.
const props = defineProps<{ canManage: boolean }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const instances = ref<OmniInstance[]>([])
const users = ref<OmniAssignableUser[]>([])
const maxChannels = ref(0)
const currentChannels = ref(0)
const loading = ref(true)
const creating = ref(false)

const form = reactive({
  instanceName: '',
  displayName: '',
  phoneNumber: '',
  provider: 'evolution',
})

const atLimit = computed(() => maxChannels.value > 0 && currentChannels.value >= maxChannels.value)

// Feedback de formulário: diz o que falta antes de deixar submeter (nunca botão morto e mudo).
const createBlockedReason = computed(() => {
  if (!form.instanceName.trim()) return 'Informe um nome único para a instância.'
  if (!form.provider.trim()) return 'Escolha o provider.'
  if (atLimit.value)
    return `Limite de ${maxChannels.value} número(s) da conta atingido. Desative um número antes de cadastrar outro.`
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
    await createInstance(api, {
      instanceName: form.instanceName.trim(),
      displayName: form.displayName.trim(),
      phoneNumber: form.phoneNumber.trim(),
      provider: form.provider,
    })
    form.instanceName = ''
    form.displayName = ''
    form.phoneNumber = ''
    ui.success('Número cadastrado.')
    await load()
  } catch (error) {
    // 409 acionável (channel_limit / number_in_use / instance_name_conflict) chega com
    // mensagem própria do back; getApiErrorMessage a preserva.
    ui.error(getApiErrorMessage(error, 'Não foi possível cadastrar o número.'))
  } finally {
    creating.value = false
  }
}

function summaryOf(inst: OmniInstance): string {
  const parts: string[] = []
  parts.push(inst.isActive ? 'ativo' : 'inativo')
  if (inst.isDefault) parts.push('padrão')
  parts.push(inst.hasEvolutionApiKey ? 'com credencial' : 'sem credencial')
  return parts.join(' · ')
}

onMounted(() => void load())
</script>

<template>
  <div class="cfg-tab">
    <p class="cfg-tab__lead">
      Cadastre números escolhendo o provider e a credencial, conecte via QR e defina quem atende.
      <strong>{{ currentChannels }}</strong>
      de
      <strong>{{ maxChannels || '∞' }}</strong>
      número(s) em uso.
    </p>

    <p v-if="loading" class="cfg-tab__loading">Carregando números…</p>

    <template v-else>
      <details class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">Adicionar número</strong>
            <span class="settings-collapse__text">
              Novo número/instância com provider e credencial.
            </span>
          </div>
          <span class="settings-collapse__meta">novo</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body">
          <div class="cfg-grid">
            <label class="cfg-field">
              <span class="cfg-field__label">Nome da instância (único) *</span>
              <input
                v-model="form.instanceName"
                class="cfg-input"
                type="text"
                :disabled="!canManage"
              />
            </label>
            <label class="cfg-field">
              <span class="cfg-field__label">Nome de exibição</span>
              <input
                v-model="form.displayName"
                class="cfg-input"
                type="text"
                :disabled="!canManage"
              />
            </label>
            <label class="cfg-field">
              <span class="cfg-field__label">Telefone (opcional)</span>
              <input
                v-model="form.phoneNumber"
                class="cfg-input"
                type="text"
                :disabled="!canManage"
              />
            </label>
            <AppSelectField
              class="cfg-field"
              label="Provider *"
              :model-value="form.provider"
              :options="OMNI_PROVIDER_OPTIONS"
              :disabled="!canManage"
              @update:model-value="form.provider = $event"
            />
          </div>
          <div class="cfg-tab__form-foot">
            <span v-if="createBlockedReason" class="cfg-tab__hint">{{ createBlockedReason }}</span>
            <AppPanelButton variant="primary" :disabled="!canCreate" @click="submitCreate">
              Cadastrar número
            </AppPanelButton>
          </div>
        </div>
      </details>

      <p v-if="instances.length === 0" class="cfg-empty">
        Nenhum número cadastrado ainda. Use "Adicionar número" acima.
      </p>

      <details v-for="inst in instances" :key="inst.id" class="settings-collapse">
        <summary class="settings-collapse__summary">
          <div class="settings-collapse__title-wrap">
            <strong class="settings-collapse__title">
              {{ inst.displayName || inst.instanceName }}
            </strong>
            <span class="settings-collapse__text">
              {{ inst.phoneNumber || 'sem número pareado' }}
            </span>
          </div>
          <span class="settings-collapse__meta">{{ summaryOf(inst) }}</span>
          <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
            expand_more
          </span>
        </summary>
        <div class="settings-collapse__body">
          <ConfigNumberCard
            :instance="inst"
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

.cfg-tab__lead {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.82rem;
  line-height: 1.4;
}

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

.cfg-tab__form-foot {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 0.75rem;
}

.cfg-tab__hint {
  color: rgb(var(--muted));
  font-size: 0.76rem;
  text-align: right;
}
</style>
