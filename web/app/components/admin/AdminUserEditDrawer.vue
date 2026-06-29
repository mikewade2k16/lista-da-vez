<script setup lang="ts">
import type { AdminUserItem } from '~/types/admin-users'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import AdminUserDataPanel from './users/AdminUserDataPanel.vue'
import AdminUserMembershipsPanel from './users/AdminUserMembershipsPanel.vue'
import AdminUserRolesPanel from './users/AdminUserRolesPanel.vue'
import AdminUserModulesPanel from './users/AdminUserModulesPanel.vue'
import AdminUserPagesPanel from './users/AdminUserPagesPanel.vue'

const PASSWORD_MIN_LENGTH = 8

const props = defineProps<{ open: boolean; user: AdminUserItem | null }>()
const emit = defineEmits<{ 'update:open': [boolean]; updated: [] }>()

const { setPassword, errorMessage } = useAdminUsersManager()
const auth = useAuthStore()
// Identidade global (senha, is_platform_admin, email, nome) so platform_admin edita.
// O resto (vinculos, papeis, modulos) e delegado e o backend valida o escopo.
const isPlatformAdmin = computed(() => auth.role === 'platform_admin')

const mode = ref<'side' | 'center' | 'fullscreen'>('side')

type TabKey = 'dados' | 'vinculos' | 'papeis' | 'modulos' | 'paginas' | 'senha'
const tabs = computed<{ key: TabKey; label: string }[]>(() => {
  const base: { key: TabKey; label: string }[] = [
    { key: 'dados', label: 'Dados' },
    { key: 'vinculos', label: 'Vinculos' },
    { key: 'papeis', label: 'Papeis' },
    { key: 'modulos', label: 'Modulos' },
    { key: 'paginas', label: 'Paginas' },
  ]
  // Senha e identity-global: so platform_admin (espelha o backend, que retorna 403
  // forbidden_field para campos de identidade vindos de ator delegado).
  if (isPlatformAdmin.value) base.push({ key: 'senha', label: 'Senha' })
  return base
})
const activeTab = ref<TabKey>('dados')

// Abrir um usuario diferente volta para a primeira aba.
watch(
  () => [props.open, props.user?.id],
  () => {
    if (props.open) activeTab.value = 'dados'
  },
)

function onUpdated() {
  emit('updated')
}

// --- Senha (define/reseta) ---
const passwordValue = ref('')
const passwordSaving = ref(false)
const passwordError = computed(() => {
  const pw = passwordValue.value.trim()
  if (!pw) return ''
  return pw.length < PASSWORD_MIN_LENGTH ? `Minimo de ${PASSWORD_MIN_LENGTH} caracteres.` : ''
})
async function submitPassword() {
  if (!props.user || passwordValue.value.trim().length < PASSWORD_MIN_LENGTH) return
  passwordSaving.value = true
  const ok = await setPassword(props.user.id, passwordValue.value.trim())
  passwordSaving.value = false
  if (ok) {
    passwordValue.value = ''
    emit('updated')
  }
}

const subtitle = computed(() => props.user?.email ?? '')
</script>

<template>
  <OmniEntityDrawer
    v-model:mode="mode"
    :model-value="open"
    title="Editar usuario"
    :subtitle="subtitle"
    @update:model-value="emit('update:open', $event)"
  >
    <div v-if="user" class="admin-user-edit">
      <UAlert
        v-if="errorMessage"
        color="error"
        variant="soft"
        icon="i-lucide-alert-triangle"
        :description="errorMessage"
        class="mb-3"
      />

      <nav class="admin-user-edit__tabs">
        <button
          v-for="t in tabs"
          :key="t.key"
          type="button"
          class="admin-user-edit__tab"
          :class="{ 'admin-user-edit__tab--active': activeTab === t.key }"
          @click="activeTab = t.key"
        >
          {{ t.label }}
        </button>
      </nav>

      <div class="admin-user-edit__body">
        <AdminUserDataPanel
          v-if="activeTab === 'dados'"
          :user="user"
          :can-edit-identity="isPlatformAdmin"
          @updated="onUpdated"
        />
        <AdminUserMembershipsPanel
          v-else-if="activeTab === 'vinculos'"
          :user="user"
          @updated="onUpdated"
        />
        <AdminUserRolesPanel v-else-if="activeTab === 'papeis'" :user="user" @updated="onUpdated" />
        <AdminUserModulesPanel
          v-else-if="activeTab === 'modulos'"
          :user="user"
          @updated="onUpdated"
        />
        <AdminUserPagesPanel
          v-else-if="activeTab === 'paginas'"
          :user="user"
          @updated="onUpdated"
        />

        <section v-else-if="activeTab === 'senha'" class="admin-user-edit__senha">
          <h4 class="text-sm font-semibold">Senha</h4>
          <p class="text-xs text-[rgb(var(--muted))] mb-2">
            Define uma nova senha; o usuario passa a logar com ela na hora.
          </p>
          <div class="flex items-start gap-2">
            <div class="flex-1">
              <UInput
                :model-value="passwordValue"
                type="password"
                placeholder="minimo 8 chars"
                @update:model-value="passwordValue = String($event ?? '')"
                @keyup.enter="submitPassword"
              />
              <p v-if="passwordError" class="text-xs text-[rgb(var(--danger))] mt-1">
                {{ passwordError }}
              </p>
            </div>
            <UButton
              label="Salvar senha"
              color="primary"
              :loading="passwordSaving"
              :disabled="passwordSaving || Boolean(passwordError) || !passwordValue.trim()"
              @click="submitPassword"
            />
          </div>
        </section>
      </div>
    </div>

    <template #footer>
      <div class="flex w-full justify-end">
        <UButton
          label="Fechar"
          color="neutral"
          variant="ghost"
          @click="emit('update:open', false)"
        />
      </div>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.admin-user-edit__tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin-bottom: 1rem;
  border-bottom: 1px solid rgb(var(--border));
}

.admin-user-edit__tab {
  padding: 0.5rem 0.85rem;
  font-size: 0.85rem;
  color: rgb(var(--muted));
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
}

.admin-user-edit__tab:hover {
  color: rgb(var(--text));
}

.admin-user-edit__tab--active {
  color: rgb(var(--text));
  border-bottom-color: rgb(var(--primary));
  font-weight: 600;
}

.admin-user-edit__body {
  min-height: 12rem;
}
</style>
