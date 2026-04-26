<script setup>
import { computed, reactive, ref, watch } from "vue";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import { formatCurrencyBRL, formatPercent } from "~/domain/utils/admin-metrics";
import { canManageStores } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { useMultiStoreStore } from "~/stores/multistore";
import { useUiStore } from "~/stores/ui";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const multiStore = useMultiStoreStore();
const ui = useUiStore();
const auth = useAuthStore();
const storeDrafts = ref({});
const newStore = reactive({
  name: "",
  code: "",
  city: "",
  cloneActiveRoster: true,
  defaultTemplateId: "",
  monthlyGoal: "",
  weeklyGoal: "",
  avgTicketGoal: "",
  conversionGoal: "",
  paGoal: ""
});

const activeRole = computed(() => {
  const activeProfile =
    (props.state.profiles || []).find((profile) => profile.id === props.state.activeProfileId) ||
    props.state.profiles?.[0] ||
    null;

  return activeProfile?.role || "consultant";
});
const canEditStores = computed(() => canManageStores(auth.role, auth.permissionKeys, auth.permissionsResolved));
const operationTemplates = computed(() => props.state.operationTemplates || []);
const templateOptions = computed(() => [
  { value: "", label: "Template padrao" },
  ...operationTemplates.value.map((template) => ({
    value: String(template.id || "").trim(),
    label: String(template.label || "").trim()
  }))
]);
const overview = computed(() => multiStore.overview || null);
const rows = computed(() => overview.value?.stores || []);
const managedStores = computed(() => props.state.managedStores || props.state.stores || []);
const activeManagedStores = computed(() => managedStores.value.filter((store) => store.isActive !== false));
const archivedManagedStores = computed(() => managedStores.value.filter((store) => store.isActive === false));
const summary = computed(() => overview.value?.summary || {
  activeStores: rows.value.length,
  totalAttendances: 0,
  totalSoldValue: 0,
  totalQueue: 0,
  totalActiveServices: 0,
  averageHealthScore: 0
});

watch(
  () => managedStores.value,
  (stores) => {
    storeDrafts.value = Object.fromEntries(
      (stores || []).map((store) => [
        store.id,
        {
          name: store.name,
          code: store.code || "",
          city: store.city || "",
          defaultTemplateId: store.defaultTemplateId || "",
          monthlyGoal: store.monthlyGoal || "",
          weeklyGoal: store.weeklyGoal || "",
          avgTicketGoal: store.avgTicketGoal || "",
          conversionGoal: store.conversionGoal || "",
          paGoal: store.paGoal || ""
        }
      ])
    );
  },
  { immediate: true, deep: true }
);

async function saveStore(storeId) {
  const result = await multiStore.updateStore(storeId, storeDrafts.value[storeId]);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel atualizar loja.");
    return;
  }

  if (result?.noChange) {
    ui.info("Nenhuma alteracao para salvar.");
    return;
  }

  ui.success("Loja atualizada.");
}

async function createStore() {
  const result = await multiStore.createStore(newStore);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel criar loja.");
    return;
  }

  newStore.name = "";
  newStore.code = "";
  newStore.city = "";
  newStore.cloneActiveRoster = true;
  newStore.defaultTemplateId = "";
  newStore.monthlyGoal = "";
  newStore.weeklyGoal = "";
  newStore.avgTicketGoal = "";
  newStore.conversionGoal = "";
  newStore.paGoal = "";
  ui.success("Loja criada.");

  if (result?.warningMessage) {
    ui.info(result.warningMessage);
  }
}

async function archiveStore(storeId) {
  const { confirmed } = await ui.confirm({
    title: "Arquivar loja",
    message: "A loja sera removida da operacao ativa. Deseja continuar?",
    confirmLabel: "Arquivar"
  });

  if (!confirmed) {
    return;
  }

  const result = await multiStore.archiveStore(storeId);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel arquivar loja.");
    return;
  }

  ui.success("Loja arquivada.");
}

async function restoreStore(storeId) {
  const result = await multiStore.restoreStore(storeId);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel restaurar loja.");
    return;
  }

  ui.success("Loja restaurada.");
}

async function deleteStore(store) {
  const { confirmed } = await ui.confirm({
    title: "Excluir loja",
    message: `A exclusao so funciona para loja sem consultores, acessos e historico operacional. Deseja tentar remover ${store?.name || "esta loja"}?`,
    confirmLabel: "Excluir"
  });

  if (!confirmed) {
    return;
  }

  const result = await multiStore.deleteStore(store?.id);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel remover loja.");
    return;
  }

  ui.success("Loja removida.");
}
</script>

