<script setup lang="ts">
import { ref } from "vue";

type ColumnDef = {
  name: string;
  type: string;
  nullable?: boolean;
  default?: string;
  note?: string;
};

type RelationDef = {
  from: string;
  to: string;
  label: string;
};

type TableDef = {
  name: string;
  label: string;
  status: "legacy" | "nova" | "estavel" | "relacional" | "catalogo";
  description: string;
  phase?: string;
  columns: ColumnDef[];
  relations?: RelationDef[];
};

const SETTINGS_TABLES: TableDef[] = [
  {
    name: "tenant_operation_settings",
    label: "Settings legado (tabela principal)",
    status: "legacy",
    description: "Tabela legada com 91+ colunas cobrindo operacao, modal, alertas e configuracoes diversas. Nao e mais escrita nem lida como fonte autoritativa — todas as secoes foram migradas para as tabelas novas na Fase 9. Mantida apenas como ancora de FK para opcoes e catalogo.",
    phase: "legado — Fase 9 concluida",
    columns: [
      { name: "tenant_id", type: "uuid", note: "PK — chave do tenant" },
      { name: "selected_operation_template_id", type: "text", note: "Template selecionado" },
      { name: "max_concurrent_services", type: "int", note: "Limite de atendimentos simultaneos por loja" },
      { name: "max_concurrent_services_per_consultant", type: "int", nullable: true, default: "1", note: "Limite por consultor (COALESCE 1)" },
      { name: "timing_fast_close_minutes", type: "int", note: "Tempo para atendimento rapido" },
      { name: "timing_long_service_minutes", type: "int", note: "Tempo para atendimento longo" },
      { name: "timing_low_sale_amount", type: "numeric", note: "Ticket minimo de venda baixa" },
      { name: "service_cancel_window_seconds", type: "int", nullable: true, default: "30", note: "Janela de cancelamento (COALESCE 30)" },
      { name: "test_mode_enabled", type: "bool", note: "Modo de teste" },
      { name: "auto_fill_finish_modal", type: "bool", note: "Auto-preenchimento do modal" },
      { name: "alert_min_conversion_rate", type: "numeric", note: "Alerta: conversao minima" },
      { name: "alert_max_queue_jump_rate", type: "numeric", note: "Alerta: taxa maxima de fora da vez" },
      { name: "alert_min_pa_score", type: "numeric", note: "Alerta: PA minima" },
      { name: "alert_min_ticket_average", type: "numeric", note: "Alerta: ticket medio minimo" },
      { name: "finish_flow_mode", type: "text", nullable: true, default: "legacy", note: "Modo do modal: legacy | erp-reconciliation" },
      { name: "title", type: "text", note: "Titulo do modal" },
      { name: "product_seen_label", type: "text", note: "Label produto visto" },
      { name: "...", type: "...", note: "74 campos adicionais de labels, placeholders, show/hide, required e modos de selecao do modal" },
      { name: "created_at", type: "timestamptz", note: "Criacao" },
      { name: "updated_at", type: "timestamptz", note: "Ultima atualizacao" }
    ]
  },
  {
    name: "tenant_operation_core_settings",
    label: "Core operacional (tabela nova)",
    status: "nova",
    description: "Tabela separada para configuracoes operacionais estaveis e tipadas. Criada na Fase 3 com backfill. Ativada com dual-read/write na Fase 6. Leitura e escrita exclusivas desde a Fase 9 — sem fallback legacy. Scan por nome com pgx.RowToStructByName (Fase 8).",
    phase: "Fase 3 criada | Fase 6 dual-write | Fase 9 exclusiva",
    columns: [
      { name: "tenant_id", type: "uuid", note: "PK — chave do tenant" },
      { name: "selected_operation_template_id", type: "text", note: "Template operacional selecionado" },
      { name: "max_concurrent_services", type: "int", note: "Limite de atendimentos simultaneos" },
      { name: "max_concurrent_services_per_consultant", type: "int", default: "1", note: "Limite por consultor" },
      { name: "timing_fast_close_minutes", type: "int", note: "Tempo para atendimento rapido (minutos)" },
      { name: "timing_long_service_minutes", type: "int", note: "Tempo para atendimento longo (minutos)" },
      { name: "timing_low_sale_amount", type: "numeric", note: "Valor minimo de ticket baixo" },
      { name: "service_cancel_window_seconds", type: "int", default: "30", note: "Janela de cancelamento (segundos)" },
      { name: "test_mode_enabled", type: "bool", default: "false", note: "Modo de teste ativo" },
      { name: "auto_fill_finish_modal", type: "bool", default: "false", note: "Auto-preenchimento do modal de encerramento" },
      { name: "updated_by", type: "uuid", nullable: true, note: "ID do usuario que fez a ultima alteracao" },
      { name: "updated_at", type: "timestamptz", note: "Ultima atualizacao" }
    ],
    relations: [
      { from: "tenant_id", to: "tenant_operation_settings.tenant_id", label: "ancora de FK para opcoes/catalogo (sem leitura/escrita desde Fase 9)" }
    ]
  },
  {
    name: "tenant_finish_modal_settings",
    label: "Modal de encerramento (tabela nova)",
    status: "nova",
    description: "Configuracao do modal de encerramento em documento jsonb versionado. Criada na Fase 3, ativada com dual-read/write na Fase 4. Leitura e escrita exclusivas desde a Fase 9. A coluna finish_flow_mode e autoritativa sobre o jsonb. Aplicacao de template via endpoint transacional (Fase 7).",
    phase: "Fase 3 criada | Fase 4 dual-write | Fase 9 exclusiva",
    columns: [
      { name: "tenant_id", type: "uuid", note: "PK — chave do tenant" },
      { name: "finish_flow_mode", type: "text", default: "legacy", note: "Modo do fluxo de encerramento: legacy | erp-reconciliation. Prevalece sobre o jsonb." },
      { name: "schema_version", type: "int", default: "1", note: "Versao do schema do config jsonb. v1 cobre todos os 76 campos do modal." },
      { name: "config", type: "jsonb", note: "Documento camelCase com labels, placeholders, show/hide, required e modos de selecao. Chaves alinham com json.Marshal do ModalConfig Go." },
      { name: "updated_by", type: "uuid", nullable: true, note: "ID do usuario que fez a ultima alteracao" },
      { name: "updated_at", type: "timestamptz", note: "Ultima atualizacao" }
    ],
    relations: [
      { from: "tenant_id", to: "tenant_operation_settings.tenant_id", label: "ancora de FK (sem leitura/escrita desde Fase 9)" },
      { from: "config (jsonb)", to: "ModalConfig (Go struct)", label: "json.Marshal/Unmarshal — camelCase" }
    ]
  },
  {
    name: "tenant_alert_settings",
    label: "Alertas operacionais (tabela nova)",
    status: "nova",
    description: "Thresholds de alertas operacionais separados do core. Criada na Fase 3, ativada com dual-read/write na Fase 5. Leitura e escrita exclusivas desde a Fase 9. Os 4 campos de alerta sao sobrepostos pelo GetOperationSection. Scan por nome com pgx.RowToStructByName (Fase 8).",
    phase: "Fase 3 criada | Fase 5 dual-write | Fase 9 exclusiva",
    columns: [
      { name: "tenant_id", type: "uuid", note: "PK — chave do tenant" },
      { name: "alert_min_conversion_rate", type: "numeric", note: "Taxa minima de conversao esperada (0-1)" },
      { name: "alert_max_queue_jump_rate", type: "numeric", note: "Taxa maxima de fora da vez tolerada (0-1)" },
      { name: "alert_min_pa_score", type: "numeric", note: "PA (pontuacao de atendimento) minima esperada" },
      { name: "alert_min_ticket_average", type: "numeric", note: "Ticket medio minimo esperado (R$)" },
      { name: "updated_by", type: "uuid", nullable: true, note: "ID do usuario que fez a ultima alteracao" },
      { name: "updated_at", type: "timestamptz", note: "Ultima atualizacao" }
    ],
    relations: [
      { from: "tenant_id", to: "tenant_operation_settings.tenant_id", label: "ancora de FK (sem leitura/escrita desde Fase 9)" }
    ]
  },
  {
    name: "tenant_setting_options",
    label: "Catalogos de opcoes",
    status: "relacional",
    description: "Catalogo normalizado de opcoes configuráveis por tenant. Cada grupo (kind) forma uma lista ordenada de itens. Permanece como tabela relacional independente.",
    phase: "estavel",
    columns: [
      { name: "id", type: "text", note: "PK — identificador unico do item" },
      { name: "tenant_id", type: "uuid", note: "Escopo do tenant" },
      { name: "kind", type: "text", note: "Grupo: visit_reason | customer_source | pause_reason | queue_jump_reason | loss_reason | profession" },
      { name: "label", type: "text", note: "Texto exibido na UI" },
      { name: "sort_order", type: "int", nullable: true, note: "Ordem explicita dentro do grupo" },
      { name: "active", type: "bool", default: "true", note: "Se o item esta ativo/visivel" },
      { name: "created_at", type: "timestamptz", note: "Criacao" },
      { name: "updated_at", type: "timestamptz", note: "Ultima atualizacao" }
    ],
    relations: [
      { from: "tenant_id", to: "tenants.id", label: "escopo por tenant" }
    ]
  },
  {
    name: "tenant_catalog_products",
    label: "Catalogo manual de produtos",
    status: "catalogo",
    description: "Catalogo administrativo de produtos configurado manualmente por tenant. Diferente do catalogo ERP (erp_item_current), este e de curadoria manual para o modal de atendimento.",
    phase: "estavel",
    columns: [
      { name: "id", type: "text", note: "PK — identificador unico do produto" },
      { name: "tenant_id", type: "uuid", note: "Escopo do tenant" },
      { name: "name", type: "text", note: "Nome do produto exibido no modal" },
      { name: "code", type: "text", nullable: true, note: "Codigo/referencia do produto" },
      { name: "sort_order", type: "int", nullable: true, note: "Ordem de exibicao no modal" },
      { name: "active", type: "bool", default: "true", note: "Se o produto esta ativo" },
      { name: "created_at", type: "timestamptz", note: "Criacao" },
      { name: "updated_at", type: "timestamptz", note: "Ultima atualizacao" }
    ],
    relations: [
      { from: "tenant_id", to: "tenants.id", label: "escopo por tenant" }
    ]
  },
  {
    name: "store_operation_settings",
    label: "Settings por loja (legado pre-tenant)",
    status: "legacy",
    description: "Tabela legada anterior a migracao para escopo por tenant. Mantida como fonte de backfill para tenants que ainda nao tem linha em tenant_operation_settings. Nao e mais escrita.",
    phase: "pre-legado",
    columns: [
      { name: "store_id", type: "bigint", note: "PK — ID da loja (modelo antigo)" },
      { name: "...", type: "...", note: "Mesmos campos de tenant_operation_settings, mas scoped por store_id" },
      { name: "updated_at", type: "timestamptz", note: "Ultima atualizacao" }
    ]
  },
  {
    name: "store_setting_options",
    label: "Opcoes por loja (legado pre-tenant)",
    status: "legacy",
    description: "Tabela legada de opcoes scoped por loja. Mantida como fonte de backfill. Nao e mais escrita apos a migracao para tenant_setting_options.",
    phase: "pre-legado",
    columns: [
      { name: "id", type: "text", note: "PK" },
      { name: "store_id", type: "bigint", note: "Escopo da loja (modelo antigo)" },
      { name: "kind", type: "text", note: "Grupo de opcao" },
      { name: "label", type: "text", note: "Texto exibido" },
      { name: "sort_order", type: "int", nullable: true, note: "Ordem" },
      { name: "active", type: "bool", note: "Ativo" }
    ]
  },
  {
    name: "store_catalog_products",
    label: "Catalogo de produtos por loja (legado pre-tenant)",
    status: "legacy",
    description: "Tabela legada de catalogo manual scoped por loja. Mantida como fonte de backfill. Nao e mais escrita apos a migracao para tenant_catalog_products.",
    phase: "pre-legado",
    columns: [
      { name: "id", type: "text", note: "PK" },
      { name: "store_id", type: "bigint", note: "Escopo da loja (modelo antigo)" },
      { name: "name", type: "text", note: "Nome do produto" },
      { name: "code", type: "text", nullable: true, note: "Codigo" },
      { name: "sort_order", type: "int", nullable: true, note: "Ordem" },
      { name: "active", type: "bool", note: "Ativo" }
    ]
  }
];

