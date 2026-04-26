<script setup>
import { computed, reactive, ref, watch } from "vue";
import { storeToRefs } from "pinia";

import AppEntityGrid from "~/components/ui/AppEntityGrid.vue";
import { useAuthStore } from "~/stores/auth";
import { useTenantsStore } from "~/stores/tenants";
import { useUiStore } from "~/stores/ui";

const auth = useAuthStore();
const ui = useUiStore();
const tenantsStore = useTenantsStore();

const { tenants, pending, errorMessage, canCreate, manageable } = storeToRefs(tenantsStore);

const searchValue = ref("");
const statusFilter = ref("all");
const selectedTenantId = ref("");
const createSaving = ref(false);
const detailSaving = ref(false);
const actionTenantId = ref("");

const createDraft = reactive({
  name: "",
  slug: "",
  active: true
});

const detailDraft = reactive({
  name: "",
  slug: "",
  active: true
});

const columns = [
  { id: "name", label: "Cliente", width: "minmax(220px, 1.8fr)", locked: true },
  { id: "slug", label: "Slug", width: "minmax(140px, 1fr)" },
  { id: "status", label: "Status", width: "minmax(108px, 0.7fr)", align: "center" },
  { id: "actions", label: "Abrir", width: "minmax(90px, 0.6fr)", align: "end", locked: true }
];

const summary = computed(() => ({
  total: tenants.value.length,
  active: tenants.value.filter((tenant) => tenant.active).length,
  inactive: tenants.value.filter((tenant) => !tenant.active).length
}));

const filteredRows = computed(() => {
  const search = String(searchValue.value || "").trim().toLowerCase();

  return tenants.value
    .filter((tenant) => {
      if (statusFilter.value === "active") {
        return tenant.active;
      }

      if (statusFilter.value === "inactive") {
        return !tenant.active;
      }

      return true;
    })
    .filter((tenant) => {
      if (!search) {
        return true;
      }

      return tenant.name.toLowerCase().includes(search) || tenant.slug.toLowerCase().includes(search);
    })
    .map((tenant) => ({
      ...tenant,
      status: tenant.active ? "Ativo" : "Inativo",
      actions: "Abrir"
    }));
});

const selectedTenant = computed(() =>
  tenants.value.find((tenant) => tenant.id === String(selectedTenantId.value || "").trim()) || null
);

const selectedTenantLabel = computed(() => selectedTenant.value?.name || "Selecione um cliente");

watch(
  tenants,
  (nextTenants) => {
    if (!nextTenants.length) {
      selectedTenantId.value = "";
      return;
    }

    const selectedStillExists = nextTenants.some((tenant) => tenant.id === selectedTenantId.value);
    if (!selectedStillExists) {
      selectedTenantId.value = nextTenants[0].id;
    }
  },
  { immediate: true, deep: true }
);

watch(
  selectedTenant,
  (tenant) => {
    detailDraft.name = tenant?.name || "";
    detailDraft.slug = tenant?.slug || "";
    detailDraft.active = Boolean(tenant?.active ?? true);
  },
  { immediate: true }
);

function normalizeSlug(value) {
  return String(value || "")
    .trim()
    .toLowerCase()
    .replace(/[_\s]+/g, "-")
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
}

function applyCreateSlug() {
  createDraft.slug = normalizeSlug(createDraft.slug || createDraft.name);
}

function applyDetailSlug() {
  detailDraft.slug = normalizeSlug(detailDraft.slug || detailDraft.name);
}

function selectTenant(tenantId) {
  selectedTenantId.value = String(tenantId || "").trim();
}

function resetCreateDraft() {
  createDraft.name = "";
  createDraft.slug = "";
  createDraft.active = true;
}

async function handleRefresh() {
  try {
    await tenantsStore.refreshTenants();
  } catch {
    ui.error(errorMessage.value || "Nao foi possivel atualizar os clientes.");
  }
}

async function handleCreate() {
  createSaving.value = true;
  const result = await tenantsStore.createTenant({
    name: createDraft.name,
    slug: createDraft.slug,
    active: createDraft.active
  });
  createSaving.value = false;

  if (!result?.ok) {
    ui.error(result?.message || "Nao foi possivel criar o cliente.");
    return;
  }

  resetCreateDraft();
  selectedTenantId.value = result.tenant?.id || selectedTenantId.value;
  ui.success("Cliente criado.");
}

