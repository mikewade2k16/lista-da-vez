<script setup>
import { computed, reactive, ref, watch } from "vue";
import AppSelectField from "~/components/ui/AppSelectField.vue";

import { canManageUserPasswords, canManageUsers, getRoleLabel } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { useUiStore } from "~/stores/ui";
import { useUsersStore } from "~/stores/users";

const auth = useAuthStore();
const ui = useUiStore();
const usersStore = useUsersStore();

const editingUserId = ref("");
const createMode = ref("invite");
const createDraft = reactive({
  displayName: "",
  email: "",
  password: "",
  role: "consultant",
  tenantId: "",
  storeIds: [],
  active: true
});
const editDraft = reactive({
  displayName: "",
  email: "",
  password: "",
  role: "consultant",
  tenantId: "",
  storeIds: [],
  active: true
});

const canEditUsers = computed(() => canManageUsers(auth.role));
const canManagePasswords = computed(() => canManageUserPasswords(auth.role));
const tenantOptions = computed(() => auth.tenantContext || []);
const allStoreOptions = computed(() => auth.storeContext || []);
const roleCatalog = computed(() => usersStore.assignableRoles || []);
const genericRoleCatalog = computed(() => roleCatalog.value.filter((role) => role.id !== "consultant"));
const createRoleCatalog = computed(() => genericRoleCatalog.value);
const tenantSelectOptions = computed(() =>
  tenantOptions.value.map((tenant) => ({
    value: String(tenant.id || "").trim(),
    label: String(tenant.name || "").trim()
  }))
);
const genericRoleOptions = computed(() =>
  genericRoleCatalog.value.map((role) => ({
    value: String(role.id || "").trim(),
    label: String(role.label || "").trim()
  }))
);
const createRoleOptions = computed(() =>
  createRoleCatalog.value.map((role) => ({
    value: String(role.id || "").trim(),
    label: String(role.label || "").trim()
  }))
);

function isConsultantManaged(user) {
  return String(user?.managedBy || "").trim() === "consultants" || String(user?.role || "").trim() === "consultant";
}

function getRoleDefinition(roleId) {
  return roleCatalog.value.find((role) => role.id === roleId) || null;
}

function isStoreScoped(roleId) {
  return getRoleDefinition(roleId)?.scope === "store";
}

function isSingleStoreScoped(roleId) {
  return isStoreScoped(roleId);
}

function isTenantScoped(roleId) {
  return getRoleDefinition(roleId)?.scope === "tenant";
}

function syncDraftScope(draft) {
  if (isStoreScoped(draft.role)) {
    draft.tenantId = draft.tenantId || auth.activeTenantId || tenantOptions.value[0]?.id || "";
    return;
  }

  draft.storeIds = [];
  if (isTenantScoped(draft.role)) {
    draft.tenantId = draft.tenantId || auth.activeTenantId || tenantOptions.value[0]?.id || "";
    return;
  }

  draft.tenantId = "";
}

function resetCreateDraft() {
  createDraft.displayName = "";
  createDraft.email = "";
  createDraft.password = "";
  createMode.value = canManagePasswords.value ? "invite" : "invite";
  createDraft.role = createRoleCatalog.value[0]?.id || "store_terminal";
  createDraft.tenantId = auth.activeTenantId || tenantOptions.value[0]?.id || "";
  createDraft.storeIds = [];
  createDraft.active = true;
  syncDraftScope(createDraft);
}

function resetEditDraft(user = null) {
  editingUserId.value = user?.id || "";
  editDraft.displayName = user?.displayName || "";
  editDraft.email = user?.email || "";
  editDraft.password = "";
  editDraft.role = user?.role || genericRoleCatalog.value[0]?.id || "manager";
  editDraft.tenantId = user?.tenantId || auth.activeTenantId || tenantOptions.value[0]?.id || "";
  editDraft.storeIds = Array.isArray(user?.storeIds) ? [...user.storeIds] : [];
  editDraft.active = Boolean(user?.active ?? true);
  syncDraftScope(editDraft);
}

function toggleStoreSelection(draft, storeId) {
  const normalizedStoreId = String(storeId || "").trim();
  if (!normalizedStoreId) {
    return;
  }

  if (isSingleStoreScoped(draft.role)) {
    draft.storeIds = draft.storeIds.includes(normalizedStoreId) ? [] : [normalizedStoreId];
    return;
  }

  draft.storeIds = draft.storeIds.includes(normalizedStoreId)
    ? draft.storeIds.filter((id) => id !== normalizedStoreId)
    : [...draft.storeIds, normalizedStoreId];
}