const STATUS_LABELS: Record<string, string> = {
  legacy: "legado",
  nova: "nova",
  estavel: "estavel",
  relacional: "relacional",
  catalogo: "catalogo"
};

const expandedTables = ref<Set<string>>(new Set(["tenant_operation_core_settings", "tenant_finish_modal_settings", "tenant_alert_settings", "tenant_setting_options", "tenant_catalog_products"]));

function toggleTable(name: string) {
  if (expandedTables.value.has(name)) {
    expandedTables.value.delete(name);
  } else {
    expandedTables.value.add(name);
  }
}
</script>

<template>
  <section class="banco-schema">
    <div class="banco-schema__legend">
      <span class="banco-schema__chip banco-schema__chip--nova">nova</span>
      <span class="banco-schema__chip-label">Tabela criada na refatoracao — em fase de ativacao</span>
      <span class="banco-schema__chip banco-schema__chip--estavel">estavel</span>
      <span class="banco-schema__chip-label">Tabela madura, sem alteracoes estruturais previstas</span>
      <span class="banco-schema__chip banco-schema__chip--relacional">relacional</span>
      <span class="banco-schema__chip-label">Tabela relacional normalizada</span>
      <span class="banco-schema__chip banco-schema__chip--catalogo">catalogo</span>
      <span class="banco-schema__chip-label">Catalogo administrativo manual</span>
      <span class="banco-schema__chip banco-schema__chip--legacy">legado</span>
      <span class="banco-schema__chip-label">Tabela legada — sera descontinuada</span>
    </div>

    <div class="banco-schema__migration-status">
      <div class="banco-schema__migration-item">
        <span class="material-icons-round banco-schema__migration-icon banco-schema__migration-icon--done">check_circle</span>
        <div>
          <strong>Fase 4 concluida</strong>
          <span>Modal — dual-read/write em tenant_finish_modal_settings</span>
        </div>
      </div>
      <div class="banco-schema__migration-item">
        <span class="material-icons-round banco-schema__migration-icon banco-schema__migration-icon--done">check_circle</span>
        <div>
          <strong>Fase 5 concluida</strong>
          <span>Alertas — dual-read/write em tenant_alert_settings</span>
        </div>
      </div>
      <div class="banco-schema__migration-item">
        <span class="material-icons-round banco-schema__migration-icon banco-schema__migration-icon--done">check_circle</span>
        <div>
          <strong>Fase 6 concluida</strong>
          <span>Core — dual-read/write em tenant_operation_core_settings</span>
        </div>
      </div>
      <div class="banco-schema__migration-item">
        <span class="material-icons-round banco-schema__migration-icon banco-schema__migration-icon--done">check_circle</span>
        <div>
          <strong>Fase 7 concluida</strong>
          <span>Template transacional — POST /v1/settings/templates/:id/apply atomico</span>
        </div>
      </div>
      <div class="banco-schema__migration-item">
        <span class="material-icons-round banco-schema__migration-icon banco-schema__migration-icon--done">check_circle</span>
        <div>
          <strong>Fase 8 concluida</strong>
          <span>Scan por nome — pgx.RowToStructByName nas 3 tabelas novas</span>
        </div>
      </div>
      <div class="banco-schema__migration-item banco-schema__migration-item--highlight">
        <span class="material-icons-round banco-schema__migration-icon banco-schema__migration-icon--done">verified</span>
        <div>
          <strong>Fase 9 concluida — migracao completa</strong>
          <span>Corte — tenant_operation_settings nao e mais lida/escrita para modal, core ou alertas</span>
        </div>
      </div>
    </div>

    <div class="banco-schema__tables">
      <article
        v-for="table in SETTINGS_TABLES"
        :key="table.name"
        class="banco-schema__table"
        :class="`banco-schema__table--${table.status}`"
      >
        <button
          type="button"
          class="banco-schema__table-head"
          @click="toggleTable(table.name)"
        >
          <div class="banco-schema__table-head-left">
            <span :class="['material-icons-round', 'banco-schema__chevron', expandedTables.has(table.name) ? 'banco-schema__chevron--open' : '']">chevron_right</span>
            <code class="banco-schema__table-name">{{ table.name }}</code>
            <span :class="`banco-schema__chip banco-schema__chip--${table.status}`">{{ STATUS_LABELS[table.status] }}</span>
            <span v-if="table.phase" class="banco-schema__phase">{{ table.phase }}</span>
          </div>
          <span class="banco-schema__col-count">{{ table.columns.length }} cols</span>
        </button>

        <div v-if="expandedTables.has(table.name)" class="banco-schema__table-body">
          <p class="banco-schema__table-desc">{{ table.description }}</p>

          <div class="banco-schema__columns">
            <div class="banco-schema__columns-header">
              <span>Coluna</span>
              <span>Tipo</span>
              <span>Observacao</span>
            </div>
            <div
              v-for="col in table.columns"
              :key="col.name"
              class="banco-schema__column-row"
            >
              <code class="banco-schema__col-name">{{ col.name }}</code>
              <span class="banco-schema__col-type">
                <code>{{ col.type }}</code>
                <span v-if="col.nullable" class="banco-schema__col-tag banco-schema__col-tag--null">nullable</span>
                <span v-if="col.default" class="banco-schema__col-tag banco-schema__col-tag--default">default {{ col.default }}</span>
              </span>
              <span class="banco-schema__col-note">{{ col.note || "" }}</span>
            </div>
          </div>

          <div v-if="table.relations && table.relations.length > 0" class="banco-schema__relations">
            <strong class="banco-schema__relations-title">
              <span class="material-icons-round">share</span>
              Relacionamentos
            </strong>
            <div
              v-for="rel in table.relations"
              :key="rel.from + rel.to"
              class="banco-schema__relation-row"
            >
              <code class="banco-schema__col-name">{{ rel.from }}</code>
              <span class="material-icons-round banco-schema__rel-arrow">arrow_forward</span>
              <code class="banco-schema__col-name">{{ rel.to }}</code>
              <span class="banco-schema__rel-label">{{ rel.label }}</span>
            </div>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.banco-schema {
  display: grid;
  gap: 1rem;
}

