<script setup>
import { computed, reactive, ref, watch } from "vue";
import { Archive, Info, KeyRound, Mail, Plus, RotateCcw, X } from "lucide-vue-next";

import AppDetailDialog from "~/components/ui/AppDetailDialog.vue";
import AppEntityGrid from "~/components/ui/AppEntityGrid.vue";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import AppToggleSwitch from "~/components/ui/AppToggleSwitch.vue";
import { canManageUserPasswords } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { useUiStore } from "~/stores/ui";
import { useUsersStore } from "~/stores/users";

const ALL_STORES_VALUE = "all";

const ROLE_LABELS = {
  consultant: "Consultor",
  manager: "Gerente",
  marketing: "Marketing",
  director: "Diretor",
  owner: "Gestao geral",
  platform_admin: "Admin sistema",
  store_terminal: "Usuario de loja"
};

const auth = useAuthStore();
const ui = useUiStore();
const usersStore = useUsersStore();

const createComposerOpen = ref(false);
const createMode = ref("invite");
const selectedDetailUser = ref(null);
const rowDrafts = ref({});
const rowBusy = reactive({});

const filters = reactive({
  search: "",
  status: "active",
  role: "",
  store: "",
  tenant: ""
});

const createDraft = reactive({
  displayName: "",
  email: "",
  employeeCode: "",
  role: "manager",
  tenantId: "",
  storeId: "",
  active: true,
  password: ""
});

const canManagePasswords = computed(() => canManageUserPasswords(auth.role));
const canOverrideConsultantManaged = computed(() => auth.role === "platform_admin");
const storeLookup = computed(() => new Map((auth.storeContext || []).map((store) => [String(store.id || "").trim(), store])));
const tenantLookup = computed(() => new Map((auth.tenantContext || []).map((tenant) => [String(tenant.id || "").trim(), tenant])));
const clientFilterOptions = computed(() => [
  { value: "", label: "Cliente" },
  ...(auth.tenantContext || []).map((tenant) => ({
    value: String(tenant.id || "").trim(),
    label: String(tenant.name || "").trim()
  }))
]);
const statusFilterOptions = [
  { value: "active", label: "Status: ativos" },
  { value: "inactive", label: "Status: inativos" },
  { value: "", label: "Status: todos" }
];

const createRoleOptions = computed(() =>
  usersStore.assignableRoles
    .filter((role) => role.id !== "consultant")
    .map((role) => ({
      value: role.id,
      label: getRoleLabel(role.id)
    }))
);

const filterRoleOptions = computed(() => {
  const seen = new Set();
  const options = [{ value: "", label: "Perfil" }];

  for (const user of usersStore.users) {
    const roleId = normalizeText(user.role);
    if (!roleId || seen.has(roleId)) {
      continue;
    }

    seen.add(roleId);
    options.push({ value: roleId, label: getRoleLabel(roleId) });
  }

  return options;
});

const storeFilterOptions = computed(() => [
  { value: "", label: "Loja" },
  { value: ALL_STORES_VALUE, label: "ALL" },
  ...(auth.storeContext || []).map((store) => ({
    value: String(store.id || "").trim(),
    label: String(store.name || "").trim()
  }))
]);

const gridColumns = computed(() => [
  { id: "name", label: "Nome", width: "1.55fr", locked: true },
  { id: "nick", label: "Nick", width: "0.78fr" },
  { id: "email", label: "Email", width: "1.35fr" },
  { id: "status", label: "Status", width: "0.68fr", align: "center" },
  { id: "profile", label: "Perfil", width: "0.92fr" },
  { id: "store", label: "Loja", width: "0.96fr" },
  { id: "employeeCode", label: "Matricula", width: "0.72fr", align: "center" },
  { id: "onboarding", label: "Acesso", width: "0.9fr" },
  { id: "actions", label: "Opcoes", width: "0.76fr", locked: true, align: "end" }
]);