async function handleSave() {
  if (!selectedTenant.value) {
    return;
  }

  detailSaving.value = true;
  const result = await tenantsStore.updateTenant(selectedTenant.value.id, {
    name: detailDraft.name,
    slug: detailDraft.slug,
    active: detailDraft.active
  });
  detailSaving.value = false;

  if (!result?.ok) {
    ui.error(result?.message || "Nao foi possivel atualizar o cliente.");
    return;
  }

  if (!result.noChange) {
    ui.success("Cliente atualizado.");
  }
}

async function handleArchive() {
  if (!selectedTenant.value) {
    return;
  }

  actionTenantId.value = selectedTenant.value.id;
  const result = await tenantsStore.archiveTenant(selectedTenant.value.id);
  actionTenantId.value = "";

  if (!result?.ok) {
    ui.error(result?.message || "Nao foi possivel arquivar o cliente.");
    return;
  }

  ui.success("Cliente arquivado.");
}

async function handleRestore() {
  if (!selectedTenant.value) {
    return;
  }

  actionTenantId.value = selectedTenant.value.id;
  const result = await tenantsStore.restoreTenant(selectedTenant.value.id);
  actionTenantId.value = "";

  if (!result?.ok) {
    ui.error(result?.message || "Nao foi possivel reativar o cliente.");
    return;
  }

  ui.success("Cliente reativado.");
}
</script>

