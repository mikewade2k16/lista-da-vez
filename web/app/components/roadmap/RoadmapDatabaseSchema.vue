<script setup lang="ts">
import { computed, ref } from "vue";
import RoadmapDatabaseDiagram from "~/components/roadmap/RoadmapDatabaseDiagram.vue";
import {
  DATABASE_SCHEMAS,
  type DatabaseSchema,
  type SchemaField,
  type SchemaStatus,
  type SchemaTable
} from "~/components/roadmap/database-schema-data";

const STATUS_LABEL: Record<SchemaStatus, string> = {
  implemented: "Implementado",
  building: "Em construção",
  planned: "Planejado"
};

const STATUS_ICON: Record<SchemaStatus, string> = {
  implemented: "check_circle",
  building: "construction",
  planned: "schedule"
};

const schemas = computed<DatabaseSchema[]>(() => DATABASE_SCHEMAS);

const selectedSchemaId = ref<string>(schemas.value[0]?.id ?? "core");
const expandedTable = ref<string>("");
const viewMode = ref<"list" | "diagram">("diagram");

const selectedSchema = computed<DatabaseSchema | undefined>(
  () => schemas.value.find((schema) => schema.id === selectedSchemaId.value)
);

const totals = computed(() => {
  const counters = { schemas: 0, tables: 0, implemented: 0, building: 0, planned: 0 };
  for (const schema of schemas.value) {
    counters.schemas += 1;
    for (const table of schema.tables) {
      counters.tables += 1;
      counters[table.status] += 1;
    }
  }
  return counters;
});

function selectSchema(schemaId: string) {
  selectedSchemaId.value = schemaId;
  expandedTable.value = "";
}

function toggleTable(tableKey: string) {
  expandedTable.value = expandedTable.value === tableKey ? "" : tableKey;
}

function tableKey(schema: DatabaseSchema, table: SchemaTable) {
  return `${schema.id}.${table.name}`;
}

function fieldFlags(field: SchemaField): string[] {
  const flags: string[] = [];
  if (field.primaryKey) flags.push("PK");
  if (field.unique && !field.primaryKey) flags.push("UNIQUE");
  if (field.foreignKey) flags.push("FK");
  if (field.nullable) flags.push("NULL");
  return flags;
}

function fkLabel(field: SchemaField): string {
  if (!field.foreignKey) return "";
  const target = `${field.foreignKey.schema}.${field.foreignKey.table}`;
  return field.foreignKey.onDelete ? `${target} (${field.foreignKey.onDelete})` : target;
}
</script>