function getStoreNames(storeIds = []) {
  const names = storeIds
    .map((storeId) => allStoreOptions.value.find((store) => store.id === storeId)?.name || "")
    .filter(Boolean);

  return names.length ? names.join(", ") : "-";
}

function getScopedStoreOptions(tenantId) {
  const normalizedTenantId = String(tenantId || "").trim();
  return allStoreOptions.value.filter((store) => !normalizedTenantId || store.tenantId === normalizedTenantId);
}

function getOnboardingLabel(user) {
  if (Boolean(user?.onboarding?.mustChangePassword)) return "Troca de senha pendente";
  const status = String(user?.onboarding?.status || "").trim();
  if (status === "ready") return "Pronto";
  if (status === "pending") return "Convite pendente";
  if (status === "expired") return "Convite expirado";
  if (status === "inactive") return "Conta inativa";
  return "Sem convite";
}

function getOnboardingTone(user) {
  if (Boolean(user?.onboarding?.mustChangePassword)) return "insight-tag insight-tag--warning";
  const status = String(user?.onboarding?.status || "").trim();
  if (status === "ready") return "insight-tag insight-tag--success";
  if (status === "pending") return "insight-tag insight-tag--warning";
  if (status === "expired") return "insight-tag";
  return "insight-tag";
}

async function presentInvitation(invitationPayload, successMessage) {
  const inviteUrl = String(invitationPayload?.inviteUrl || "").trim();

  if (!inviteUrl) {
    ui.success(successMessage);
    return;
  }

  if (import.meta.client && navigator?.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(inviteUrl);
      ui.success(`${successMessage} Link copiado.`);
      return;
    } catch {}
  }

  await ui.prompt({
    title: "Link de convite",
    message: "Copie o link abaixo para enviar ao usuario.",
    inputLabel: "Convite",
    initialValue: inviteUrl,
    confirmLabel: "Fechar"
  });
}

async function submitCreate() {
  if (createMode.value === "password" && !canManagePasswords.value) {
    ui.error("Somente o dev pode definir senha manualmente neste momento.");
    createMode.value = "invite";
    createDraft.password = "";
    return;
  }

  if (createMode.value === "password" && String(createDraft.password || "").trim().length < 8) {
    ui.error("Defina uma senha inicial com pelo menos 8 caracteres.");
    return;
  }

  const result = await usersStore.createUser({
    ...createDraft,
    password: createMode.value === "password" ? createDraft.password : ""
  });

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel criar usuario.");
    return;
  }

  const createdMode = createMode.value;
  resetCreateDraft();

  if (createdMode === "password" && !result?.invitation?.inviteUrl) {
    ui.success("Usuario criado com senha temporaria. No primeiro acesso ele precisara trocar a senha.");
    return;
  }

  await presentInvitation(result?.invitation, "Usuario criado e convidado.");
}

async function submitUpdate() {
  const result = await usersStore.updateUser(editingUserId.value, editDraft);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel atualizar usuario.");
    return;
  }

  if (result?.noChange) {
    ui.info("Nenhuma alteracao para salvar.");
    return;
  }

  editingUserId.value = "";
  ui.success("Usuario atualizado.");
}

async function resendInvite(user) {
  const result = await usersStore.inviteUser(user.id);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel gerar o convite.");
    return;
  }

  await presentInvitation(result?.invitation, "Novo convite gerado.");
}

async function archiveUser(user) {
  const { confirmed } = await ui.confirm({
    title: "Inativar usuario",
    message: `Deseja inativar ${user.displayName}?`,
    confirmLabel: "Inativar"
  });

  if (!confirmed) {
    return;
  }

  const result = await usersStore.archiveUser(user.id);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel inativar usuario.");
    return;
  }

  if (editingUserId.value === user.id) {
    editingUserId.value = "";
  }

  ui.success("Usuario inativado.");
}