<template>
  <section class="tenants-workspace">
    <header class="settings-card tenants-workspace__hero">
      <div class="settings-card__header">
        <div>
          <h2 class="settings-card__title">Clientes e agencias</h2>
          <p class="settings-card__text">
            Organize os clientes acessiveis do painel, ajuste nome e slug, e controle quem permanece ativo no ecossistema.
          </p>
        </div>

        <button class="tenants-workspace__ghost-btn" type="button" @click="handleRefresh">
          Atualizar lista
        </button>
      </div>

      <div class="tenants-workspace__metrics">
        <article class="tenants-workspace__metric-card">
          <span>Total acessivel</span>
          <strong>{{ summary.total }}</strong>
        </article>
        <article class="tenants-workspace__metric-card is-positive">
          <span>Ativos</span>
          <strong>{{ summary.active }}</strong>
        </article>
        <article class="tenants-workspace__metric-card is-muted">
          <span>Inativos</span>
          <strong>{{ summary.inactive }}</strong>
        </article>
      </div>
    </header>

    <div class="tenants-workspace__layout">
      <section class="settings-card tenants-workspace__list-card">
        <div class="tenants-workspace__section-head">
          <div>
            <h3>Base de clientes</h3>
            <p>Selecione um cliente para revisar ou editar ao lado.</p>
          </div>

          <span class="tenants-workspace__section-pill">{{ filteredRows.length }} na lista</span>
        </div>

        <div v-if="errorMessage" class="tenants-workspace__error-card">
          <strong>Nao foi possivel carregar os clientes.</strong>
          <p>{{ errorMessage }}</p>
        </div>

        <AppEntityGrid
          testid="tenants-grid"
          storage-key="tenants-workspace-grid"
          :columns="columns"
          :rows="filteredRows"
          :loading="pending"
          :search-value="searchValue"
          search-placeholder="Pesquisar cliente ou slug..."
          empty-title="Nenhum cliente encontrado"
          empty-text="Ajuste os filtros ou cadastre um novo cliente para preencher a base."
          @update:search-value="searchValue = $event"
        >
          <template #toolbar-filters>
            <label class="tenants-workspace__filter-field">
              <span>Status</span>
              <select v-model="statusFilter">
                <option value="all">Todos</option>
                <option value="active">Ativos</option>
                <option value="inactive">Inativos</option>
              </select>
            </label>
          </template>

          <template #cell-name="{ row }">
            <button
              class="tenants-workspace__row-link"
              type="button"
              :class="{ 'is-active': selectedTenantId === row.id }"
              @click="selectTenant(row.id)"
            >
              <strong>{{ row.name }}</strong>
              <span>{{ row.active ? 'Cliente ativo' : 'Cliente pausado' }}</span>
            </button>
          </template>

          <template #cell-slug="{ row }">
            <span class="tenants-workspace__slug-chip">{{ row.slug }}</span>
          </template>

          <template #cell-status="{ row }">
            <span class="tenants-workspace__status-pill" :class="row.active ? 'is-active' : 'is-inactive'">
              {{ row.status }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <button class="tenants-workspace__row-action" type="button" @click="selectTenant(row.id)">
              Abrir
            </button>
          </template>
        </AppEntityGrid>
      </section>

      <div class="tenants-workspace__side-stack">
        <section v-if="canCreate" class="settings-card tenants-workspace__panel-card">
          <div class="tenants-workspace__section-head">
            <div>
              <h3>Novo cliente</h3>
              <p>Somente admin da plataforma cria novos grupos/clientes.</p>
            </div>
          </div>

          <div class="tenants-workspace__form-grid">
            <label class="tenants-workspace__field">
              <span>Nome</span>
              <input v-model="createDraft.name" type="text" placeholder="Ex.: Grupo Centro" @blur="applyCreateSlug">
            </label>

            <label class="tenants-workspace__field">
              <span>Slug</span>
              <div class="tenants-workspace__slug-row">
                <input v-model="createDraft.slug" type="text" placeholder="grupo-centro">
                <button class="tenants-workspace__ghost-btn" type="button" @click="applyCreateSlug">Gerar</button>
              </div>
            </label>

            <label class="tenants-workspace__checkbox">
              <input v-model="createDraft.active" type="checkbox">
              <span>Criar como ativo</span>
            </label>
          </div>

          <div class="tenants-workspace__panel-actions">
            <button class="tenants-workspace__ghost-btn" type="button" @click="resetCreateDraft">Limpar</button>
            <button class="tenants-workspace__primary-btn" type="button" :disabled="createSaving" @click="handleCreate">
              {{ createSaving ? "Criando..." : "Criar cliente" }}
            </button>
          </div>
        </section>

        <section class="settings-card tenants-workspace__panel-card">
          <div class="tenants-workspace__section-head">
            <div>
              <h3>Perfil do cliente</h3>
              <p>Revise e ajuste o cadastro selecionado sem sair da listagem.</p>
            </div>

            <span class="tenants-workspace__section-pill">{{ selectedTenantLabel }}</span>
          </div>

          <div v-if="!selectedTenant" class="tenants-workspace__empty-panel">
            <strong>Nenhum cliente selecionado.</strong>
            <p>Escolha um item da lista para abrir o editor lateral.</p>
          </div>

          <template v-else>
            <div class="tenants-workspace__detail-banner" :class="selectedTenant.active ? 'is-active' : 'is-inactive'">
              <div>
                <strong>{{ selectedTenant.name }}</strong>
                <p>{{ selectedTenant.active ? 'Cliente ativo no painel.' : 'Cliente inativo, mas preservado para restauracao.' }}</p>
              </div>

              <span>{{ selectedTenant.slug }}</span>
            </div>

            <div class="tenants-workspace__form-grid">
              <label class="tenants-workspace__field">
                <span>Nome</span>
                <input v-model="detailDraft.name" type="text" :disabled="detailSaving || !manageable">
              </label>

              <label class="tenants-workspace__field">
                <span>Slug</span>
                <div class="tenants-workspace__slug-row">
                  <input v-model="detailDraft.slug" type="text" :disabled="detailSaving || !manageable">
                  <button class="tenants-workspace__ghost-btn" type="button" :disabled="detailSaving || !manageable" @click="applyDetailSlug">
                    Gerar
                  </button>
                </div>
              </label>

              <label class="tenants-workspace__checkbox">
                <input v-model="detailDraft.active" type="checkbox" :disabled="detailSaving || !manageable">
                <span>Cliente ativo no painel</span>
              </label>
            </div>

            <div class="tenants-workspace__panel-actions">
              <div class="tenants-workspace__secondary-actions">
                <button
                  v-if="selectedTenant.active"
                  class="tenants-workspace__danger-btn"
                  type="button"
                  :disabled="actionTenantId === selectedTenant.id || !manageable"
                  @click="handleArchive"
                >
                  {{ actionTenantId === selectedTenant.id ? "Arquivando..." : "Arquivar" }}
                </button>

                <button
                  v-else
                  class="tenants-workspace__ghost-btn"
                  type="button"
                  :disabled="actionTenantId === selectedTenant.id || !manageable"
                  @click="handleRestore"
                >
                  {{ actionTenantId === selectedTenant.id ? "Reativando..." : "Reativar" }}
                </button>
              </div>

              <button class="tenants-workspace__primary-btn" type="button" :disabled="detailSaving || !manageable" @click="handleSave">
                {{ detailSaving ? "Salvando..." : "Salvar cliente" }}
              </button>
            </div>

            <p v-if="!canCreate && manageable" class="tenants-workspace__help-note">
              Como admin de cliente/agencia, voce consegue manter os clientes acessiveis, mas a criacao continua restrita ao admin da plataforma.
            </p>
          </template>
        </section>
      </div>
    </div>
  </section>
</template>

<style scoped>
.tenants-workspace {
  display: grid;
  flex: 1;
  gap: 1rem;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding-right: 0.2rem;
}

.tenants-workspace__hero {
  display: grid;
  gap: 1rem;
  padding: 1rem;
}

.tenants-workspace__ghost-btn,
.tenants-workspace__row-action,
.tenants-workspace__primary-btn,
.tenants-workspace__danger-btn {
  min-height: 2.5rem;
  padding: 0 0.95rem;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(15, 23, 42, 0.72);
  color: var(--text-main);
  font-size: 0.8rem;
  font-weight: 700;
  cursor: pointer;
}

.tenants-workspace__ghost-btn:disabled,
.tenants-workspace__row-action:disabled,
.tenants-workspace__primary-btn:disabled,
.tenants-workspace__danger-btn:disabled {
  opacity: 0.56;
  cursor: not-allowed;
}

.tenants-workspace__primary-btn {
  border-color: rgba(34, 197, 94, 0.24);
  background: rgba(34, 197, 94, 0.18);
  color: #dcfce7;
}

.tenants-workspace__danger-btn {
  border-color: rgba(248, 113, 113, 0.24);
  background: rgba(127, 29, 29, 0.28);
  color: #fecaca;
}

.tenants-workspace__metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  gap: 0.75rem;
}