<template>
  <div class="schema-view">
    <header class="schema-view__header">
      <div class="schema-view__heading">
        <h3 class="schema-view__title">Banco de dados — schemas e tabelas</h3>
        <p class="schema-view__text">
          Visão por schema (core, queue, contacts, finance, …). Status reflete o que já está em produção
          (implementado), em construção ou apenas planejado por fase.
        </p>
      </div>

      <div class="schema-view__totals">
        <div class="schema-view__total">
          <span class="schema-view__total-value">{{ totals.schemas }}</span>
          <span class="schema-view__total-label">Schemas</span>
        </div>
        <div class="schema-view__total">
          <span class="schema-view__total-value">{{ totals.tables }}</span>
          <span class="schema-view__total-label">Tabelas</span>
        </div>
        <div class="schema-view__total schema-view__total--implemented">
          <span class="schema-view__total-value">{{ totals.implemented }}</span>
          <span class="schema-view__total-label">Implementadas</span>
        </div>
        <div class="schema-view__total schema-view__total--building">
          <span class="schema-view__total-value">{{ totals.building }}</span>
          <span class="schema-view__total-label">Em construção</span>
        </div>
        <div class="schema-view__total schema-view__total--planned">
          <span class="schema-view__total-value">{{ totals.planned }}</span>
          <span class="schema-view__total-label">Planejadas</span>
        </div>
      </div>
    </header>

    <nav class="schema-view__schemas" aria-label="Schemas">
      <button
        v-for="schema in schemas"
        :key="schema.id"
        type="button"
        class="schema-chip"
        :class="[`schema-chip--${schema.status}`, { 'is-active': selectedSchemaId === schema.id }]"
        @click="selectSchema(schema.id)"
      >
        <span class="material-icons-round schema-chip__icon">{{ STATUS_ICON[schema.status] }}</span>
        <span class="schema-chip__label">{{ schema.label }}</span>
        <span class="schema-chip__count">{{ schema.tables.length }}</span>
      </button>
    </nav>

    <section v-if="selectedSchema" class="schema-detail" :class="`schema-detail--${selectedSchema.status}`">
      <header class="schema-detail__header">
        <div class="schema-detail__title-row">
          <h4 class="schema-detail__title">schema <code>{{ selectedSchema.label }}</code></h4>
          <span class="schema-detail__status" :class="`schema-detail__status--${selectedSchema.status}`">
            {{ STATUS_LABEL[selectedSchema.status] }}
          </span>
          <span class="schema-detail__phase">{{ selectedSchema.phase }}</span>
        </div>
        <p class="schema-detail__description">{{ selectedSchema.description }}</p>
      </header>

      <div class="view-toggle" role="tablist" aria-label="Modo de visualizacao">
        <button
          type="button"
          role="tab"
          :aria-selected="viewMode === 'diagram'"
          :class="['view-toggle__btn', { 'is-active': viewMode === 'diagram' }]"
          @click="viewMode = 'diagram'"
        >
          <span class="material-icons-round">hub</span>
          <span>Diagrama</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="viewMode === 'list'"
          :class="['view-toggle__btn', { 'is-active': viewMode === 'list' }]"
          @click="viewMode = 'list'"
        >
          <span class="material-icons-round">list_alt</span>
          <span>Lista detalhada</span>
        </button>
      </div>

      <p v-if="selectedSchema.tables.length === 0" class="schema-detail__empty">
        Nenhuma tabela documentada ainda neste schema.
      </p>

      <RoadmapDatabaseDiagram
        v-else-if="viewMode === 'diagram'"
        :schema="selectedSchema"
      />

      <ul v-else class="schema-tables">
        <li
          v-for="table in selectedSchema.tables"
          :key="tableKey(selectedSchema, table)"
          class="schema-table"
          :class="`schema-table--${table.status}`"
        >
          <button
            type="button"
            class="schema-table__head"
            @click="toggleTable(tableKey(selectedSchema, table))"
            :aria-expanded="expandedTable === tableKey(selectedSchema, table)"
          >
            <div class="schema-table__head-main">
              <span class="material-icons-round schema-table__icon">{{ STATUS_ICON[table.status] }}</span>
              <code class="schema-table__name">{{ selectedSchema.label }}.{{ table.name }}</code>
              <span class="schema-table__status" :class="`schema-table__status--${table.status}`">
                {{ STATUS_LABEL[table.status] }}
              </span>
              <span v-if="table.phase" class="schema-table__phase">{{ table.phase }}</span>
            </div>
            <span class="material-icons-round schema-table__chevron">
              {{ expandedTable === tableKey(selectedSchema, table) ? 'expand_less' : 'expand_more' }}
            </span>
          </button>

          <p class="schema-table__description">{{ table.description }}</p>

          <div v-if="expandedTable === tableKey(selectedSchema, table)" class="schema-table__body">
            <p v-if="table.fields.length === 0" class="schema-table__empty">
              Campos ainda não documentados — serão definidos quando a fase iniciar.
            </p>

            <div v-else class="schema-fields-wrapper">
              <table class="schema-fields">
                <thead>
                  <tr>
                    <th scope="col">Campo</th>
                    <th scope="col">Tipo</th>
                    <th scope="col">Flags</th>
                    <th scope="col">Default</th>
                    <th scope="col">Referência / nota</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="field in table.fields" :key="field.name">
                    <td class="schema-fields__name"><code>{{ field.name }}</code></td>
                    <td class="schema-fields__type"><code>{{ field.type }}</code></td>
                    <td class="schema-fields__flags">
                      <span
                        v-for="flag in fieldFlags(field)"
                        :key="flag"
                        class="schema-flag"
                        :class="`schema-flag--${flag.toLowerCase()}`"
                      >{{ flag }}</span>
                    </td>
                    <td class="schema-fields__default">
                      <code v-if="field.default">{{ field.default }}</code>
                      <span v-else class="schema-fields__muted">—</span>
                    </td>
                    <td class="schema-fields__ref">
                      <span v-if="field.foreignKey" class="schema-fk">→ <code>{{ fkLabel(field) }}</code></span>
                      <span v-else-if="field.description" class="schema-fields__note">{{ field.description }}</span>
                      <span v-else class="schema-fields__muted">—</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div v-if="table.indexes && table.indexes.length > 0" class="schema-indexes">
              <span class="schema-indexes__label">Índices</span>
              <ul class="schema-indexes__list">
                <li v-for="(idx, i) in table.indexes" :key="i"><code>{{ idx }}</code></li>
              </ul>
            </div>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.schema-view {
  display: grid;
  gap: 1.2rem;
}

