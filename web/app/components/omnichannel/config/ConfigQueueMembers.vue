<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import {
  addQueueMember,
  fetchQueueMembers,
  removeQueueMember,
} from '~/domain/omnichannel/config-api'
import type { OmniAssignableUser, OmniQueueMember } from '~/domain/omnichannel/config-types'

// Membros da fila = incremental (POST/DELETE um a um). A tela guarda a seleção, faz o DIFF
// contra o back e dispara as chamadas; se uma falhar, mostra quem entrou e quem falhou —
// nunca um "salvou" mudo. queue_members É o gate de dado: remover alguém tira o acesso dele
// às conversas daquela fila (o aviso deixa isso explícito).
const props = defineProps<{
  queueId: string
  users: OmniAssignableUser[]
  disabled?: boolean
}>()

const auth = useAuthStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()
const api = createApiRequest(runtimeConfig, () => auth.accessToken)

const current = ref<Set<string>>(new Set())
const selection = ref<Set<string>>(new Set())
const loading = ref(false)
const saving = ref(false)

async function load(): Promise<void> {
  loading.value = true
  try {
    const members: OmniQueueMember[] = await fetchQueueMembers(api, props.queueId)
    const active = new Set(members.filter((m) => m.isActive).map((m) => m.userId))
    current.value = active
    selection.value = new Set(active)
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Não foi possível carregar os membros da fila.'))
  } finally {
    loading.value = false
  }
}

const toAdd = computed(() => [...selection.value].filter((id) => !current.value.has(id)))
const toRemove = computed(() => [...current.value].filter((id) => !selection.value.has(id)))
const dirty = computed(() => toAdd.value.length > 0 || toRemove.value.length > 0)

function toggle(userId: string): void {
  const next = new Set(selection.value)
  if (next.has(userId)) next.delete(userId)
  else next.add(userId)
  selection.value = next
}

function nameOf(userId: string): string {
  const u = props.users.find((x) => x.id === userId)
  return u?.name || u?.email || userId
}

async function save(): Promise<void> {
  if (!dirty.value || saving.value) return
  saving.value = true
  const added: string[] = []
  const removed: string[] = []
  const failed: string[] = []
  for (const id of toAdd.value) {
    try {
      await addQueueMember(api, props.queueId, id)
      added.push(nameOf(id))
    } catch {
      failed.push(nameOf(id))
    }
  }
  for (const id of toRemove.value) {
    try {
      await removeQueueMember(api, props.queueId, id)
      removed.push(nameOf(id))
    } catch {
      failed.push(nameOf(id))
    }
  }
  await load()
  saving.value = false

  const parts: string[] = []
  if (added.length) parts.push(`entraram: ${added.join(', ')}`)
  if (removed.length) parts.push(`saíram: ${removed.join(', ')}`)
  if (failed.length) {
    ui.error(
      `Falharam: ${failed.join(', ')}.${parts.length ? ' Aplicados — ' + parts.join('; ') + '.' : ''}`,
    )
  } else if (parts.length) {
    ui.success(`Membros atualizados (${parts.join('; ')}).`)
  }
}

onMounted(() => void load())
watch(
  () => props.queueId,
  () => void load(),
)
</script>

<template>
  <div class="cfg-members">
    <span class="cfg-field__label">Atendentes desta fila</span>
    <p class="cfg-members__warn">
      Remover um atendente tira o acesso dele às conversas desta fila.
    </p>
    <p v-if="loading" class="cfg-members__hint">Carregando membros…</p>
    <p v-else-if="users.length === 0" class="cfg-members__hint">
      Nenhum atendente elegível na conta.
    </p>
    <div v-else class="cfg-members__list">
      <label v-for="u in users" :key="u.id" class="cfg-members__item">
        <input
          type="checkbox"
          :checked="selection.has(u.id)"
          :disabled="disabled || saving"
          @change="toggle(u.id)"
        />
        <span>{{ u.name || u.email }}</span>
      </label>
    </div>
    <div class="cfg-members__foot">
      <span v-if="dirty" class="cfg-members__hint">
        {{ toAdd.length }} a adicionar · {{ toRemove.length }} a remover
      </span>
      <AppPanelButton variant="secondary" :disabled="disabled || saving || !dirty" @click="save">
        Salvar membros
      </AppPanelButton>
    </div>
  </div>
</template>

<style scoped>
.cfg-members {
  display: grid;
  gap: 0.4rem;
}

.cfg-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: rgb(var(--muted));
}

.cfg-members__warn {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.cfg-members__hint {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.cfg-members__list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.35rem;
}

.cfg-members__item {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.8rem;
  color: rgb(var(--text));
}

.cfg-members__foot {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 0.3rem;
}
</style>