.tenants-workspace__metric-card {
  display: grid;
  gap: 0.3rem;
  padding: 0.95rem 1rem;
  border-radius: 1rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(15, 23, 42, 0.46);
}

.tenants-workspace__metric-card span {
  color: var(--text-muted);
  font-size: 0.76rem;
}

.tenants-workspace__metric-card strong {
  color: #ffffff;
  font-size: 1.45rem;
}

.tenants-workspace__metric-card.is-positive {
  border-color: rgba(34, 197, 94, 0.14);
  background: rgba(20, 83, 45, 0.2);
}

.tenants-workspace__metric-card.is-muted {
  border-color: rgba(148, 163, 184, 0.14);
}

.tenants-workspace__layout {
  display: grid;
  grid-template-columns: minmax(0, 1.6fr) minmax(21rem, 0.95fr);
  gap: 1rem;
  align-items: start;
  min-height: 0;
}

.tenants-workspace__list-card,
.tenants-workspace__panel-card {
  display: grid;
  gap: 1rem;
  padding: 1rem;
}

.tenants-workspace__side-stack {
  display: grid;
  gap: 1rem;
  min-height: 0;
}

.tenants-workspace__section-head {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 0.9rem;
}

.tenants-workspace__section-head h3 {
  margin: 0;
  color: #ffffff;
  font-size: 1rem;
}

.tenants-workspace__section-head p {
  margin: 0.28rem 0 0;
  color: var(--text-muted);
  font-size: 0.8rem;
  line-height: 1.45;
}

.tenants-workspace__section-pill {
  display: inline-flex;
  align-items: center;
  min-height: 1.95rem;
  padding: 0 0.72rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.16);
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.tenants-workspace__filter-field,
.tenants-workspace__field,
.tenants-workspace__checkbox {
  display: grid;
  gap: 0.42rem;
}

