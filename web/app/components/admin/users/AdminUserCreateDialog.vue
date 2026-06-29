<script setup lang="ts">
import { useAdminUsersManager } from '~/composables/useAdminUsersManager'

// Modal de criacao de usuario, fatiado do AdminUsersWorkspace para deixar o host
// fino (apresentacao + form local isolados aqui). Comportamento IDENTICO ao anterior:
// mesmas validacoes (senha minima, exige cliente/agencia/platform admin, confirmacao
// de membro-de-agencia), mesmo body de createUser e mesmo evento de sucesso.
// O host controla a abertura (v-model:open) e popula as opcoes (props); a criacao bate
// no manager COMPARTILHADO (useAdminUsersManager via inject), preservando a fonte unica.

// Espelha o minimo do backend (admin_users_service.go: "must be at least 8 chars").
const PASSWORD_MIN_LENGTH = 8

const props = defineProps<{
  open: boolean
  canCreate: boolean
  accountOptions: { value: string; label: string }[]
  organizationOptions: { value: string; label: string }[]
}>()
const emit = defineEmits<{ 'update:open': [boolean]; created: [string] }>()

const { creating, createUser } = useAdminUsersManager()

const createForm = reactive({
  email: '',
  displayName: '',
  nick: '',
  isPlatformAdmin: false,
  temporaryPassword: '',
  accountId: '',
  organizationId: '',
  role: 'owner',
  orgRole: 'agency_member',
})
const createAgencyConfirmed = ref(false)

function resetForm() {
  createForm.email = ''
  createForm.displayName = ''
  createForm.nick = ''
  createForm.isPlatformAdmin = false
  createForm.temporaryPassword = ''
  createForm.accountId = ''
  createForm.organizationId = ''
  createForm.role = 'owner'
  createForm.orgRole = 'agency_member'
  createAgencyConfirmed.value = false
}

// Abrir o modal sempre reseta o form (mesma semantica do openCreate antigo).
watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) resetForm()
  },
)

// Senha na criacao: opcional (vazia = fluxo de convite), mas se preenchida tem
// que respeitar o minimo do backend. Bloqueia o submit com hint inline.
const createPasswordError = computed(() => {
  const pw = createForm.temporaryPassword.trim()
  if (!pw) return ''
  return pw.length < PASSWORD_MIN_LENGTH ? `Minimo de ${PASSWORD_MIN_LENGTH} caracteres.` : ''
})

// Um usuario sem cliente/agencia e sem ser platform_admin nao consegue logar (sem
// papel resolvido o login falha). Evita criar um usuario "inutil": exige cliente
// (com papel), agencia (com cargo) OU a flag de platform admin.
const createNeedsClient = computed(
  () => !createForm.isPlatformAdmin && !createForm.accountId && !createForm.organizationId,
)

// Vincular Cliente + Agencia juntos torna o usuario MEMBRO DA AGENCIA (ve todos os
// clientes/modulos da agencia) — perigoso para um usuario que deveria ser so deste
// cliente. Quando os dois selects estao preenchidos, exigimos confirmacao explicita.
const createBindsClientAndAgency = computed(
  () => Boolean(createForm.accountId) && Boolean(createForm.organizationId),
)

async function submitCreate() {
  if (!props.canCreate || createPasswordError.value || createNeedsClient.value) return
  if (createBindsClientAndAgency.value && !createAgencyConfirmed.value) return
  const createdId = await createUser({ ...createForm })
  if (!createdId) return
  emit('update:open', false)
  emit('created', createdId)
}
</script>