const filteredUsers = computed(() => {
  return [...usersStore.users]
    .filter((user) => {
      const role = normalizeText(user.role);
      const tenantId = normalizeText(user.tenantId);
      const searchHaystack = normalizeSearch([
        user.displayName,
        user.email,
        user.employeeCode,
        user.jobTitle,
        buildNickname(user.displayName),
        getStoreLabel(user),
        getRoleLabel(role)
      ].join(" "));

      if (filters.search && !searchHaystack.includes(normalizeSearch(filters.search))) {
        return false;
      }

      if (filters.status === "active" && !user.active) {
        return false;
      }

      if (filters.status === "inactive" && user.active) {
        return false;
      }

      if (filters.role && role !== filters.role) {
        return false;
      }

      if (filters.tenant && tenantId !== filters.tenant) {
        return false;
      }

      if (filters.store === ALL_STORES_VALUE) {
        return !isStoreScopedRole(role);
      }

      if (filters.store) {
        return Array.isArray(user.storeIds) && user.storeIds.includes(filters.store);
      }

      return true;
    })
    .sort((left, right) => left.displayName.localeCompare(right.displayName, "pt-BR"));
});

const detailSections = computed(() => {
  const user = selectedDetailUser.value;
  if (!user) {
    return [];
  }

  return [
    {
      id: "identity",
      title: "Identidade",
      fields: [
        { label: "Nome", value: user.displayName },
        { label: "Nick", value: buildNickname(user.displayName) },
        { label: "Email", value: user.email },
        { label: "Matricula", value: user.employeeCode || "-" }
      ]
    },
    {
      id: "access",
      title: "Acesso",
      fields: [
        { label: "Perfil", value: getRoleLabel(user.role) },
        { label: "Loja", value: getStoreLabel(user) },
        { label: "Status", value: user.active ? "Ativo" : "Inativo" },
        { label: "Onboarding", value: getOnboardingLabel(user) },
        { label: "Cargo", value: user.jobTitle || "-" }
      ]
    },
    {
      id: "meta",
      title: "Sistema",
      fields: [
        { label: "User ID", value: user.id },
        { label: "Tenant", value: tenantLookup.value.get(normalizeText(user.tenantId))?.name || user.tenantId || "Plataforma" },
        { label: "Origem", value: isConsultantManaged(user) ? "Consultores" : "Usuarios" }
      ]
    }
  ];
});

function normalizeText(value) {
  return String(value || "").trim();
}

function normalizeSearch(value) {
  return normalizeText(value)
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase();
}

function getRoleLabel(role) {
  const roleId = normalizeText(role);
  return ROLE_LABELS[roleId] || roleId || "Sem papel";
}

function isStoreScopedRole(role) {
  const normalizedRole = normalizeText(role);
  return normalizedRole === "consultant" || normalizedRole === "manager" || normalizedRole === "store_terminal";
}

function isConsultantManaged(user) {
  return normalizeText(user?.managedBy) === "consultants" || normalizeText(user?.role) === "consultant";
}

function isRosterLocked(user) {
  return isConsultantManaged(user) && !canOverrideConsultantManaged.value;
}

function buildNickname(displayName) {
  const parts = normalizeText(displayName).split(/\s+/).filter(Boolean);
  if (!parts.length) {
    return "-";
  }

  const first = parts[0];
  const second = parts.length > 1 ? parts[1] : "";
  const nickname = second ? `${first} ${second.charAt(0).toUpperCase()}.` : first;
  return nickname.length > 18 ? `${first.slice(0, 16)}...` : nickname;
}

function getStoreName(storeId) {
  return storeLookup.value.get(normalizeText(storeId))?.name || normalizeText(storeId) || "-";
}

function getStoreLabel(user) {
  if (!isStoreScopedRole(user.role)) {
    return "ALL";
  }

  const names = (Array.isArray(user.storeIds) ? user.storeIds : []).map((storeId) => getStoreName(storeId)).filter(Boolean);
  return names.join(", ") || "Loja nao vinculada";
}

function getOnboardingLabel(user) {
  if (!user.active) {
    return "Conta inativa";
  }

  if (user.onboarding?.mustChangePassword) {
    return "Troca pendente";
  }

  switch (normalizeText(user.onboarding?.status)) {
    case "ready":
      return "Pronto";
    case "pending":
      return "Convite pendente";
    case "expired":
      return "Convite expirado";
    case "inactive":
      return "Conta inativa";
    default:
      return "Sem convite";
  }
}