.tenants-workspace__filter-field span,
.tenants-workspace__field span {
  color: var(--text-muted);
  font-size: 0.74rem;
  font-weight: 700;
}

.tenants-workspace__filter-field select,
.tenants-workspace__field input {
  min-height: 2.7rem;
  width: 100%;
  padding: 0 0.85rem;
  border-radius: 0.95rem;
  border: 1px solid rgba(255, 255, 255, 0.09);
  background: rgba(15, 23, 42, 0.74);
  color: #ffffff;
}

.tenants-workspace__filter-field select:focus,
.tenants-workspace__field input:focus {
  outline: none;
  border-color: rgba(56, 189, 248, 0.45);
  box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.12);
}

.tenants-workspace__slug-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.55rem;
}

.tenants-workspace__checkbox {
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 0.65rem;
  color: var(--text-main);
  font-size: 0.82rem;
}

.tenants-workspace__checkbox input {
  width: 1rem;
  height: 1rem;
}

.tenants-workspace__form-grid {
  display: grid;
  gap: 0.9rem;
}

.tenants-workspace__panel-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.tenants-workspace__secondary-actions {
  display: flex;
  gap: 0.65rem;
  flex-wrap: wrap;
}

.tenants-workspace__error-card,
.tenants-workspace__empty-panel {
  display: grid;
  gap: 0.35rem;
  padding: 0.95rem 1rem;
  border-radius: 1rem;
  border: 1px solid rgba(248, 113, 113, 0.18);
  background: rgba(69, 10, 10, 0.16);
}

.tenants-workspace__empty-panel {
  border-color: rgba(148, 163, 184, 0.14);
  background: rgba(15, 23, 42, 0.44);
}

.tenants-workspace__error-card strong,
.tenants-workspace__empty-panel strong {
  color: #ffffff;
}

.tenants-workspace__error-card p,
.tenants-workspace__empty-panel p,
.tenants-workspace__help-note {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.45;
}

.tenants-workspace__row-link {
  display: grid;
  gap: 0.2rem;
  width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.tenants-workspace__row-link strong {
  color: #ffffff;
  font-size: 0.84rem;
}

.tenants-workspace__row-link span {
  color: var(--text-muted);
  font-size: 0.74rem;
}

.tenants-workspace__row-link.is-active strong {
  color: #7dd3fc;
}

.tenants-workspace__slug-chip,
.tenants-workspace__status-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.9rem;
  padding: 0 0.7rem;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 700;
}

.tenants-workspace__slug-chip {
  background: rgba(148, 163, 184, 0.14);
  color: var(--text-muted);
}

.tenants-workspace__status-pill.is-active,
.tenants-workspace__detail-banner.is-active {
  background: rgba(20, 83, 45, 0.2);
  color: #bbf7d0;
}

.tenants-workspace__status-pill.is-inactive,
.tenants-workspace__detail-banner.is-inactive {
  background: rgba(100, 116, 139, 0.16);
  color: #cbd5e1;
}

.tenants-workspace__detail-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.85rem;
  padding: 0.95rem 1rem;
  border-radius: 1rem;
}

.tenants-workspace__detail-banner strong {
  color: #ffffff;
  font-size: 0.9rem;
}

.tenants-workspace__detail-banner p {
  margin: 0.28rem 0 0;
  color: currentColor;
  font-size: 0.78rem;
  line-height: 1.45;
}

.tenants-workspace__detail-banner span {
  font-size: 0.76rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

@media (max-width: 980px) {
  .tenants-workspace__layout {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 720px) {
  .tenants-workspace__section-head,
  .tenants-workspace__detail-banner,
  .tenants-workspace__panel-actions {
    grid-template-columns: minmax(0, 1fr);
    display: grid;
  }

  .tenants-workspace__slug-row {
    grid-template-columns: minmax(0, 1fr);
  }

  .tenants-workspace__secondary-actions,
  .tenants-workspace__ghost-btn,
  .tenants-workspace__primary-btn,
  .tenants-workspace__danger-btn,
  .tenants-workspace__row-action {
    width: 100%;
  }
}
</style>