async function resetPassword(user) {
  if (!canManagePasswords.value) {
    ui.error("Somente o dev pode resetar senha pelo painel neste momento.");
    return;
  }

  const { confirmed, value } = await ui.prompt({
    title: "Redefinir senha",
    message: `Defina uma senha temporaria para ${user.displayName}. No proximo acesso ele precisara trocar a senha.`,
    inputLabel: "Nova senha temporaria",
    inputPlaceholder: "Minimo de 8 caracteres",
    confirmLabel: "Salvar senha",
    initialValue: "",
    required: true
  });

  if (!confirmed) {
    return;
  }

  const nextPassword = String(value || "").trim();
  if (nextPassword.length < 8) {
    ui.error("Defina uma senha com pelo menos 8 caracteres.");
    return;
  }

  const result = await usersStore.resetPassword(user.id, nextPassword);
  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel redefinir a senha.");
    return;
  }

  ui.success("Senha temporaria redefinida.");
}

watch(
  roleCatalog,
  () => {
    if (!createDraft.role && roleCatalog.value.length) {
      createDraft.role = createRoleCatalog.value[0]?.id || roleCatalog.value[0].id;
    }

    if (!createDraft.tenantId) {
      createDraft.tenantId = auth.activeTenantId || tenantOptions.value[0]?.id || "";
    }

    syncDraftScope(createDraft);
  },
  { immediate: true }
);

watch(canManagePasswords, (allowed) => {
  if (allowed) {
    return;
  }

  if (createMode.value === "password") {
    createMode.value = "invite";
    createDraft.password = "";
  }
});

watch(() => createDraft.role, () => syncDraftScope(createDraft));
watch(() => editDraft.role, () => syncDraftScope(editDraft));

await usersStore.ensureLoaded();
</script>