function getOnboardingTone(user) {
  if (user.onboarding?.mustChangePassword) {
    return "users-access-manager__pill users-access-manager__pill--warning";
  }

  if (normalizeText(user.onboarding?.status) === "ready") {
    return "users-access-manager__pill users-access-manager__pill--success";
  }

  if (normalizeText(user.onboarding?.status) === "pending") {
    return "users-access-manager__pill users-access-manager__pill--info";
  }

  return "users-access-manager__pill";
}

function createRowDraft(user) {
  return {
    displayName: normalizeText(user.displayName),
    email: normalizeText(user.email),
    employeeCode: normalizeText(user.employeeCode),
    role: normalizeText(user.role),
    storeId: isStoreScopedRole(user.role) ? normalizeText(user.storeIds?.[0]) : ALL_STORES_VALUE,
    active: Boolean(user.active)
  };
}

function getRowDraft(user) {
  if (!rowDrafts.value[user.id]) {
    rowDrafts.value[user.id] = createRowDraft(user);
  }

  return rowDrafts.value[user.id];
}

function resetRowDraft(user) {
  rowDrafts.value[user.id] = createRowDraft(user);
}

function resetCreateDraft() {
  createDraft.displayName = "";
  createDraft.email = "";
  createDraft.employeeCode = "";
  createDraft.password = "";
  createDraft.role = createRoleOptions.value[0]?.value || "manager";
  createDraft.tenantId = normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id);
  createDraft.storeId = normalizeText(auth.storeContext?.[0]?.id);
  createDraft.active = true;

  syncCreateScope();
}

function syncCreateScope() {
  if (isStoreScopedRole(createDraft.role)) {
    const scopedStores = getScopedStoreOptions(createDraft.tenantId);
    if (!scopedStores.some((option) => option.value === createDraft.storeId)) {
      createDraft.storeId = scopedStores[0]?.value || "";
    }
    return;
  }

  createDraft.storeId = ALL_STORES_VALUE;
}

function getScopedStoreOptions(tenantId) {
  const normalizedTenantId = normalizeText(tenantId);
  return (auth.storeContext || [])
    .filter((store) => !normalizedTenantId || normalizeText(store.tenantId) === normalizedTenantId)
    .map((store) => ({
      value: normalizeText(store.id),
      label: normalizeText(store.name)
    }));
}

function getRoleSelectOptions(user) {
  if (isRosterLocked(user)) {
    return [{ value: normalizeText(user.role), label: getRoleLabel(user.role) }];
  }

  return createRoleOptions.value;
}

function getStoreSelectOptions(user, draft) {
  const role = normalizeText(draft?.role || user?.role);
  if (!isStoreScopedRole(role)) {
    return [{ value: ALL_STORES_VALUE, label: "ALL" }];
  }

  return getScopedStoreOptions(user?.tenantId || auth.activeTenantId);
}

function clearFilters() {
  filters.search = "";
  filters.status = "active";
  filters.role = "";
  filters.store = "";
  filters.tenant = "";
}

