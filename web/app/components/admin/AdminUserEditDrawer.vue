<script setup lang="ts">
import type { AccountMembershipItem, AdminUserItem } from '~/types/admin-users'

const PASSWORD_MIN_LENGTH = 8

// Niveis editaveis por vinculo (tenant-scoped, nao exigem loja). Espelha a
// validacao do backend (UpdateMembershipRole: owner/director/marketing).
const ROLE_OPTIONS = [
  { value: 'owner', label: 'Owner — acesso total do cliente/agencia' },
  { value: 'director', label: 'Director — acesso amplo, sem gestao de usuarios' },
  { value: 'marketing', label: 'Marketing — foco em campanhas/site' },
]
const ROLE_LABELS: Record<string, string> = {
  owner: 'Owner',
  director: 'Director',
  marketing: 'Marketing',
  manager: 'Gerente',
  consultant: 'Consultor',
  store_terminal: 'Terminal de loja',
  supervisor: 'Supervisor',
  '': 'Sem papel',
}

const props = defineProps<{ open: boolean; user: AdminUserItem | null }>()
const emit = defineEmits<{ 'update:open': [boolean]; updated: [] }>()

const { updateField, setPassword, fetchMemberships, updateMembershipRole, errorMessage } =
  useAdminUsersManager()
const auth = useAuthStore()
const canManage = computed(() => auth.role === 'platform_admin')

// Dados basicos — espelham o user e salvam via PATCH ao perder o foco / mudar.
const form = reactive({
  displayName: '',
  nick: '',
  email: '',
  isActive: true,
  isPlatformAdmin: false,
})
watch(
  () => props.user,
  (u) => {
    if (!u) return
    form.displayName = u.displayName
    form.nick = u.nick
    form.email = u.email
    form.isActive = u.isActive
    form.isPlatformAdmin = u.isPlatformAdmin
  },
  { immediate: true },
)

function saveField(field: 'displayName' | 'nick' | 'email' | 'isActive' | 'isPlatformAdmin') {
  if (!props.user || !canManage.value) return
  updateField(props.user.id, field, form[field], { immediate: true })
  emit('updated')
}

function toggleField(field: 'isActive' | 'isPlatformAdmin', value: unknown) {
  form[field] = Boolean(value)
  saveField(field)
}

// Vinculos (cliente/agencia) + nivel por vinculo.
const memberships = ref<AccountMembershipItem[]>([])
// Indicador read-only no header: o usuario e membro de alguma conta-agencia (ve
// todos os clientes/modulos da agencia). O drawer NAO cria vinculo de agencia,
// entao aqui e so sinalizacao.
const isAgencyMember = computed(() => memberships.value.some((m) => m.isAgency))
const loadingMemberships = ref(false)
const savingAccountId = ref('')

async function loadMemberships() {
  if (!props.user) return
  loadingMemberships.value = true
  memberships.value = await fetchMemberships(props.user.id)
  loadingMemberships.value = false
}

watch(
  () => [props.open, props.user?.id],
  () => {
    if (props.open && props.user) void loadMemberships()
  },
  { immediate: true },
)

function roleOptionsFor(membership: AccountMembershipItem) {
  // Garante que o papel atual apareca mesmo quando nao for um dos editaveis.
  if (membership.role && !ROLE_OPTIONS.some((option) => option.value === membership.role)) {
    return [
      { value: membership.role, label: ROLE_LABELS[membership.role] || membership.role },
      ...ROLE_OPTIONS,
    ]
  }
  return ROLE_OPTIONS
}

async function changeRole(accountId: string, role: string) {
  if (!props.user || !canManage.value) return
  savingAccountId.value = accountId
  const next = await updateMembershipRole(props.user.id, accountId, role)
  savingAccountId.value = ''
  if (next) {
    memberships.value = next
    emit('updated')
  }
}

// Senha (define/reseta).
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
</script>