<template>
  <article v-if="canEditUsers" class="settings-card" data-testid="multistore-users-card">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Usuarios e acessos</h3>
      <p class="settings-card__text">Controle de contas, papeis, escopo e onboarding por convite. Definicao e reset manual de senha ficam restritos ao dev por enquanto.</p>
    </header>

    <article v-if="usersStore.errorMessage" class="insight-card">
      <p class="settings-card__text">{{ usersStore.errorMessage }}</p>
    </article>

    <div class="insight-table-wrap">
      <table class="insight-table">
        <thead>
          <tr>
            <th>Usuario</th>
            <th>Email</th>
            <th>Papel</th>
            <th>Escopo</th>
            <th>Onboarding</th>
            <th>Status</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!usersStore.users.length">
            <td colspan="7">Nenhum usuario ativo para este tenant.</td>
          </tr>
          <template v-for="user in usersStore.users" :key="user.id">
            <tr>
              <td>{{ user.displayName }}</td>
              <td>{{ user.email }}</td>
              <td>{{ getRoleLabel(user.role) }}</td>
              <td>{{ user.role === "consultant" || user.role === "manager" || user.role === "store_terminal" ? getStoreNames(user.storeIds) : user.tenantId || "Plataforma" }}</td>
              <td>
                <span :class="getOnboardingTone(user)">{{ getOnboardingLabel(user) }}</span>
                <p v-if="isConsultantManaged(user)" class="settings-card__text">Gerenciado pela aba Consultores.</p>
              </td>
              <td>
                <span :class="user.active ? 'insight-tag insight-tag--success' : 'insight-tag'">
                  {{ user.active ? "Ativo" : "Inativo" }}
                </span>
              </td>
              <td>
                <button
                  v-if="user.active && user.onboarding?.status !== 'ready' && !isConsultantManaged(user)"
                  class="option-row__save"
                  type="button"
                  @click="resendInvite(user)"
                >
                  {{ user.onboarding?.status === "pending" ? "Copiar convite" : "Gerar convite" }}
                </button>
                <button v-if="canManagePasswords && user.active && user.onboarding?.hasPassword" class="option-row__save" type="button" @click="resetPassword(user)">Resetar senha</button>
                <button v-if="!isConsultantManaged(user)" class="option-row__save" type="button" @click="resetEditDraft(user)">Editar</button>
                <button v-if="!isConsultantManaged(user)" class="product-row__remove" type="button" @click="archiveUser(user)">Inativar</button>
              </td>
            </tr>
            <tr v-if="editingUserId === user.id && !isConsultantManaged(user)">
              <td colspan="7">
                <form class="multistore-form multistore-form--add" @submit.prevent="submitUpdate">
                  <div class="multistore-form__row">
                    <input v-model="editDraft.displayName" class="product-add__input" type="text" placeholder="Nome completo *">
                    <input v-model="editDraft.email" class="product-add__input" type="email" placeholder="Email *">
                    <AppSelectField
                      class="product-add__input"
                      :model-value="editDraft.role"
                      :options="genericRoleOptions"
                      placeholder="Selecionar papel"
                      @update:model-value="editDraft.role = $event"
                    />
                    <AppSelectField
                      v-if="isTenantScoped(editDraft.role) || isStoreScoped(editDraft.role)"
                      class="product-add__input"
                      :model-value="editDraft.tenantId"
                      :options="tenantSelectOptions"
                      placeholder="Selecionar tenant"
                      @update:model-value="editDraft.tenantId = $event"
                    />
                  </div>
                  <div v-if="isStoreScoped(editDraft.role)" class="multistore-user__scope-grid">
                    <label v-for="store in getScopedStoreOptions(editDraft.tenantId)" :key="store.id" class="settings-toggle">
                      <input
                        :checked="editDraft.storeIds.includes(store.id)"
                        type="checkbox"
                        @change="toggleStoreSelection(editDraft, store.id)"
                      >
                      <span>{{ store.name }}</span>
                    </label>
                  </div>
                  <p v-if="isSingleStoreScoped(editDraft.role)" class="settings-card__text">Esse papel fica vinculado a uma unica loja.</p>
                  <div class="multistore-form__actions">
                    <label class="settings-toggle">
                      <input v-model="editDraft.active" type="checkbox">
                      <span>Conta ativa</span>
                    </label>
                    <button class="option-row__save" type="submit">Salvar usuario</button>
                    <button class="product-row__remove" type="button" @click="editingUserId = ''">Cancelar</button>
                  </div>
                </form>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <form class="multistore-form multistore-form--add" @submit.prevent="submitCreate">
      <div class="multistore-form__mode-switch" data-testid="users-create-mode">
        <button
          class="option-row__save"
          :class="{ 'is-active': createMode === 'invite' }"
          type="button"
          @click="createMode = 'invite'; createDraft.password = ''"
        >
          Enviar convite
        </button>
        <button
          v-if="canManagePasswords"
          class="option-row__save"
          :class="{ 'is-active': createMode === 'password' }"
          type="button"
          @click="createMode = 'password'"
        >
          Definir senha
        </button>
      </div>
      <div class="multistore-form__row">
        <input v-model="createDraft.displayName" class="product-add__input" type="text" placeholder="Nome completo *">
        <input v-model="createDraft.email" class="product-add__input" type="email" placeholder="Email *">
        <input
          v-if="canManagePasswords && createMode === 'password'"
          v-model="createDraft.password"
          class="product-add__input"
          type="password"
          placeholder="Senha inicial *"
        >
        <AppSelectField
          class="product-add__input"
          :model-value="createDraft.role"
          :options="createRoleOptions"
          placeholder="Selecionar papel"
          @update:model-value="createDraft.role = $event"
        />
        <AppSelectField
          v-if="isTenantScoped(createDraft.role) || isStoreScoped(createDraft.role)"
          class="product-add__input"
          :model-value="createDraft.tenantId"
          :options="tenantSelectOptions"
          placeholder="Selecionar tenant"
          @update:model-value="createDraft.tenantId = $event"
        />
      </div>
      <div v-if="isStoreScoped(createDraft.role)" class="multistore-user__scope-grid">
        <label v-for="store in getScopedStoreOptions(createDraft.tenantId)" :key="store.id" class="settings-toggle">
          <input
            :checked="createDraft.storeIds.includes(store.id)"
            type="checkbox"
            @change="toggleStoreSelection(createDraft, store.id)"
          >
          <span>{{ store.name }}</span>
        </label>
      </div>
      <p v-if="isSingleStoreScoped(createDraft.role)" class="settings-card__text">Esse papel fica vinculado a uma unica loja.</p>
      <div class="multistore-form__actions">
        <label class="settings-toggle">
          <input v-model="createDraft.active" type="checkbox">
          <span>Criar conta ativa</span>
        </label>
        <button class="product-add__button" type="submit">
          {{ canManagePasswords && createMode === "password" ? "Criar acesso" : "Enviar convite" }}
        </button>
      </div>
      <p class="settings-card__text">Consultores devem ser criados na gestao de consultores para nascerem com roster e login vinculados.</p>
      <p v-if="!canManagePasswords" class="settings-card__text">Para perfis nao-dev, o onboarding segue somente por convite.</p>
    </form>
  </article>
</template>