async function presentInvitation(invitationPayload, successMessage) {
  const inviteUrl = normalizeText(invitationPayload?.inviteUrl);
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

function withRowBusy(userId, callback) {
  if (rowBusy[userId]) {
    return Promise.resolve();
  }

  rowBusy[userId] = true;
  return Promise.resolve(callback()).finally(() => {
    rowBusy[userId] = false;
  });
}

function buildUpdatePayload(user) {
  const draft = getRowDraft(user);
  return {
    displayName: normalizeText(draft.displayName),
    email: normalizeText(draft.email),
    employeeCode: normalizeText(draft.employeeCode),
    role: normalizeText(draft.role),
    tenantId: normalizeText(user.tenantId || auth.activeTenantId),
    storeIds: isStoreScopedRole(draft.role) ? [normalizeText(draft.storeId)].filter(Boolean) : [],
    active: Boolean(draft.active)
  };
}

async function saveRow(user, { silent = true } = {}) {
  if (isRosterLocked(user)) {
    ui.info("Esse consultor continua gerenciado pelo roster por enquanto.");
    resetRowDraft(user);
    return;
  }

  const payload = buildUpdatePayload(user);
  if (!payload.displayName || !payload.email) {
    ui.error("Nome e email sao obrigatorios.");
    resetRowDraft(user);
    return;
  }

  if (isStoreScopedRole(payload.role) && payload.storeIds.length === 0) {
    ui.error("Selecione uma loja valida para este perfil.");
    resetRowDraft(user);
    return;
  }

  await withRowBusy(user.id, async () => {
    const result = await usersStore.updateUser(user.id, payload);
    if (result?.ok === false) {
      ui.error(result.message || "Nao foi possivel atualizar o acesso.");
      resetRowDraft(user);
      return;
    }

    if (!silent && !result?.noChange) {
      ui.success("Acesso atualizado.");
    }
  });
}

async function handleInlineBlur(user) {
  await saveRow(user);
}

async function handleStatusChange(user, nextValue) {
  const draft = getRowDraft(user);
  draft.active = nextValue;
  await saveRow(user);
}

async function handleRoleChange(user, nextRole) {
  const draft = getRowDraft(user);
  draft.role = normalizeText(nextRole);
  if (!isStoreScopedRole(draft.role)) {
    draft.storeId = ALL_STORES_VALUE;
  } else if (!draft.storeId || draft.storeId === ALL_STORES_VALUE) {
    draft.storeId = getStoreSelectOptions(user, draft)[0]?.value || "";
  }

  await saveRow(user);
}

async function handleStoreChange(user, nextStoreId) {
  const draft = getRowDraft(user);
  draft.storeId = normalizeText(nextStoreId);
  await saveRow(user);
}

async function handleArchiveAction(user) {
  if (isRosterLocked(user)) {
    ui.info("Arquive consultores pelo fluxo de roster enquanto o atalho unificado nao entra.");
    return;
  }

  if (user.active) {
    const { confirmed } = await ui.confirm({
      title: "Inativar acesso",
      message: `Deseja inativar ${user.displayName}?`,
      confirmLabel: "Inativar"
    });

    if (!confirmed) {
      return;
    }
  }

  const draft = getRowDraft(user);
  draft.active = !user.active;
  await saveRow(user, { silent: false });
}

async function handleInviteAction(user) {
  const result = await usersStore.inviteUser(user.id);
  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel gerar o convite.");
    return;
  }

  await presentInvitation(result?.invitation, "Convite gerado.");
}

async function handleResetPassword(user) {
  const { confirmed, value } = await ui.prompt({
    title: "Redefinir senha",
    message: `Defina uma senha temporaria para ${user.displayName}.`,
    inputLabel: "Nova senha temporaria",
    inputPlaceholder: "Minimo de 8 caracteres",
    confirmLabel: "Salvar senha",
    required: true
  });

  if (!confirmed) {
    return;
  }

  const nextPassword = normalizeText(value);
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

async function submitCreate() {
  if (!normalizeText(createDraft.displayName) || !normalizeText(createDraft.email)) {
    ui.error("Nome e email sao obrigatorios.");
    return;
  }

  if (createMode.value === "password" && normalizeText(createDraft.password).length < 8) {
    ui.error("Defina uma senha inicial com pelo menos 8 caracteres.");
    return;
  }

  if (isStoreScopedRole(createDraft.role) && !normalizeText(createDraft.storeId)) {
    ui.error("Selecione uma loja para este novo acesso.");
    return;
  }

  const result = await usersStore.createUser({
    displayName: createDraft.displayName,
    email: createDraft.email,
    employeeCode: createDraft.employeeCode,
    password: createMode.value === "password" ? createDraft.password : "",
    role: createDraft.role,
    tenantId: createDraft.role === "platform_admin" ? "" : createDraft.tenantId,
    storeIds: isStoreScopedRole(createDraft.role) ? [createDraft.storeId].filter(Boolean) : [],
    active: createDraft.active
  });

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel criar o acesso.");
    return;
  }

  const createdMode = createMode.value;
  resetCreateDraft();
  createComposerOpen.value = false;

  if (createdMode === "password") {
    ui.success("Usuario criado com senha temporaria.");
    return;
  }

  await presentInvitation(result?.invitation, "Usuario criado e convidado.");
}