<template>
  <UModal :open="open" @update:open="emit('update:open', $event)">
    <template #content>
      <UCard>
        <template #header>
          <div>
            <div class="flex items-center gap-2">
              <h3 class="text-base font-semibold">Editar usuario</h3>
              <UBadge v-if="isAgencyMember" color="primary" variant="soft" size="xs">
                Membro de agencia
              </UBadge>
            </div>
            <p class="text-xs text-[rgb(var(--muted))]">{{ user?.email }}</p>
          </div>
        </template>

        <div class="space-y-5 max-h-[70vh] overflow-y-auto">
          <UAlert
            v-if="errorMessage"
            color="error"
            variant="soft"
            icon="i-lucide-alert-triangle"
            :description="errorMessage"
          />

          <section class="space-y-3">
            <h4 class="text-sm font-semibold">Dados</h4>
            <div>
              <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nome</label>
              <UInput
                :model-value="form.displayName"
                :disabled="!canManage"
                @update:model-value="form.displayName = String($event ?? '')"
                @blur="saveField('displayName')"
              />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nick</label>
                <UInput
                  :model-value="form.nick"
                  :disabled="!canManage"
                  @update:model-value="form.nick = String($event ?? '')"
                  @blur="saveField('nick')"
                />
              </div>
              <div>
                <label class="block text-xs text-[rgb(var(--muted))] mb-1">Email</label>
                <UInput
                  :model-value="form.email"
                  :disabled="!canManage"
                  @update:model-value="form.email = String($event ?? '')"
                  @blur="saveField('email')"
                />
              </div>
            </div>
            <div class="flex items-center gap-6">
              <div class="flex items-center gap-2">
                <USwitch
                  :model-value="form.isActive"
                  :disabled="!canManage"
                  @update:model-value="toggleField('isActive', $event)"
                />
                <span class="text-sm">Ativo</span>
              </div>
              <div class="flex items-center gap-2">
                <USwitch
                  :model-value="form.isPlatformAdmin"
                  :disabled="!canManage"
                  @update:model-value="toggleField('isPlatformAdmin', $event)"
                />
                <span class="text-sm">Platform admin</span>
              </div>
            </div>
          </section>

          <section class="space-y-3">
            <div>
              <h4 class="text-sm font-semibold">Vinculos e nivel de acesso</h4>
              <p class="text-xs text-[rgb(var(--muted))]">
                Cliente ou agencia que o usuario pertence. O nivel define o que ele acessa dentro
                daquele cliente/agencia.
              </p>
            </div>

            <p v-if="loadingMemberships" class="text-xs text-[rgb(var(--muted))]">
              Carregando vinculos...
            </p>
            <p v-else-if="memberships.length === 0" class="text-xs text-[rgb(var(--muted))]">
              Sem vinculos. Sem cliente/agencia o usuario nao loga (a menos que seja platform
              admin).
            </p>

            <div
              v-for="m in memberships"
              :key="m.accountId"
              class="flex items-center justify-between gap-3 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2"
            >
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="font-medium truncate">{{ m.accountName }}</span>
                  <UBadge :color="m.isAgency ? 'primary' : 'neutral'" variant="soft" size="xs">
                    {{ m.isAgency ? 'agencia' : 'cliente' }}
                  </UBadge>
                  <UBadge v-if="!m.isActive" color="neutral" variant="soft" size="xs">
                    inativo
                  </UBadge>
                </div>
                <span class="text-xs text-[rgb(var(--muted))]">{{ m.accountSlug }}</span>
              </div>
              <select
                class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] px-2 py-1 text-sm"
                :value="m.role"
                :disabled="!canManage || savingAccountId === m.accountId"
                @change="changeRole(m.accountId, ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="opt in roleOptionsFor(m)" :key="opt.value" :value="opt.value">
                  {{ opt.label }}
                </option>
              </select>
            </div>
          </section>

          <section v-if="canManage" class="space-y-3">
            <div>
              <h4 class="text-sm font-semibold">Senha</h4>
              <p class="text-xs text-[rgb(var(--muted))]">
                Define uma nova senha; o usuario passa a logar com ela na hora.
              </p>
            </div>
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

          <section>
            <h4 class="text-sm font-semibold">Modulos e paginas</h4>
            <p class="text-xs text-[rgb(var(--muted))]">
              Dar/remover acesso a modulos e paginas por usuario entra aqui na proxima etapa (Fase
              1B). Por enquanto, ajuste pelo detalhe em /operacao/usuarios.
            </p>
          </section>
        </div>

        <template #footer>
          <div class="flex justify-end">
            <UButton
              label="Fechar"
              color="neutral"
              variant="ghost"
              @click="emit('update:open', false)"
            />
          </div>
        </template>
      </UCard>
    </template>
  </UModal>
</template>
