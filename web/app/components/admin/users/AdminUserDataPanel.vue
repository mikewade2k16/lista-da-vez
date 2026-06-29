<script setup lang="ts">
import type { AdminUserItem } from '~/types/admin-users'

// Painel "Dados" do usuario. Espelha a secao de identidade do AdminUserEditDrawer
// legado, mas isolado e reutilizavel. Todos os campos sao identity-global
// (nome/nick/email/ativo/platform admin): so platform_admin pode editar — por isso
// o gate vem de fora via `canEditIdentity` e desabilita TODOS os controles.
const props = defineProps<{ user: AdminUserItem; canEditIdentity: boolean }>()
const emit = defineEmits<{ updated: [] }>()

const m = useAdminUsersManager()

// Form local espelhando o usuario. Re-hidrata sempre que o usuario muda (troca de
// linha no drawer) para nunca exibir dado de um usuario em cima de outro.
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

// Persiste um campo de texto ao perder o foco. Backend so toca o que mudou; aqui
// evitamos PATCH redundante quando o valor e' igual ao do usuario carregado.
function saveText(field: 'displayName' | 'nick' | 'email') {
  if (!props.canEditIdentity) return
  if (form[field] === props.user[field]) return
  m.updateField(props.user.id, field, form[field], { immediate: true })
  emit('updated')
}

// Persiste um switch (ativo / platform admin) na hora da troca.
function saveToggle(field: 'isActive' | 'isPlatformAdmin', value: unknown) {
  if (!props.canEditIdentity) return
  form[field] = Boolean(value)
  m.updateField(props.user.id, field, form[field], { immediate: true })
  emit('updated')
}

// Indicador de salvamento por campo (savingMap usa a chave `${id}:${field}`).
function saving(field: string) {
  return Boolean(m.savingMap.value[`${props.user.id}:${field}`])
}
</script>

<template>
  <section class="admin-user-data">
    <UAlert
      v-if="!canEditIdentity"
      class="admin-user-data__notice"
      color="neutral"
      variant="soft"
      icon="i-lucide-lock"
      description="Apenas platform admin pode editar a identidade do usuario."
    />

    <div class="admin-user-data__grid">
      <label class="admin-user-data__field admin-user-data__field--wide">
        <span class="admin-user-data__label">
          Nome
          <span class="admin-user-data__req">*</span>
        </span>
        <UInput
          :model-value="form.displayName"
          :disabled="!canEditIdentity"
          :loading="saving('displayName')"
          placeholder="Nome de exibicao"
          @update:model-value="form.displayName = String($event ?? '')"
          @blur="saveText('displayName')"
        />
      </label>

      <label class="admin-user-data__field">
        <span class="admin-user-data__label">
          Nick
          <span class="admin-user-data__opt">(opcional)</span>
        </span>
        <UInput
          :model-value="form.nick"
          :disabled="!canEditIdentity"
          :loading="saving('nick')"
          placeholder="Apelido curto"
          @update:model-value="form.nick = String($event ?? '')"
          @blur="saveText('nick')"
        />
      </label>

      <label class="admin-user-data__field">
        <span class="admin-user-data__label">
          E-mail
          <span class="admin-user-data__req">*</span>
        </span>
        <UInput
          :model-value="form.email"
          type="email"
          :disabled="!canEditIdentity"
          :loading="saving('email')"
          placeholder="usuario@dominio.com"
          @update:model-value="form.email = String($event ?? '')"
          @blur="saveText('email')"
        />
      </label>
    </div>

    <div class="admin-user-data__switches">
      <div class="admin-user-data__switch">
        <USwitch
          :model-value="form.isActive"
          :disabled="!canEditIdentity || saving('isActive')"
          @update:model-value="saveToggle('isActive', $event)"
        />
        <div class="admin-user-data__switch-copy">
          <span class="admin-user-data__switch-title">Ativo</span>
          <span class="admin-user-data__switch-hint">
            Usuario inativo nao consegue logar no painel.
          </span>
        </div>
      </div>

      <div class="admin-user-data__switch">
        <USwitch
          :model-value="form.isPlatformAdmin"
          :disabled="!canEditIdentity || saving('isPlatformAdmin')"
          @update:model-value="saveToggle('isPlatformAdmin', $event)"
        />
        <div class="admin-user-data__switch-copy">
          <span class="admin-user-data__switch-title">Platform admin</span>
          <span class="admin-user-data__switch-hint">
            Acesso total a plataforma e a todos os clientes.
          </span>
        </div>
      </div>
    </div>

    <p class="admin-user-data__foot">
      <span class="admin-user-data__req">*</span>
      campos obrigatorios. As alteracoes salvam automaticamente ao sair do campo.
    </p>
  </section>
</template>

<style scoped>
.admin-user-data {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.admin-user-data__notice {
  margin-bottom: 0.25rem;
}

.admin-user-data__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.85rem;
}

.admin-user-data__field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-width: 0;
}

.admin-user-data__field--wide {
  grid-column: 1 / -1;
}

.admin-user-data__label {
  font-size: 0.78rem;
  color: rgb(var(--muted));
}

.admin-user-data__req {
  color: rgb(var(--danger));
}

.admin-user-data__opt {
  color: rgb(var(--muted) / 0.7);
  font-weight: 400;
}

.admin-user-data__switches {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
  gap: 0.75rem;
}

.admin-user-data__switch {
  display: flex;
  align-items: flex-start;
  gap: 0.65rem;
  padding: 0.75rem 0.85rem;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface-2));
}

.admin-user-data__switch-copy {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
}

.admin-user-data__switch-title {
  font-size: 0.88rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.admin-user-data__switch-hint {
  font-size: 0.74rem;
  color: rgb(var(--muted));
}

.admin-user-data__foot {
  margin: 0;
  font-size: 0.74rem;
  color: rgb(var(--muted));
}
</style>