function openDetails(user) {
  selectedDetailUser.value = user;
}

function closeDetails() {
  selectedDetailUser.value = null;
}

function canShowInviteAction(user) {
  if (isRosterLocked(user)) {
    return false;
  }

  return user.active && normalizeText(user.onboarding?.status) !== "ready";
}

watch(
  () => usersStore.users,
  (users) => {
    const nextDrafts = {};
    for (const user of users) {
      nextDrafts[user.id] = createRowDraft(user);
    }
    rowDrafts.value = nextDrafts;
  },
  { immediate: true, deep: true }
);

watch(
  () => createDraft.role,
  () => {
    syncCreateScope();
  }
);

watch(
  () => createDraft.tenantId,
  () => {
    syncCreateScope();
  }
);

await usersStore.ensureLoaded();
resetCreateDraft();
</script>

<template>
  <section class="users-access-manager">
    <div class="users-access-manager__launcher-row">
      <button class="users-access-manager__launcher" type="button" @click="createComposerOpen = !createComposerOpen">
        <Plus class="users-access-manager__button-icon" :size="16" :stroke-width="2.15" />
        <span>{{ createComposerOpen ? "Fechar cadastro" : "Novo cadastro" }}</span>
      </button>

      <p class="users-access-manager__launcher-hint">
        Grade compacta para acessos, pronta para reaproveitar o mesmo padrao nas outras areas.
      </p>
    </div>

    <transition name="users-access-manager-fade">
      <form v-if="createComposerOpen" class="users-access-manager__create-card" @submit.prevent="submitCreate">
        <header class="users-access-manager__create-header">
          <div>
            <h3>Novo acesso</h3>
            <p>Abra o cadastro via convite ou defina senha inicial quando o perfil permitir.</p>
          </div>

          <button class="users-access-manager__close-btn" type="button" @click="createComposerOpen = false">
            <X :size="16" :stroke-width="2.15" />
          </button>
        </header>

        <div class="users-access-manager__mode-switch">
          <button
            class="users-access-manager__mode-btn"
            :class="{ 'is-active': createMode === 'invite' }"
            type="button"
            @click="createMode = 'invite'; createDraft.password = ''"
          >
            Convite
          </button>

          <button
            v-if="canManagePasswords"
            class="users-access-manager__mode-btn"
            :class="{ 'is-active': createMode === 'password' }"
            type="button"
            @click="createMode = 'password'"
          >
            Senha inicial
          </button>
        </div>

        <div class="users-access-manager__create-grid">
          <input v-model="createDraft.displayName" class="users-access-manager__field" type="text" placeholder="Nome completo *">
          <input v-model="createDraft.email" class="users-access-manager__field" type="email" placeholder="Email *">
          <input v-model="createDraft.employeeCode" class="users-access-manager__field" type="text" placeholder="Matricula">

          <input
            v-if="canManagePasswords && createMode === 'password'"
            v-model="createDraft.password"
            class="users-access-manager__field"
            type="password"
            placeholder="Senha inicial *"
          >

          <AppSelectField
            class="users-access-manager__select"
            :model-value="createDraft.role"
            :options="createRoleOptions"
            :show-leading-icon="false"
            placeholder="Perfil"
            @update:model-value="createDraft.role = $event"
          />

          <AppSelectField
            v-if="auth.role === 'platform_admin'"
            class="users-access-manager__select"
            :model-value="createDraft.tenantId"
            :options="clientFilterOptions.filter((option) => option.value)"
            :show-leading-icon="false"
            placeholder="Cliente"
            @update:model-value="createDraft.tenantId = $event"
          />

          <AppSelectField
            class="users-access-manager__select"
            :model-value="createDraft.storeId"
            :options="isStoreScopedRole(createDraft.role) ? getScopedStoreOptions(createDraft.tenantId) : [{ value: ALL_STORES_VALUE, label: 'ALL' }]"
            :show-leading-icon="false"
            :disabled="!isStoreScopedRole(createDraft.role)"
            placeholder="Loja"
            @update:model-value="createDraft.storeId = $event"
          />
        </div>

        <div class="users-access-manager__create-actions">
          <label class="users-access-manager__checkbox">
            <input v-model="createDraft.active" type="checkbox">
            <span>Criar conta ativa</span>
          </label>

          <button class="users-access-manager__submit-btn" type="submit">
            {{ canManagePasswords && createMode === 'password' ? 'Criar acesso' : 'Enviar convite' }}
          </button>
        </div>

        <p class="users-access-manager__hint">
          Consultores seguem vinculados ao roster. Quando necessario, o admin da plataforma pode corrigir nome, loja, perfil e matricula por aqui.
        </p>
      </form>
    </transition>

    <AppEntityGrid
      :columns="gridColumns"
      :rows="filteredUsers"
      :loading="usersStore.pending"
      :search-value="filters.search"
      :storage-key="'users-access-grid-columns-v1'"
      empty-title="Nenhum usuario encontrado"
      empty-text="Ajuste os filtros ou abra um novo cadastro para preencher a grade."
      testid="users-access-grid"
      @update:search-value="filters.search = $event"
    >
      <template #toolbar-filters>
        <AppSelectField
          class="users-access-manager__toolbar-select"
          :model-value="filters.status"
          :options="statusFilterOptions"
          :show-leading-icon="false"
          compact
          @update:model-value="filters.status = $event"
        />

        <AppSelectField
          class="users-access-manager__toolbar-select"
          :model-value="filters.role"
          :options="filterRoleOptions"
          :show-leading-icon="false"
          compact
          @update:model-value="filters.role = $event"
        />

        <AppSelectField
          v-if="auth.role === 'platform_admin'"
          class="users-access-manager__toolbar-select"
          :model-value="filters.tenant"
          :options="clientFilterOptions"
          :show-leading-icon="false"
          compact
          @update:model-value="filters.tenant = $event"
        />

        <AppSelectField
          class="users-access-manager__toolbar-select"
          :model-value="filters.store"
          :options="storeFilterOptions"
          :show-leading-icon="false"
          compact
          @update:model-value="filters.store = $event"
        />
      </template>

      <template #toolbar-actions>
        <span class="users-access-manager__counter">{{ filteredUsers.length }} registros</span>
        <button class="users-access-manager__ghost-btn" type="button" @click="clearFilters">Limpar</button>
      </template>

      <template #cell-name="{ row }">
        <div class="users-access-manager__identity-cell">
          <input
            v-if="!isRosterLocked(row)"
            v-model="getRowDraft(row).displayName"
            class="users-access-manager__inline-input"
            type="text"
            @blur="handleInlineBlur(row)"
            @keydown.enter.prevent="$event.target.blur()"
          >
          <div v-else class="users-access-manager__locked-copy">
            <strong>{{ row.displayName }}</strong>
            <small>Gerenciado pelo roster</small>
          </div>
          <small class="users-access-manager__subcopy">{{ row.jobTitle || getRoleLabel(row.role) }}</small>
        </div>
      </template>

      <template #cell-nick="{ row }">
        <span class="users-access-manager__nick-chip">{{ buildNickname(row.displayName) }}</span>
      </template>

      <template #cell-email="{ row }">
        <input
          v-if="!isRosterLocked(row)"
          v-model="getRowDraft(row).email"
          class="users-access-manager__inline-input"
          type="email"
          @blur="handleInlineBlur(row)"
          @keydown.enter.prevent="$event.target.blur()"
        >
        <span v-else class="users-access-manager__static-copy">{{ row.email }}</span>
      </template>

      <template #cell-status="{ row }">
        <AppToggleSwitch
          compact
          :model-value="getRowDraft(row).active"
          :disabled="rowBusy[row.id] || isRosterLocked(row)"
          @change="handleStatusChange(row, $event)"
        />
      </template>

      <template #cell-profile="{ row }">
        <AppSelectField
          class="users-access-manager__inline-select"
          :model-value="getRowDraft(row).role"
          :options="getRoleSelectOptions(row)"
          :show-leading-icon="false"
          compact
          :disabled="rowBusy[row.id] || isRosterLocked(row)"
          @update:model-value="handleRoleChange(row, $event)"
        />
      </template>

      <template #cell-store="{ row }">
        <AppSelectField
          class="users-access-manager__inline-select"
          :model-value="getRowDraft(row).storeId"
          :options="getStoreSelectOptions(row, getRowDraft(row))"
          :show-leading-icon="false"
          compact
          :disabled="rowBusy[row.id] || isRosterLocked(row)"
          @update:model-value="handleStoreChange(row, $event)"
        />
      </template>

      <template #cell-employeeCode="{ row }">
        <input
          v-if="!isRosterLocked(row)"
          v-model="getRowDraft(row).employeeCode"
          class="users-access-manager__inline-input users-access-manager__inline-input--compact"
          type="text"
          placeholder="-"
          @blur="handleInlineBlur(row)"
          @keydown.enter.prevent="$event.target.blur()"
        >
        <span v-else class="users-access-manager__static-copy">{{ row.employeeCode || "-" }}</span>
      </template>

      <template #cell-onboarding="{ row }">
        <span :class="getOnboardingTone(row)">{{ getOnboardingLabel(row) }}</span>
      </template>

      <template #cell-actions="{ row }">
        <div class="users-access-manager__actions">
          <button class="users-access-manager__icon-btn" type="button" title="Ver detalhes" @click="openDetails(row)">
            <Info :size="15" :stroke-width="2.15" />
          </button>

          <button
            v-if="canShowInviteAction(row)"
            class="users-access-manager__icon-btn"
            type="button"
            :title="normalizeText(row.onboarding?.status) === 'pending' ? 'Copiar convite' : 'Gerar convite'"
            @click="handleInviteAction(row)"
          >
            <Mail :size="15" :stroke-width="2.15" />
          </button>

          <button
            v-if="canManagePasswords && row.onboarding?.hasPassword"
            class="users-access-manager__icon-btn"
            type="button"
            title="Resetar senha"
            @click="handleResetPassword(row)"
          >
            <KeyRound :size="15" :stroke-width="2.15" />
          </button>

          <button
            class="users-access-manager__icon-btn"
            type="button"
            :title="row.active ? 'Inativar' : 'Reativar'"
            @click="handleArchiveAction(row)"
          >
            <Archive v-if="row.active" :size="15" :stroke-width="2.15" />
            <RotateCcw v-else :size="15" :stroke-width="2.15" />
          </button>
        </div>
      </template>
    </AppEntityGrid>

    <AppDetailDialog
      :model-value="Boolean(selectedDetailUser)"
      :title="selectedDetailUser?.displayName || 'Detalhes do acesso'"
      :subtitle="selectedDetailUser?.email || ''"
      :sections="detailSections"
      @update:model-value="!$event && closeDetails()"
    />
  </section>