.schema-view__header {
  display: grid;
  gap: 0.85rem;
}

.schema-view__heading {
  display: grid;
  gap: 0.3rem;
}

.schema-view__title {
  margin: 0;
  font-size: 1.15rem;
  color: var(--text-main);
}

.schema-view__text {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.85rem;
  line-height: 1.5;
  max-width: 880px;
}

.schema-view__totals {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
}

.schema-view__total {
  display: grid;
  gap: 0.1rem;
  padding: 0.6rem 0.9rem;
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.4);
  min-width: 100px;
}

.schema-view__total-value {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-main);
  line-height: 1;
}

.schema-view__total-label {
  font-size: 0.7rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.schema-view__total--implemented {
  border-color: rgba(34, 197, 94, 0.45);
  background: rgba(34, 197, 94, 0.12);
}
.schema-view__total--implemented .schema-view__total-value { color: #4ade80; }

.schema-view__total--building {
  border-color: rgba(59, 130, 246, 0.45);
  background: rgba(59, 130, 246, 0.12);
}
.schema-view__total--building .schema-view__total-value { color: #60a5fa; }

.schema-view__total--planned {
  border-color: rgba(148, 163, 184, 0.35);
  background: rgba(148, 163, 184, 0.1);
}

.schema-view__schemas {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.schema-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.5rem 0.85rem;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  background: rgba(15, 23, 42, 0.6);
  color: var(--text-main);
  font-size: 0.85rem;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  cursor: pointer;
  transition: all 0.15s ease;
}

.schema-chip:hover {
  border-color: rgba(99, 102, 241, 0.4);
  background: rgba(99, 102, 241, 0.08);
}

.schema-chip.is-active {
  border-color: rgba(99, 102, 241, 0.7);
  background: rgba(99, 102, 241, 0.18);
  color: #c7d2fe;
}

.schema-chip__icon {
  font-size: 1rem;
  line-height: 1;
}

.schema-chip--implemented .schema-chip__icon { color: #4ade80; }
.schema-chip--building .schema-chip__icon { color: #60a5fa; }
.schema-chip--planned .schema-chip__icon { color: var(--text-muted); }

.schema-chip__count {
  display: inline-block;
  padding: 0.05rem 0.45rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.18);
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 600;
}

.schema-chip.is-active .schema-chip__count {
  background: rgba(99, 102, 241, 0.25);
  color: #c7d2fe;
}

.schema-detail {
  display: grid;
  gap: 1rem;
  padding: 1.1rem;
  border-radius: 16px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.5);
}

.schema-detail--implemented { border-color: rgba(34, 197, 94, 0.3); }
.schema-detail--building { border-color: rgba(59, 130, 246, 0.35); }

.schema-detail__header {
  display: grid;
  gap: 0.45rem;
}

.schema-detail__title-row {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.schema-detail__title {
  margin: 0;
  font-size: 1.05rem;
  color: var(--text-main);
  font-weight: 600;
}

.schema-detail__title code {
  background: rgba(99, 102, 241, 0.18);
  color: #c7d2fe;
  padding: 0.1rem 0.45rem;
  border-radius: 6px;
  font-size: 0.95rem;
}

.schema-detail__status {
  padding: 0.18rem 0.55rem;
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.schema-detail__status--implemented {
  background: rgba(34, 197, 94, 0.18);
  color: #86efac;
}
.schema-detail__status--building {
  background: rgba(59, 130, 246, 0.18);
  color: #93c5fd;
}
.schema-detail__status--planned {
  background: rgba(148, 163, 184, 0.18);
  color: #cbd5f5;
}

.schema-detail__phase {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-style: italic;
  margin-left: auto;
}

.schema-detail__description {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.85rem;
  line-height: 1.5;
}

.schema-detail__empty {
  margin: 0;
  padding: 1.2rem;
  text-align: center;
  color: var(--text-muted);
  font-style: italic;
  border: 1px dashed rgba(148, 163, 184, 0.25);
  border-radius: 12px;
}

.view-toggle {
  display: inline-flex;
  gap: 0.25rem;
  padding: 0.25rem;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(148, 163, 184, 0.18);
  width: max-content;
}

.view-toggle__btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.5rem 0.85rem;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.82rem;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.12s ease, color 0.12s ease;
}

.view-toggle__btn:hover {
  background: rgba(99, 102, 241, 0.12);
  color: #c7d2fe;
}

.view-toggle__btn.is-active {
  background: rgba(99, 102, 241, 0.22);
  color: #c7d2fe;
}

.view-toggle__btn .material-icons-round {
  font-size: 1.1rem;
}

.schema-tables {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.6rem;
}

.schema-table {
  display: grid;
  gap: 0.4rem;
  padding: 0.85rem 1rem;
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.6);
}

.schema-table--implemented { border-color: rgba(34, 197, 94, 0.25); }
.schema-table--building { border-color: rgba(59, 130, 246, 0.3); }

.schema-table__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  color: inherit;
  width: 100%;
  text-align: left;
}

.schema-table__head-main {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.schema-table__icon {
  font-size: 1.05rem;
}

.schema-table--implemented .schema-table__icon { color: #4ade80; }
.schema-table--building .schema-table__icon { color: #60a5fa; }
.schema-table--planned .schema-table__icon { color: var(--text-muted); }

.schema-table__name {
  font-size: 0.92rem;
  color: var(--text-main);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.schema-table__status {
  padding: 0.12rem 0.45rem;
  border-radius: 999px;
  font-size: 0.65rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.schema-table__status--implemented {
  background: rgba(34, 197, 94, 0.15);
  color: #86efac;
}
.schema-table__status--building {
  background: rgba(59, 130, 246, 0.15);
  color: #93c5fd;
}
.schema-table__status--planned {
  background: rgba(148, 163, 184, 0.15);
  color: #cbd5f5;
}

.schema-table__phase {
  font-size: 0.72rem;
  color: var(--text-muted);
  font-style: italic;
}

.schema-table__chevron {
  font-size: 1.25rem;
  color: var(--text-muted);
  flex-shrink: 0;
}

.schema-table__description {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.8rem;
  line-height: 1.5;
}

.schema-table__body {
  display: grid;
  gap: 0.7rem;
  padding-top: 0.5rem;
  border-top: 1px solid rgba(148, 163, 184, 0.15);
}

.schema-table__empty {
  margin: 0;
  padding: 0.6rem;
  text-align: center;
  color: var(--text-muted);
  font-style: italic;
  font-size: 0.8rem;
}

.schema-fields-wrapper {
  overflow-x: auto;
}

.schema-fields {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8rem;
}

.schema-fields th,
.schema-fields td {
  padding: 0.45rem 0.6rem;
  text-align: left;
  border-bottom: 1px solid rgba(148, 163, 184, 0.1);
  vertical-align: top;
}

.schema-fields th {
  font-weight: 600;
  color: var(--text-muted);
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  background: rgba(15, 23, 42, 0.5);
}

.schema-fields code {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 0.78rem;
  color: var(--text-main);
}

.schema-fields__name code {
  color: #93c5fd;
}

.schema-fields__type code {
  color: #fbbf24;
}

.schema-fields__flags {
  display: flex;
  gap: 0.3rem;
  flex-wrap: wrap;
}

.schema-flag {
  display: inline-block;
  padding: 0.1rem 0.4rem;
  border-radius: 4px;
  font-size: 0.65rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  background: rgba(148, 163, 184, 0.15);
  color: var(--text-muted);
}

.schema-flag--pk {
  background: rgba(34, 197, 94, 0.18);
  color: #86efac;
}

.schema-flag--fk {
  background: rgba(99, 102, 241, 0.18);
  color: #c7d2fe;
}

.schema-flag--unique {
  background: rgba(245, 158, 11, 0.18);
  color: #fcd34d;
}

.schema-flag--null {
  background: rgba(148, 163, 184, 0.15);
  color: var(--text-muted);
}

.schema-fk code {
  color: #c7d2fe;
}

.schema-fields__muted {
  color: var(--text-muted);
  font-style: italic;
}

.schema-fields__note {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.schema-indexes {
  display: grid;
  gap: 0.3rem;
  padding-top: 0.5rem;
}

.schema-indexes__label {
  font-size: 0.7rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  font-weight: 600;
}

.schema-indexes__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.2rem;
}

.schema-indexes__list code {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 0.75rem;
  color: var(--text-muted);
}
</style>
