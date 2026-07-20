<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import ConfigNumberCredentials from '~/components/omnichannel/config/ConfigNumberCredentials.vue'
import ConfigNumberCapabilities from '~/components/omnichannel/config/ConfigNumberCapabilities.vue'
import ConfigNumberConnection from '~/components/omnichannel/config/ConfigNumberConnection.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { setInstanceUsers, updateInstance } from '~/domain/omnichannel/config-api'
import { OMNI_PROVIDER_LABEL } from '~/domain/omnichannel/config-types'
import type {
  OmniAssignableUser,
  OmniInstance,
  OmniProvider,
} from '~/domain/omnichannel/config-types'

// Editor de UM número. O provider é fixado na criação (o PATCH do back não o altera) e
// aparece read-only, resolvido pela sessão. Campos editáveis: nome de exibição, telefone,
// fila, responsável, ativo/padrão e usuários atribuídos (o escopo de quem vê as conversas).
const props = defineProps<{
  instance: OmniInstance
  users: OmniAssignableUser[]
  disabled?: boolean
}>()
const emit = defineEmits<{ changed: [] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const draft = reactive({
  displayName: '',
  phoneNumber: '',
  queueLabel: '',
  responsibleUserId: '',
  isActive: true,
  isDefault: false,
})
const assigned = ref<Set<string>>(new Set())
const saving = ref(false)
const resolvedProvider = ref('')

function hydrate(): void {
  draft.displayName = props.instance.displayName || ''
  draft.phoneNumber = props.instance.phoneNumber || ''
  draft.queueLabel = props.instance.queueLabel || ''
  draft.responsibleUserId = props.instance.responsibleUserId || ''
  draft.isActive = props.instance.isActive
  draft.isDefault = props.instance.isDefault
  assigned.value = new Set(props.instance.assignedUserIds || [])
}

watch(() => props.instance, hydrate, { immediate: true })

const providerLabel = computed(() => {
  const p = (resolvedProvider.value || '') as OmniProvider
  return OMNI_PROVIDER_LABEL[p] || resolvedProvider.value || 'não resolvido'
})

const responsibleOptions = computed(() => [
  { value: '', label: 'Sem responsável' },
  ...props.users.map((u) => ({ value: u.id, label: u.name || u.email, meta: u.email })),
])

function toggleAssigned(userId: string): void {
  const next = new Set(assigned.value)
  if (next.has(userId)) next.delete(userId)
  else next.add(userId)
  assigned.value = next
}

async function save(): Promise<void> {
  saving.value = true
  try {
    await updateInstance(api, props.instance.id, {
      displayName: draft.displayName.trim(),
      phoneNumber: draft.phoneNumber.trim(),
      queueLabel: draft.queueLabel.trim(),
      responsibleUserId: draft.responsibleUserId,
      userScopePolicy: props.instance.userScopePolicy,
      isActive: draft.isActive,
      isDefault: draft.isDefault,
    })
    await setInstanceUsers(api, props.instance.id, [...assigned.value])
    ui.success('Número atualizado.')
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível salvar o número.'))
  } finally {
    saving.value = false
  }
}

async function deactivate(): Promise<void> {
  const { confirmed } = await ui.confirm({
    title: 'Desativar número?',
    message:
      'O número fica inativo e libera um canal da conta. As conversas já recebidas continuam visíveis no inbox. Pode reativar depois.',
    confirmLabel: 'Desativar',
    cancelLabel: 'Cancelar',
  })
  if (!confirmed) return
  saving.value = true
  try {
    await updateInstance(api, props.instance.id, {
      userScopePolicy: props.instance.userScopePolicy,
      isActive: false,
    })
    ui.success('Número desativado.')
    emit('changed')
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível desativar o número.'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="cfg-card">
    <div class="cfg-card__meta">
      <span class="cfg-field__label">Provider</span>
      <span class="cfg-card__provider">{{ providerLabel }}</span>
      <span class="cfg-card__note">Definido na criação — não pode ser trocado depois.</span>
    </div>

    <div class="cfg-grid">
      <label class="cfg-field">
        <span class="cfg-field__label">Nome de exibição</span>
        <input v-model="draft.displayName" class="cfg-input" type="text" :disabled="disabled" />
      </label>
      <label class="cfg-field">
        <span class="cfg-field__label">Telefone (só dígitos, com DDD)</span>
        <input v-model="draft.phoneNumber" class="cfg-input" type="text" :disabled="disabled" />
      </label>
      <label class="cfg-field">
        <span class="cfg-field__label">Rótulo da fila (opcional)</span>
        <input v-model="draft.queueLabel" class="cfg-input" type="text" :disabled="disabled" />
      </label>
      <AppSelectField
        class="cfg-field"
        label="Responsável"
        :model-value="draft.responsibleUserId"
        :options="responsibleOptions"
        :disabled="disabled"
        @update:model-value="draft.responsibleUserId = $event"
      />
    </div>

    <div class="cfg-toggles">
      <AppToggleSwitch v-model="draft.isActive" :disabled="disabled" label="Ativo" />
      <AppToggleSwitch v-model="draft.isDefault" :disabled="disabled" label="Número padrão" />
    </div>

    <section class="cfg-block">
      <span class="cfg-field__label">Atendentes com acesso a este número</span>
      <p v-if="users.length === 0" class="cfg-empty">Nenhum atendente elegível na conta.</p>
      <div v-else class="cfg-users">
        <label v-for="u in users" :key="u.id" class="cfg-user">
          <input
            type="checkbox"
            :checked="assigned.has(u.id)"
            :disabled="disabled"
            @change="toggleAssigned(u.id)"
          />
          <span>{{ u.name || u.email }}</span>
        </label>
      </div>
    </section>

    <section class="cfg-block">
      <ConfigNumberConnection
        :instance-name="instance.instanceName"
        :disabled="disabled"
        @provider-resolved="resolvedProvider = $event"
      />
    </section>

    <section class="cfg-block">
      <ConfigNumberCredentials
        :instance-name="instance.instanceName"
        :initial-set="instance.hasEvolutionApiKey"
        :disabled="disabled"
        @saved="emit('changed')"
      />
    </section>

    <section class="cfg-block">
      <span class="cfg-field__label">Capacidades do número</span>
      <ConfigNumberCapabilities :instance-id="instance.id" />
    </section>

    <div class="cfg-card__actions">
      <AppPanelButton
        v-if="instance.isActive"
        variant="ghost"
        :disabled="disabled || saving"
        @click="deactivate"
      >
        Desativar número
      </AppPanelButton>
      <AppPanelButton variant="primary" :disabled="disabled || saving" @click="save">
        Salvar número
      </AppPanelButton>
    </div>
  </div>
</template>

<style scoped>
.cfg-card {
  display: grid;
  gap: 0.85rem;
}

.cfg-card__meta {
  display: grid;
  gap: 0.15rem;
}

.cfg-card__provider {
  font-size: 0.85rem;
  font-weight: 700;
  color: rgb(var(--text));
}

.cfg-card__note {
  font-size: 0.72rem;
  color: rgb(var(--muted));
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
}

.cfg-block {
  display: grid;
  gap: 0.5rem;
  padding-top: 0.65rem;
  border-top: 1px solid rgb(var(--border) / 0.6);
}

.cfg-users {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.35rem;
}

.cfg-user {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.8rem;
  color: rgb(var(--text));
}

.cfg-empty {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.8rem;
}

.cfg-card__actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