</template>

<style scoped>
.users-access-manager {
  display: grid;
  gap: 0.85rem;
}

.users-access-manager__launcher-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.8rem;
  flex-wrap: wrap;
}

.users-access-manager__launcher {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-height: 2.35rem;
  padding: 0 0.9rem;
  border: 1px solid rgba(34, 197, 94, 0.36);
  border-radius: 999px;
  background: rgba(34, 197, 94, 0.16);
  color: #dcfce7;
  font-weight: 700;
  font-size: 0.76rem;
  cursor: pointer;
}

.users-access-manager__button-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}

.users-access-manager__launcher-hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.users-access-manager__create-card {
  display: grid;
  gap: 0.85rem;
  padding: 0.85rem;
  border-radius: 1rem;
  border: 1px solid var(--line-soft);
  background: rgba(13, 18, 29, 0.92);
  box-shadow: var(--shadow-card);
}

.users-access-manager__create-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.8rem;
}

.users-access-manager__create-header h3 {
  margin: 0;
  color: #ffffff;
  font-size: 0.95rem;
}

.users-access-manager__create-header p {
  margin: 0.2rem 0 0;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.users-access-manager__close-btn {
  width: 2.1rem;
  height: 2.1rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-main);
  cursor: pointer;
}

.users-access-manager__mode-switch {
  display: inline-flex;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.users-access-manager__mode-btn,
.users-access-manager__ghost-btn,
.users-access-manager__submit-btn {
  min-height: 2.25rem;
  padding: 0 0.82rem;
  border-radius: 999px;
  border: 1px solid rgba(129, 140, 248, 0.18);
  background: rgba(18, 25, 38, 0.9);
  color: var(--text-main);
  font-weight: 700;
  font-size: 0.76rem;
  cursor: pointer;
}

.users-access-manager__mode-btn.is-active {
  border-color: rgba(129, 140, 248, 0.42);
  background: rgba(99, 102, 241, 0.18);
}

.users-access-manager__submit-btn {
  border-color: rgba(34, 197, 94, 0.32);
  background: rgba(34, 197, 94, 0.16);
  color: #dcfce7;
}

.users-access-manager__create-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.65rem;
}