<template>
  <UModal :open="open" @update:open="emit('update:open', $event)">
    <template #content>
      <UCard>
        <template #header>
          <h3 class="text-base font-semibold">Novo usuario</h3>
        </template>

        <div class="space-y-3">
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Email</label>
            <UInput
              :model-value="createForm.email"
              placeholder="usuario@exemplo.com"
              @update:model-value="createForm.email = String($event ?? '')"
            />
          </div>
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nome</label>
            <UInput
              :model-value="createForm.displayName"
              placeholder="Nome completo"
              @update:model-value="createForm.displayName = String($event ?? '')"
            />
          </div>
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nick (opcional)</label>
            <UInput
              :model-value="createForm.nick"
              placeholder="apelido curto"
              @update:model-value="createForm.nick = String($event ?? '')"
            />
          </div>
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">
              Senha temporaria (opcional — se vazia, user precisa convite)
            </label>
            <UInput
              :model-value="createForm.temporaryPassword"
              type="password"
              placeholder="minimo 8 chars"
              @update:model-value="createForm.temporaryPassword = String($event ?? '')"
            />
            <p v-if="createPasswordError" class="text-xs text-[rgb(var(--danger))] mt-1">
              {{ createPasswordError }}
            </p>
          </div>
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Cliente (opcional)</label>
            <select
              class="w-full rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2 text-sm"
              :value="createForm.accountId"
              @change="createForm.accountId = ($event.target as HTMLSelectElement).value"
            >
              <option v-for="opt in accountOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
            <p v-if="createNeedsClient" class="text-xs text-[rgb(var(--danger))] mt-1">
              Selecione um cliente, uma agencia (abaixo) ou marque platform admin — senao o usuario
              nao consegue logar.
            </p>
          </div>
          <div v-if="createForm.accountId">
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Papel no cliente</label>
            <select
              class="w-full rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2 text-sm"
              :value="createForm.role"
              @change="createForm.role = ($event.target as HTMLSelectElement).value"
            >
              <option value="owner">Owner (acesso total do cliente)</option>
              <option value="director">Director</option>
              <option value="marketing">Marketing</option>
            </select>
            <p class="text-xs text-[rgb(var(--muted))] mt-1">
              Cria o papel legado (login + operacao). Sem isso o usuario nao consegue entrar.
            </p>
          </div>
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Agencia (opcional)</label>
            <select
              class="w-full rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2 text-sm"
              :value="createForm.organizationId"
              @change="createForm.organizationId = ($event.target as HTMLSelectElement).value"
            >
              <option v-for="opt in organizationOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </div>
          <div v-if="createForm.organizationId">
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Cargo na agencia</label>
            <select
              class="w-full rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2 text-sm"
              :value="createForm.orgRole"
              @change="createForm.orgRole = ($event.target as HTMLSelectElement).value"
            >
              <option value="agency_owner">Dono da agencia (acesso total)</option>
              <option value="agency_member">Membro (acesso limitado)</option>
            </select>
            <p class="text-xs text-[rgb(var(--muted))] mt-1">
              O cargo define o acesso: dono ve tudo da agencia; membro tem acesso limitado. Ele
              entra como membro da conta-agencia e navega pelos clientes da agencia.
            </p>
          </div>
          <div
            v-if="createBindsClientAndAgency"
            class="rounded-[var(--radius-md)] border border-[rgb(var(--danger))] bg-[rgb(var(--surface-2))] px-3 py-2"
          >
            <p class="text-xs text-[rgb(var(--danger))]">
              Atencao: vincular uma agencia torna o usuario MEMBRO DA AGENCIA — ele passa a ver
              todos os clientes e modulos da agencia. Para um usuario so deste cliente, deixe
              Agencia vazio.
            </p>
            <label class="mt-2 flex items-center gap-2 text-xs">
              <input v-model="createAgencyConfirmed" type="checkbox" />
              <span>Entendo, e um membro de agencia</span>
            </label>
          </div>
          <div class="flex items-center gap-2">
            <USwitch v-model="createForm.isPlatformAdmin" />
            <span class="text-sm">Platform admin (acesso global)</span>
          </div>
        </div>

        <template #footer>
          <div class="flex justify-end gap-2">
            <UButton
              label="Cancelar"
              color="neutral"
              variant="ghost"
              @click="emit('update:open', false)"
            />
            <UButton
              label="Criar"
              color="primary"
              :loading="creating"
              :disabled="
                creating ||
                Boolean(createPasswordError) ||
                createNeedsClient ||
                (createBindsClientAndAgency && !createAgencyConfirmed)
              "
              @click="submitCreate"
            />
          </div>
        </template>
      </UCard>
    </template>
  </UModal>
</template>