.banco-schema__legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.8rem;
  padding: 0.75rem 1rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.75rem;
  background: rgba(13, 18, 29, 0.7);
  font-size: 0.78rem;
  color: var(--text-muted);
}

.banco-schema__chip-label {
  margin-right: 0.6rem;
}

.banco-schema__migration-status {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 0.6rem;
}

.banco-schema__migration-item {
  display: flex;
  align-items: flex-start;
  gap: 0.6rem;
  padding: 0.75rem 1rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.75rem;
  background: rgba(13, 18, 29, 0.7);
  font-size: 0.82rem;
}

.banco-schema__migration-item > div {
  display: grid;
  gap: 0.1rem;
}

.banco-schema__migration-item strong {
  font-size: 0.84rem;
  color: var(--text-main);
}

.banco-schema__migration-item span {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.banco-schema__migration-icon {
  font-size: 1.1rem;
  flex-shrink: 0;
  margin-top: 0.1rem;
}

.banco-schema__migration-icon--done { color: #53c6a0; }
.banco-schema__migration-icon--pending { color: #888; }

.banco-schema__migration-item--highlight {
  border-color: rgba(83, 198, 160, 0.35);
  background: rgba(83, 198, 160, 0.06);
}

.banco-schema__migration-item--highlight strong {
  color: #53c6a0;
}

.banco-schema__tables {
  display: grid;
  gap: 0.6rem;
}

.banco-schema__table {
  border: 1px solid var(--line-soft);
  border-radius: 1rem;
  overflow: hidden;
  background: rgba(13, 18, 29, 0.9);
}

.banco-schema__table--nova { border-color: rgba(83, 198, 160, 0.3); }
.banco-schema__table--legacy { border-color: rgba(180, 100, 80, 0.25); }
.banco-schema__table--relacional { border-color: rgba(98, 129, 255, 0.25); }
.banco-schema__table--catalogo { border-color: rgba(200, 170, 60, 0.25); }
.banco-schema__table--estavel { border-color: rgba(120, 160, 255, 0.2); }

.banco-schema__table-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  width: 100%;
  padding: 0.85rem 1rem;
  border: none;
  background: transparent;
  color: var(--text-main);
  text-align: left;
  cursor: pointer;
}

.banco-schema__table-head:hover {
  background: rgba(255, 255, 255, 0.03);
}

.banco-schema__table-head-left {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.banco-schema__chevron {
  font-size: 1.05rem;
  color: var(--text-muted);
  transition: transform 0.15s ease;
}

.banco-schema__chevron--open {
  transform: rotate(90deg);
}

.banco-schema__table-name {
  font-size: 0.85rem;
  color: #b8d0ff;
  word-break: break-all;
}

.banco-schema__phase {
  font-size: 0.72rem;
  color: var(--text-muted);
}

.banco-schema__col-count {
  font-size: 0.72rem;
  color: var(--text-muted);
  white-space: nowrap;
}

.banco-schema__table-body {
  padding: 0 1rem 1rem;
  display: grid;
  gap: 0.85rem;
  border-top: 1px solid var(--line-soft);
}

.banco-schema__table-desc {
  margin: 0.65rem 0 0;
  color: var(--text-muted);
  font-size: 0.8rem;
  line-height: 1.55;
  max-width: 70ch;
}

.banco-schema__columns {
  display: grid;
  gap: 0;
  border: 1px solid var(--line-soft);
  border-radius: 0.6rem;
  overflow: hidden;
  font-size: 0.8rem;
}

.banco-schema__columns-header {
  display: grid;
  grid-template-columns: 1fr 1fr 2fr;
  gap: 0.5rem;
  padding: 0.45rem 0.75rem;
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.banco-schema__column-row {
  display: grid;
  grid-template-columns: 1fr 1fr 2fr;
  gap: 0.5rem;
  padding: 0.42rem 0.75rem;
  border-top: 1px solid var(--line-soft);
  align-items: start;
}

.banco-schema__column-row:nth-child(even) {
  background: rgba(255, 255, 255, 0.015);
}

.banco-schema__col-name {
  font-size: 0.78rem;
  color: #b8d0ff;
  word-break: break-word;
}

.banco-schema__col-type {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.3rem;
  color: var(--text-muted);
}

.banco-schema__col-type code {
  font-size: 0.75rem;
  color: #a0c4a0;
}

.banco-schema__col-note {
  color: var(--text-muted);
  font-size: 0.77rem;
  line-height: 1.45;
}

.banco-schema__col-tag {
  font-size: 0.66rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 0.1rem 0.35rem;
  border-radius: 999px;
}

.banco-schema__col-tag--null {
  background: rgba(180, 100, 80, 0.18);
  color: #e0a090;
}

.banco-schema__col-tag--default {
  background: rgba(98, 129, 255, 0.15);
  color: #c0ccff;
}

.banco-schema__chip {
  display: inline-flex;
  align-items: center;
  font-size: 0.68rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  white-space: nowrap;
}

.banco-schema__chip--nova {
  background: rgba(83, 198, 160, 0.18);
  color: #53c6a0;
}

.banco-schema__chip--legacy {
  background: rgba(180, 100, 80, 0.18);
  color: #e09080;
}

.banco-schema__chip--estavel {
  background: rgba(120, 160, 255, 0.18);
  color: #a0c0ff;
}

.banco-schema__chip--relacional {
  background: rgba(98, 129, 255, 0.18);
  color: #c0ccff;
}

.banco-schema__chip--catalogo {
  background: rgba(200, 170, 60, 0.18);
  color: #e0c878;
}

.banco-schema__relations {
  display: grid;
  gap: 0.45rem;
  padding: 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.6rem;
  background: rgba(255, 255, 255, 0.02);
}

.banco-schema__relations-title {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.78rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.banco-schema__relations-title .material-icons-round {
  font-size: 0.95rem;
}

.banco-schema__relation-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.78rem;
}

.banco-schema__rel-arrow {
  font-size: 0.9rem;
  color: var(--text-muted);
}

.banco-schema__rel-label {
  color: var(--text-muted);
  font-size: 0.75rem;
}
</style>