.users-access-manager__field,
.users-access-manager__inline-input {
  width: 100%;
  min-height: 2.4rem;
  box-sizing: border-box;
  border-radius: 0.8rem;
  border: 1px solid rgba(129, 140, 248, 0.14);
  background: rgba(18, 25, 38, 0.95);
  color: var(--text-main);
  padding: 0 0.75rem;
  font-size: 0.82rem;
  outline: none;
}

.users-access-manager__field:focus,
.users-access-manager__inline-input:focus {
  border-color: rgba(129, 140, 248, 0.36);
  box-shadow: 0 0 0 3px rgba(129, 140, 248, 0.12);
}

.users-access-manager__inline-input {
  min-height: 2.15rem;
}

.users-access-manager__inline-input--compact {
  min-height: 2rem;
}

.users-access-manager__select,
.users-access-manager__toolbar-select,
.users-access-manager__inline-select {
  min-width: 0;
}

.users-access-manager__toolbar-select {
  min-width: 8.25rem;
}

.users-access-manager__create-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.users-access-manager__checkbox {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.users-access-manager__checkbox input {
  accent-color: var(--accent-focus);
}

.users-access-manager__hint,
.users-access-manager__subcopy {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.72rem;
}

.users-access-manager__counter {
  display: inline-flex;
  align-items: center;
  min-height: 2.2rem;
  padding: 0 0.78rem;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(18, 25, 38, 0.82);
  color: var(--text-muted);
  font-size: 0.74rem;
}

.users-access-manager__identity-cell,
.users-access-manager__locked-copy {
  width: 100%;
  display: grid;
  gap: 0.24rem;
}

.users-access-manager__locked-copy strong,
.users-access-manager__static-copy {
  color: var(--text-main);
  font-size: 0.82rem;
}

.users-access-manager__locked-copy small {
  color: var(--text-muted);
  font-size: 0.7rem;
}

.users-access-manager__nick-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.72rem;
  padding: 0 0.62rem;
  border-radius: 999px;
  background: rgba(129, 140, 248, 0.14);
  color: #dbe4ff;
  font-weight: 700;
  font-size: 0.72rem;
}