<template>
  <section class="admin-panel" data-testid="multistore-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Visao consolidada multi-loja</h2>
      <p class="admin-panel__text">Comparativo operacional para acompanhar performance entre lojas.</p>
    </header>

    <article v-if="multiStore.errorMessage" class="settings-card">
      <p class="settings-card__text">{{ multiStore.errorMessage }}</p>
    </article>

    <article v-else-if="multiStore.pending && !multiStore.ready" class="settings-card">
      <p class="settings-card__text">Carregando consolidado multiloja...</p>
    </article>

    <section class="metric-grid" data-testid="multistore-summary">
      <article class="metric-card"><span class="metric-card__label">Lojas ativas</span><strong class="metric-card__value">{{ summary.activeStores }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Atendimentos consolidados</span><strong class="metric-card__value">{{ summary.totalAttendances }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Vendas consolidadas</span><strong class="metric-card__value">{{ formatCurrencyBRL(summary.totalSoldValue) }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Fila atual total</span><strong class="metric-card__value">{{ summary.totalQueue }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Em atendimento agora</span><strong class="metric-card__value">{{ summary.totalActiveServices }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Score medio operacional</span><strong class="metric-card__value">{{ Math.round(summary.averageHealthScore) }}</strong></article>
    </section>

    <article class="insight-card insight-card--wide" data-testid="multistore-comparison-table">
      <h3 class="insight-card__title">Comparativo consolidado por loja</h3>
      <div class="insight-table-wrap">
        <table class="insight-table">
          <thead>
            <tr>
              <th>Loja</th>
              <th>Consultores</th>
              <th>Atendimentos</th>
              <th>Conversao</th>
              <th>Meta conv.</th>
              <th>Vendas</th>
              <th>Meta mensal</th>
              <th>% Meta</th>
              <th>Ticket medio</th>
              <th>Meta ticket</th>
              <th>P.A.</th>
              <th>Meta P.A.</th>
              <th>Score</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!rows.length"><td colspan="14">Sem lojas cadastradas.</td></tr>
            <tr v-for="row in rows" :key="row.storeId">
              <td>{{ row.storeName }}</td>
              <td>{{ row.consultants }}</td>
              <td>{{ row.attendances }}</td>
              <td>{{ formatPercent(row.conversionRate) }}</td>
              <td>{{ row.conversionGoal ? formatPercent(row.conversionGoal) : '-' }}</td>
              <td>{{ formatCurrencyBRL(row.soldValue) }}</td>
              <td>{{ row.monthlyGoal ? formatCurrencyBRL(row.monthlyGoal) : '-' }}</td>
              <td>
                <span v-if="row.monthlyGoal" :class="row.soldValue >= row.monthlyGoal ? 'multistore-goal--hit' : 'multistore-goal--miss'">
                  {{ formatPercent(row.monthlyGoal ? (row.soldValue / row.monthlyGoal) * 100 : 0) }}
                </span>
                <span v-else>-</span>
              </td>
              <td>{{ formatCurrencyBRL(row.ticketAverage) }}</td>
              <td>{{ row.avgTicketGoal ? formatCurrencyBRL(row.avgTicketGoal) : '-' }}</td>
              <td>{{ row.paScore.toFixed(2) }}</td>
              <td>{{ row.paGoal ? row.paGoal.toFixed(2) : '-' }}</td>
              <td>{{ Math.round(row.healthScore) }}</td>
              <td>
                <span v-if="row.storeId === state.activeStoreId" class="insight-tag">Ativa</span>
                <button v-else class="option-row__save" type="button" @click="multiStore.setActiveStore(row.storeId)">Abrir</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </article>

    <article class="insight-card insight-card--wide" data-testid="multistore-meeting">
      <header class="intel-card__header">
        <h3 class="insight-card__title">Painel de reuniao gerencial</h3>
        <span class="insight-tag">{{ rows.length }} lojas</span>
      </header>
      <div class="meeting-grid">
        <div v-for="row in rows" :key="row.storeId" class="meeting-card">
          <div class="meeting-card__header">
            <h4 class="meeting-card__name">{{ row.storeName }}</h4>
            <span class="meeting-card__score">Score {{ Math.round(row.healthScore) }}</span>
          </div>
          <div class="meeting-goal-list">
            <div class="meeting-goal-row">
              <span class="meeting-goal-row__label">Vendas vs meta mensal</span>
              <div class="meeting-goal-row__bar">
                <div class="meeting-goal-row__track">
                  <div
                    :class="['meeting-goal-row__fill', row.monthlyGoal && row.soldValue >= row.monthlyGoal ? 'meeting-goal-row__fill--hit' : 'meeting-goal-row__fill--miss']"
                    :style="{ width: row.monthlyGoal ? `${Math.min(100, (row.soldValue / row.monthlyGoal) * 100).toFixed(1)}%` : '0%' }"
                  ></div>
                </div>
                <span :class="['meeting-goal-row__value', row.monthlyGoal && row.soldValue >= row.monthlyGoal ? 'meeting-goal-row__value--hit' : 'meeting-goal-row__value--miss']">
                  {{ row.monthlyGoal ? formatPercent((row.soldValue / row.monthlyGoal) * 100) : formatCurrencyBRL(row.soldValue) }}
                </span>
              </div>
            </div>
            <div class="meeting-goal-row">
              <span class="meeting-goal-row__label">Conversao vs meta</span>
              <div class="meeting-goal-row__bar">
                <div class="meeting-goal-row__track">
                  <div
                    :class="['meeting-goal-row__fill', row.conversionGoal && row.conversionRate >= row.conversionGoal ? 'meeting-goal-row__fill--hit' : 'meeting-goal-row__fill--miss']"
                    :style="{ width: row.conversionGoal ? `${Math.min(100, (row.conversionRate / row.conversionGoal) * 100).toFixed(1)}%` : `${Math.min(100, row.conversionRate).toFixed(1)}%` }"
                  ></div>
                </div>
                <span :class="['meeting-goal-row__value', row.conversionGoal && row.conversionRate >= row.conversionGoal ? 'meeting-goal-row__value--hit' : 'meeting-goal-row__value--miss']">
                  {{ formatPercent(row.conversionRate) }}
                </span>
              </div>
            </div>
            <div class="meeting-goal-row">
              <span class="meeting-goal-row__label">Ticket medio vs meta</span>
              <div class="meeting-goal-row__bar">
                <div class="meeting-goal-row__track">
                  <div
                    :class="['meeting-goal-row__fill', row.avgTicketGoal && row.ticketAverage >= row.avgTicketGoal ? 'meeting-goal-row__fill--hit' : 'meeting-goal-row__fill--miss']"
                    :style="{ width: row.avgTicketGoal ? `${Math.min(100, (row.ticketAverage / row.avgTicketGoal) * 100).toFixed(1)}%` : '0%' }"
                  ></div>
                </div>
                <span :class="['meeting-goal-row__value', row.avgTicketGoal && row.ticketAverage >= row.avgTicketGoal ? 'meeting-goal-row__value--hit' : 'meeting-goal-row__value--miss']">
                  {{ formatCurrencyBRL(row.ticketAverage) }}
                </span>
              </div>
            </div>
            <div class="meeting-goal-row">
              <span class="meeting-goal-row__label">P.A. vs meta</span>
              <div class="meeting-goal-row__bar">
                <div class="meeting-goal-row__track">
                  <div
                    :class="['meeting-goal-row__fill', row.paGoal && row.paScore >= row.paGoal ? 'meeting-goal-row__fill--hit' : 'meeting-goal-row__fill--miss']"
                    :style="{ width: row.paGoal ? `${Math.min(100, (row.paScore / row.paGoal) * 100).toFixed(1)}%` : '0%' }"
                  ></div>
                </div>
                <span :class="['meeting-goal-row__value', row.paGoal && row.paScore >= row.paGoal ? 'meeting-goal-row__value--hit' : 'meeting-goal-row__value--miss']">
                  {{ row.paScore.toFixed(2) }}
                </span>
              </div>
            </div>
            <div class="meeting-goal-row">
              <span class="meeting-goal-row__label">Atendimentos</span>
              <div class="meeting-goal-row__bar">
                <div class="meeting-goal-row__track">
                  <div
                    class="meeting-goal-row__fill meeting-goal-row__fill--hit"
                    :style="{ width: summary.totalAttendances ? `${((row.attendances / summary.totalAttendances) * 100).toFixed(1)}%` : '0%' }"
                  ></div>
                </div>
                <span class="meeting-goal-row__value meeting-goal-row__value--hit">{{ row.attendances }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </article>

    <article v-if="canEditStores" class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Gestao de lojas</h3>
        <p class="settings-card__text">Identificacao, template operacional e metas por loja.</p>
      </header>

      <div class="option-list">
        <form
          v-for="store in activeManagedStores"
          :key="store.id"
          class="multistore-form"
          @submit.prevent="saveStore(store.id)"
        >
          <div class="multistore-form__row">
            <input v-model="storeDrafts[store.id].name" class="product-row__input" type="text" placeholder="Nome">
            <input v-model="storeDrafts[store.id].code" class="product-row__input" type="text" placeholder="Codigo">
            <input v-model="storeDrafts[store.id].city" class="product-row__input" type="text" placeholder="Cidade">
            <AppSelectField
              class="product-row__input"
              :model-value="storeDrafts[store.id].defaultTemplateId"
              :options="templateOptions"
              placeholder="Template padrao"
              @update:model-value="storeDrafts[store.id].defaultTemplateId = $event"
            />
          </div>
          <div class="multistore-form__row">
            <label class="multistore-form__field">
              <span class="multistore-form__label">Meta mensal (R$)</span>
              <input v-model="storeDrafts[store.id].monthlyGoal" class="product-row__input" type="number" min="0" step="100" placeholder="0">
            </label>
            <label class="multistore-form__field">
              <span class="multistore-form__label">Meta semanal (R$)</span>
              <input v-model="storeDrafts[store.id].weeklyGoal" class="product-row__input" type="number" min="0" step="100" placeholder="0">
            </label>
            <label class="multistore-form__field">
              <span class="multistore-form__label">Ticket médio alvo (R$)</span>
              <input v-model="storeDrafts[store.id].avgTicketGoal" class="product-row__input" type="number" min="0" step="100" placeholder="0">
            </label>
            <label class="multistore-form__field">
              <span class="multistore-form__label">Conversão alvo (%)</span>
              <input v-model="storeDrafts[store.id].conversionGoal" class="product-row__input" type="number" min="0" max="100" step="1" placeholder="0">
            </label>
            <label class="multistore-form__field">
              <span class="multistore-form__label">P.A. alvo</span>
              <input v-model="storeDrafts[store.id].paGoal" class="product-row__input" type="number" min="0" step="0.1" placeholder="0">
            </label>
          </div>
          <div class="multistore-form__actions">
            <button class="option-row__save" type="submit">Salvar</button>
            <button class="product-row__remove" type="button" @click="archiveStore(store.id)">Arquivar</button>
            <button class="product-row__remove" type="button" @click="deleteStore(store)">Excluir</button>
          </div>
        </form>
      </div>

      <div v-if="archivedManagedStores.length" class="option-list">
        <article v-for="store in archivedManagedStores" :key="store.id" class="option-row">
          <div class="option-row__content">
            <strong>{{ store.name }}</strong>
            <span class="settings-card__text">{{ store.code || "Sem codigo" }} <template v-if="store.city">• {{ store.city }}</template></span>
          </div>
          <div class="multistore-form__actions">
            <button class="option-row__save" type="button" @click="restoreStore(store.id)">Restaurar</button>
            <button class="product-row__remove" type="button" @click="deleteStore(store)">Excluir</button>
          </div>
        </article>
      </div>

      <form class="multistore-form multistore-form--add" @submit.prevent="createStore">
        <div class="multistore-form__row">
          <input v-model="newStore.name" class="product-add__input" type="text" placeholder="Nome da loja *" data-testid="multistore-new-name">
          <input v-model="newStore.code" class="product-add__input" type="text" placeholder="Codigo curto">
          <input v-model="newStore.city" class="product-add__input" type="text" placeholder="Cidade">
          <AppSelectField
            class="product-add__input"
            :model-value="newStore.defaultTemplateId"
            :options="templateOptions"
            placeholder="Template padrao"
            @update:model-value="newStore.defaultTemplateId = $event"
          />
        </div>
        <div class="multistore-form__row">
          <label class="multistore-form__field">
            <span class="multistore-form__label">Meta mensal (R$)</span>
            <input v-model="newStore.monthlyGoal" class="product-add__input" type="number" min="0" step="100" placeholder="0">
          </label>
          <label class="multistore-form__field">
            <span class="multistore-form__label">Meta semanal (R$)</span>
            <input v-model="newStore.weeklyGoal" class="product-add__input" type="number" min="0" step="100" placeholder="0">
          </label>
          <label class="multistore-form__field">
            <span class="multistore-form__label">Ticket médio alvo (R$)</span>
            <input v-model="newStore.avgTicketGoal" class="product-add__input" type="number" min="0" step="100" placeholder="0">
          </label>
          <label class="multistore-form__field">
            <span class="multistore-form__label">Conversão alvo (%)</span>
            <input v-model="newStore.conversionGoal" class="product-add__input" type="number" min="0" max="100" step="1" placeholder="0">
          </label>
          <label class="multistore-form__field">
            <span class="multistore-form__label">P.A. alvo</span>
            <input v-model="newStore.paGoal" class="product-add__input" type="number" min="0" step="0.1" placeholder="0">
          </label>
        </div>
        <div class="multistore-form__actions">
          <label class="settings-toggle">
            <input v-model="newStore.cloneActiveRoster" type="checkbox">
            <span>Copiar consultores da loja ativa</span>
          </label>
          <button class="product-add__button" type="submit" data-testid="multistore-new-submit">Adicionar loja</button>
        </div>
      </form>
    </article>
  </section>
</template>

