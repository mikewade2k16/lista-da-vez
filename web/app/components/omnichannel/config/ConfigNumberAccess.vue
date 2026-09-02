<script setup lang="ts">
import { computed, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useOmnichannelInstanceAccessEditor } from '~/composables/omnichannel/useOmnichannelInstanceAccessEditor'
import { useOmnichannelScopeInvalidation } from '~/composables/omnichannel/useOmnichannelScopeInvalidation'
import type { OmniAssignableUser, OmniInstanceGrantLevel } from '~/domain/omnichannel/config-types'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest } from '~/utils/api-client'

const props = defineProps<{
  instanceId: string
  users: OmniAssignableUser[]
  reloadInstances: () => Promise<void>
  disabled?: boolean
}>()
const emit = defineEmits<{ changed: [] }>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)
const { accountId, publishLocalAccessChange } = useOmnichannelScopeInvalidation()
const editor = useOmnichannelInstanceAccessEditor({
  api,
  instanceId: () => props.instanceId,
})
const {
  status,
  authoritative,
  accessPolicy,
  responsibleUserId,
  grantLevels,
  errorMessage,
  activeGrantCount,
  responsibleHasManage,
  validationError,
} = editor

const responsibleOptions = computed(() => [
  { value: '', label: 'Selecione um responsável' },
  ...props.users.map((user) => ({
    value: user.id,
    label: user.name || user.email,
    meta: user.email,
  })),
])
const accessPolicyOptions = [
  { value: 'ACCOUNT_SHARED', label: 'Compartilhado com a conta' },
  { value: 'RESTRICTED', label: 'Restrito ao responsável e selecionados' },
]
const accessLevelOptions = [
  { value: '', label: 'Sem acesso' },
  { value: 'view', label: 'Somente visualizar' },
  { value: 'reply', label: 'Visualizar e responder' },
  { value: 'manage', label: 'Gerenciar conexão' },
]

watch(
  () => props.instanceId,
  () => void editor.load(),
  { immediate: true },
)

function accessLevelOf(userId: string): OmniInstanceGrantLevel | '' {
  return grantLevels.value[userId] || ''
}

function updateAccessLevel(userId: string, value: string): void {
  editor.setGrant(userId, value as OmniInstanceGrantLevel | '')
}

async function save(): Promise<void> {
  const result = await editor.save()
  if (result === 'saved') {
    publishLocalAccessChange(
      accountId.value,
      props.instanceId,
      authoritative.value?.accessRevision || 0,
    )
    emit('changed')
    try {
      await props.reloadInstances()
      ui.success('Acessos da conexão atualizados.')
    } catch {
      ui.error('Acessos salvos, mas a lista de conexões não pôde ser atualizada.')
    }
    return
  }
  if (result === 'invalid') {
    ui.error(validationError.value || 'Revise os acessos antes de salvar.')
    return
  }
  if (result === 'conflict' || result === 'error') ui.error(errorMessage.value)
}
</script>

<template>
  <details class="cfg-access">
    <summary>
      <span>Acesso da conexão</span>
      <small v-if="status === 'ready' || status === 'saving'">
        {{ accessPolicy === 'ACCOUNT_SHARED' ? 'compartilhado' : 'restrito' }} ·
        {{ activeGrantCount }} grant(s)
      </small>
      <small v-else>{{ status }}</small>
    </summary>
    <div class="cfg-access__body">
      <p v-if="status === 'idle' || status === 'loading'" class="cfg-access__muted">
        Carregando acessos autoritativos…
      </p>
      <div v-else-if="status === 'error'" class="cfg-access__error">
        <p>{{ errorMessage }}</p>
        <AppPanelButton variant="secondary" :disabled="disabled" @click="editor.load">
          Tentar novamente
        </AppPanelButton>
      </div>
      <div v-else class="cfg-access__editor">
        <div class="cfg-access__grid">
          <AppSelectField
            label="Política de acesso"
            :model-value="accessPolicy"
            :options="accessPolicyOptions"
            :disabled="disabled || status === 'saving'"
            @update:model-value="accessPolicy = $event"
          />
          <AppSelectField
            label="Responsável principal"
            :model-value="responsibleUserId"
            :options="responsibleOptions"
            :disabled="disabled || status === 'saving'"
            @update:model-value="editor.setResponsible"
          />
        </div>

        <p class="cfg-access__description">
          <template v-if="accessPolicy === 'ACCOUNT_SHARED'">
            Membros da conta usam as permissões efetivas para visualizar e responder. Somente
            usuários com grant manage configuram esta conexão.
          </template>
          <template v-else>
            Somente o responsável e os usuários selecionados alcançam esta conexão, respeitando
            também as permissões da conta.
          </template>
        </p>

        <p v-if="users.length === 0" class="cfg-access__muted">Nenhum usuário elegível na conta.</p>
        <div v-else class="cfg-access__users">
          <div v-for="user in users" :key="user.id" class="cfg-access__user">
            <span>
              <strong>{{ user.name || user.email }}</strong>
              <small>{{ user.email }}</small>
            </span>
            <AppSelectField
              :model-value="accessLevelOf(user.id)"
              :options="accessLevelOptions"
              :disabled="disabled || status === 'saving'"
              @update:model-value="updateAccessLevel(user.id, $event)"
            />
          </div>
        </div>

        <p v-if="!responsibleHasManage || validationError" class="cfg-access__warning">
          {{ validationError || 'O responsável principal precisa manter nível manage.' }}
        </p>
        <p v-if="errorMessage" class="cfg-access__warning">{{ errorMessage }}</p>
        <div class="cfg-access__actions">
          <AppPanelButton
            variant="primary"
            :disabled="disabled || status === 'saving' || !!validationError"
            @click="save"
          >
            {{ status === 'saving' ? 'Salvando acessos…' : 'Salvar acessos' }}
          </AppPanelButton>
        </div>
      </div>
    </div>
  </details>
</template>

<style scoped>
.cfg-access {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}

.cfg-access > summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.7rem 0.75rem;
  color: rgb(var(--text));
  cursor: pointer;
}

.cfg-access summary small,
.cfg-access__description,
.cfg-access__muted,
.cfg-access__error p,
.cfg-access__warning {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.cfg-access__body,
.cfg-access__editor,
.cfg-access__error {
  display: grid;
  gap: 0.75rem;
}

.cfg-access__body {
  padding: 0 0.75rem 0.75rem;
}

.cfg-access__grid,
.cfg-access__users {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.5rem;
}

.cfg-access__description,
.cfg-access__muted,
.cfg-access__error p,
.cfg-access__warning {
  margin: 0;
}

.cfg-access__user {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(150px, 0.8fr);
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}

.cfg-access__user > span {
  display: grid;
  min-width: 0;
  color: rgb(var(--text));
}

.cfg-access__user small {
  overflow: hidden;
  color: rgb(var(--muted));
  font-size: 0.68rem;
  text-overflow: ellipsis;
}

.cfg-access__warning {
  color: rgb(var(--warning));
}

.cfg-access__actions {
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 640px) {
  .cfg-access__user {
    grid-template-columns: 1fr;
  }
}
</style>