.users-access-manager__pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.72rem;
  padding: 0 0.68rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.users-access-manager__pill--success {
  background: rgba(34, 197, 94, 0.14);
  color: #86efac;
}

.users-access-manager__pill--warning {
  background: rgba(251, 191, 36, 0.14);
  color: #fde68a;
}

.users-access-manager__pill--info {
  background: rgba(56, 189, 248, 0.14);
  color: #bae6fd;
}

.users-access-manager__actions {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.32rem;
}

.users-access-manager__icon-btn {
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.72rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(18, 25, 38, 0.92);
  color: var(--text-main);
  cursor: pointer;
}

.users-access-manager__icon-btn svg,
.users-access-manager__close-btn svg {
  width: 15px;
  height: 15px;
  flex-shrink: 0;
}

.users-access-manager__icon-btn:hover {
  border-color: rgba(129, 140, 248, 0.34);
  color: #ffffff;
}

.users-access-manager-fade-enter-active,
.users-access-manager-fade-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.users-access-manager-fade-enter-from,
.users-access-manager-fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

@media (max-width: 1180px) {
  .users-access-manager__create-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .users-access-manager__create-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .users-access-manager__create-actions,
  .users-access-manager__launcher-row {
    align-items: stretch;
  }

  .users-access-manager__submit-btn,
  .users-access-manager__ghost-btn,
  .users-access-manager__launcher {
    width: 100%;
    justify-content: center;
  }
}
</style>