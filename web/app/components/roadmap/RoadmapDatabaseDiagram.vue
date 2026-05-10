<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { DatabaseSchema, SchemaField, SchemaTable } from "~/components/roadmap/database-schema-data";

const props = defineProps<{ schema: DatabaseSchema }>();

// ============================================================================
// Categorias visuais (cores) — inferidas pelo nome da tabela
// ============================================================================

type Category = "identity" | "rbac" | "modules" | "sessions" | "default";

const IDENTITY_TABLES = new Set([
  "organizations",
  "accounts",
  "users",
  "account_users",
  "organization_users"
]);
const RBAC_TABLES = new Set([
  "permissions",
  "role_templates",
  "role_template_permissions",
  "roles",
  "role_permissions",
  "user_role_assignments",
  "user_permission_overrides"
]);
const MODULE_TABLES = new Set(["modules", "account_modules"]);
const SESSION_TABLES = new Set(["user_sessions"]);

const CATEGORY_LABEL: Record<Category, string> = {
  identity: "Identidade",
  rbac: "Cargos e permissões",
  modules: "Módulos",
  sessions: "Sessões",
  default: "Outros"
};

const CATEGORY_ICON: Record<Category, string> = {
  identity: "group",
  rbac: "verified_user",
  modules: "extension",
  sessions: "schedule",
  default: "table_chart"
};

function categoryOf(table: SchemaTable): Category {
  if (IDENTITY_TABLES.has(table.name)) return "identity";
  if (RBAC_TABLES.has(table.name)) return "rbac";
  if (MODULE_TABLES.has(table.name)) return "modules";
  if (SESSION_TABLES.has(table.name)) return "sessions";
  return "default";
}

// ============================================================================
// Layout: agrupa por categoria; ordem fixa pra render previsivel
// ============================================================================

interface GroupView {
  category: Category;
  tables: SchemaTable[];
}

const CATEGORY_ORDER: Category[] = ["identity", "modules", "rbac", "sessions", "default"];

const groups = computed<GroupView[]>(() => {
  const map = new Map<Category, SchemaTable[]>();
  for (const table of props.schema.tables) {
    const cat = categoryOf(table);
    if (!map.has(cat)) map.set(cat, []);
    map.get(cat)!.push(table);
  }
  const result: GroupView[] = [];
  for (const cat of CATEGORY_ORDER) {
    const tables = map.get(cat);
    if (tables && tables.length > 0) {
      result.push({ category: cat, tables });
    }
  }
  return result;
});

const tablesWithFields = computed(() => props.schema.tables.filter((t) => t.fields.length > 0));

// ============================================================================
// Linhas SVG das FKs
// ============================================================================

interface FKLine {
  id: string;
  fromTable: string;
  toTable: string;
  fromColumn: string;
  toColumn: string;
  // coordenadas relativas ao container
  fromX: number;
  fromY: number;
  toX: number;
  toY: number;
  midX: number;
  midY: number;
  pathD: string;
}

const containerRef = ref<HTMLElement | null>(null);
const containerSize = ref({ width: 0, height: 0 });
const cardElements = new Map<string, HTMLElement>();
const lines = ref<FKLine[]>([]);
const hoveredFK = ref<string>("");

function setCardEl(tableName: string, el: Element | null) {
  if (el instanceof HTMLElement) {
    cardElements.set(tableName, el);
  } else {
    cardElements.delete(tableName);
  }
}

function recalc() {
  const container = containerRef.value;
  if (!container) {
    lines.value = [];
    return;
  }
  const containerRect = container.getBoundingClientRect();
  containerSize.value = { width: containerRect.width, height: containerRect.height };

  const newLines: FKLine[] = [];
  for (const table of props.schema.tables) {
    for (const field of table.fields) {
      const fk = field.foreignKey;
      if (!fk || fk.schema !== props.schema.id) continue;

      const fromEl = cardElements.get(table.name);
      const toEl = cardElements.get(fk.table);
      if (!fromEl || !toEl) continue;

      const fromRect = fromEl.getBoundingClientRect();
      const toRect = toEl.getBoundingClientRect();

      const fromX = fromRect.left + fromRect.width / 2 - containerRect.left;
      const fromY = fromRect.top + fromRect.height / 2 - containerRect.top;
      const toX = toRect.left + toRect.width / 2 - containerRect.left;
      const toY = toRect.top + toRect.height / 2 - containerRect.top;

      // Curva bezier suave: control points proporcionais a distancia
      const dx = toX - fromX;
      const dy = toY - fromY;
      const distance = Math.sqrt(dx * dx + dy * dy);
      const curvature = Math.min(distance * 0.25, 80);

      // Direção principal: horizontal vs vertical
      const isHorizontal = Math.abs(dx) > Math.abs(dy);
      const c1x = isHorizontal ? fromX + Math.sign(dx) * curvature : fromX;
      const c1y = isHorizontal ? fromY : fromY + Math.sign(dy) * curvature;
      const c2x = isHorizontal ? toX - Math.sign(dx) * curvature : toX;
      const c2y = isHorizontal ? toY : toY - Math.sign(dy) * curvature;

      const pathD = `M ${fromX} ${fromY} C ${c1x} ${c1y}, ${c2x} ${c2y}, ${toX} ${toY}`;

      newLines.push({
        id: `${table.name}.${field.name}->${fk.table}`,
        fromTable: table.name,
        toTable: fk.table,
        fromColumn: field.name,
        toColumn: "id",
        fromX,
        fromY,
        toX,
        toY,
        midX: (fromX + toX) / 2,
        midY: (fromY + toY) / 2,
        pathD
      });
    }
  }
  lines.value = newLines;
}

let resizeObserver: ResizeObserver | null = null;

onMounted(async () => {
  await nextTick();
  recalc();
  if (containerRef.value && typeof ResizeObserver !== "undefined") {
    resizeObserver = new ResizeObserver(() => recalc());
    resizeObserver.observe(containerRef.value);
  }
});

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  resizeObserver = null;
});

watch(
  () => props.schema.id,
  async () => {
    cardElements.clear();
    await nextTick();
    recalc();
  }
);

// ============================================================================
// Helpers de renderização
// ============================================================================

function fieldFlags(field: SchemaField): string {
  const flags: string[] = [];
  if (field.primaryKey) flags.push("PK");
  if (field.foreignKey) flags.push(`FK→${field.foreignKey.table}`);
  if (field.unique && !field.primaryKey) flags.push("UQ");
  return flags.join(" · ");
}

function isHighlighted(table: SchemaTable, field: SchemaField): boolean {
  if (!hoveredFK.value) return false;
  return hoveredFK.value === `${table.name}.${field.name}`;
}

function isCardConnected(tableName: string): boolean {
  if (!hoveredFK.value) return false;
  const line = lines.value.find((l) => `${l.fromTable}.${l.fromColumn}` === hoveredFK.value);
  if (!line) return false;
  return line.fromTable === tableName || line.toTable === tableName;
}

function setHoveredFK(table: SchemaTable, field: SchemaField) {
  if (field.foreignKey) {
    hoveredFK.value = `${table.name}.${field.name}`;
  }
}

function clearHoveredFK() {
  hoveredFK.value = "";
}

const totalFKs = computed(() => lines.value.length);
</script>

<template>
  <div class="diagram-view">
    <header class="diagram-view__header">
      <p class="diagram-view__hint">
        <span class="material-icons-round">tips_and_updates</span>
        <span>
          Passe o mouse sobre um campo <strong class="diagram-view__hint-fk">FK→</strong>
          para destacar a linha de relacionamento e os cards conectados. Linhas tracejadas
          ligam a coluna ao destino. Cores agrupam tabelas por área (identidade, RBAC,
          módulos, sessões).
        </span>
      </p>
      <div class="diagram-view__legend">
        <span class="diagram-legend"><i class="diagram-legend__swatch diagram-legend__swatch--identity"></i> Identidade</span>
        <span class="diagram-legend"><i class="diagram-legend__swatch diagram-legend__swatch--modules"></i> Módulos</span>
        <span class="diagram-legend"><i class="diagram-legend__swatch diagram-legend__swatch--rbac"></i> Cargos/Permissões</span>
        <span class="diagram-legend"><i class="diagram-legend__swatch diagram-legend__swatch--sessions"></i> Sessões</span>
        <span class="diagram-legend"><i class="diagram-legend__line"></i> FK ({{ totalFKs }})</span>
      </div>
    </header>

    <div v-if="tablesWithFields.length === 0" class="diagram-empty">
      <span class="material-icons-round">construction</span>
      <p>
        Schema <code>{{ schema.label }}</code> ainda não tem campos detalhados.
        O diagrama vai aparecer quando as tabelas forem implementadas (status atual: {{ schema.phase }}).
      </p>
    </div>

    <div v-else ref="containerRef" class="diagram-canvas">
      <!-- SVG das linhas FK -->
      <svg
        class="diagram-svg"
        :viewBox="`0 0 ${containerSize.width} ${containerSize.height}`"
        :width="containerSize.width"
        :height="containerSize.height"
        preserveAspectRatio="xMinYMin meet"
        aria-hidden="true"
      >
        <defs>
          <marker id="diagram-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="rgba(148, 163, 184, 0.5)" />
          </marker>
          <marker id="diagram-arrow-active" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#60a5fa" />
          </marker>
        </defs>

        <g
          v-for="line in lines"
          :key="line.id"
          :class="['diagram-line', { 'diagram-line--active': hoveredFK === line.id }]"
        >
          <path
            :d="line.pathD"
            fill="none"
            stroke-dasharray="5 4"
            :stroke-width="hoveredFK === line.id ? 2.5 : 1.5"
            :marker-end="hoveredFK === line.id ? 'url(#diagram-arrow-active)' : 'url(#diagram-arrow)'"
          />
        </g>
      </svg>

      <!-- Grid de cards -->
      <div class="diagram-grid">
        <section v-for="group in groups" :key="group.category" class="diagram-group">
          <header class="diagram-group__header">
            <span class="material-icons-round" :class="`diagram-group__icon diagram-group__icon--${group.category}`">
              {{ CATEGORY_ICON[group.category] }}
            </span>
            <h4 class="diagram-group__title">{{ CATEGORY_LABEL[group.category] }}</h4>
            <span class="diagram-group__count">{{ group.tables.length }}</span>
          </header>

          <div class="diagram-group__cards">
            <article
              v-for="table in group.tables"
              :key="table.name"
              :ref="(el) => setCardEl(table.name, el as Element | null)"
              :class="[
                'erd-card',
                `erd-card--${categoryOf(table)}`,
                { 'erd-card--connected': isCardConnected(table.name) }
              ]"
            >
              <header class="erd-card__head">
                <span class="material-icons-round erd-card__icon">{{ CATEGORY_ICON[categoryOf(table)] }}</span>
                <h5 class="erd-card__title">{{ table.name }}</h5>
              </header>

              <ul class="erd-card__fields">
                <li
                  v-for="field in table.fields"
                  :key="field.name"
                  :class="[
                    'erd-field',
                    {
                      'erd-field--pk': field.primaryKey,
                      'erd-field--fk': !!field.foreignKey,
                      'erd-field--highlighted': isHighlighted(table, field)
                    }
                  ]"
                  @mouseenter="setHoveredFK(table, field)"
                  @mouseleave="clearHoveredFK()"
                >
                  <span class="erd-field__name">{{ field.name }}</span>
                  <span v-if="fieldFlags(field)" class="erd-field__flag">{{ fieldFlags(field) }}</span>
                </li>
              </ul>

              <footer class="erd-card__footer">
                <span>{{ table.fields.length }} campos</span>
                <span v-if="table.indexes && table.indexes.length > 0">
                  · {{ table.indexes.length }} índices
                </span>
              </footer>
            </article>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.diagram-view {
  display: grid;
  gap: 1rem;
}

.diagram-view__header {
  display: grid;
  gap: 0.6rem;
}

.diagram-view__hint {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  margin: 0;
  padding: 0.6rem 0.8rem;
  border-radius: 10px;
  background: rgba(99, 102, 241, 0.08);
  border: 1px solid rgba(99, 102, 241, 0.18);
  color: #c7d2fe;
  font-size: 0.8rem;
  line-height: 1.5;
}

.diagram-view__hint .material-icons-round {
  font-size: 1.05rem;
  color: #a5b4fc;
  margin-top: 0.05rem;
  flex-shrink: 0;
}

.diagram-view__hint-fk {
  background: rgba(99, 102, 241, 0.25);
  padding: 0.05rem 0.3rem;
  border-radius: 4px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 0.78rem;
}

.diagram-view__legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.85rem;
  font-size: 0.78rem;
  color: var(--text-muted);
}

.diagram-legend {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.diagram-legend__swatch {
  display: inline-block;
  width: 12px;
  height: 12px;
  border-radius: 3px;
}

.diagram-legend__swatch--identity { background: #3b82f6; }
.diagram-legend__swatch--modules { background: #10b981; }
.diagram-legend__swatch--rbac { background: #a855f7; }
.diagram-legend__swatch--sessions { background: #f59e0b; }

.diagram-legend__line {
  display: inline-block;
  width: 22px;
  height: 0;
  border-top: 2px dashed rgba(148, 163, 184, 0.6);
}

.diagram-empty {
  display: grid;
  place-items: center;
  gap: 0.6rem;
  padding: 3rem 1rem;
  border: 1px dashed rgba(148, 163, 184, 0.3);
  border-radius: 14px;
  color: var(--text-muted);
  text-align: center;
}

.diagram-empty .material-icons-round {
  font-size: 2.2rem;
  color: var(--text-muted);
}

.diagram-empty p {
  margin: 0;
  max-width: 480px;
  line-height: 1.5;
  font-size: 0.88rem;
}

.diagram-empty code {
  background: rgba(99, 102, 241, 0.18);
  color: #c7d2fe;
  padding: 0.05rem 0.4rem;
  border-radius: 4px;
}

.diagram-canvas {
  position: relative;
  padding: 1rem;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.5);
  border: 1px solid rgba(148, 163, 184, 0.18);
  min-height: 500px;
}

.diagram-svg {
  position: absolute;
  top: 1rem;
  left: 1rem;
  width: calc(100% - 2rem);
  height: calc(100% - 2rem);
  pointer-events: none;
  z-index: 0;
}

.diagram-line path {
  stroke: rgba(148, 163, 184, 0.4);
  transition: stroke 0.15s ease, stroke-width 0.15s ease;
}

.diagram-line--active path {
  stroke: #60a5fa;
  stroke-width: 2.5;
}

.diagram-grid {
  position: relative;
  z-index: 1;
  display: grid;
  gap: 1.5rem;
}

.diagram-group {
  display: grid;
  gap: 0.75rem;
}

.diagram-group__header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.diagram-group__icon {
  font-size: 1.1rem;
}

.diagram-group__icon--identity { color: #60a5fa; }
.diagram-group__icon--modules { color: #34d399; }
.diagram-group__icon--rbac { color: #c084fc; }
.diagram-group__icon--sessions { color: #fbbf24; }
.diagram-group__icon--default { color: var(--text-muted); }

.diagram-group__title {
  margin: 0;
  font-size: 0.95rem;
  color: var(--text-main);
  font-weight: 600;
}

.diagram-group__count {
  margin-left: 0.25rem;
  padding: 0.05rem 0.5rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.18);
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 600;
}

.diagram-group__cards {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
}

.erd-card {
  display: grid;
  gap: 0.55rem;
  padding: 0.85rem;
  border-radius: 12px;
  border-left: 4px solid rgba(148, 163, 184, 0.5);
  background: rgba(15, 23, 42, 0.85);
  border-top: 1px solid rgba(148, 163, 184, 0.18);
  border-right: 1px solid rgba(148, 163, 184, 0.18);
  border-bottom: 1px solid rgba(148, 163, 184, 0.18);
  transition: border-color 0.15s ease, transform 0.15s ease, box-shadow 0.15s ease;
  position: relative;
}

.erd-card--identity { border-left-color: #3b82f6; box-shadow: 0 0 0 0 rgba(59, 130, 246, 0); }
.erd-card--modules { border-left-color: #10b981; }
.erd-card--rbac { border-left-color: #a855f7; }
.erd-card--sessions { border-left-color: #f59e0b; }

.erd-card--connected {
  border-color: rgba(96, 165, 250, 0.55);
  box-shadow: 0 0 0 2px rgba(96, 165, 250, 0.25);
}

.erd-card__head {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.erd-card__icon {
  font-size: 1rem;
  color: var(--text-muted);
}

.erd-card--identity .erd-card__icon { color: #60a5fa; }
.erd-card--modules .erd-card__icon { color: #34d399; }
.erd-card--rbac .erd-card__icon { color: #c084fc; }
.erd-card--sessions .erd-card__icon { color: #fbbf24; }

.erd-card__title {
  margin: 0;
  font-size: 0.88rem;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  color: var(--text-main);
}

.erd-card__fields {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.18rem;
}

.erd-field {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.25rem 0.45rem;
  border-radius: 6px;
  font-size: 0.76rem;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  color: var(--text-muted);
  cursor: default;
  transition: background 0.12s ease, color 0.12s ease;
}

.erd-field--pk {
  color: #fcd34d;
  font-weight: 600;
}

.erd-field--fk {
  color: #93c5fd;
  cursor: help;
}

.erd-field--fk:hover,
.erd-field--highlighted {
  background: rgba(96, 165, 250, 0.18);
  color: #dbeafe;
}

.erd-field__name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.erd-field__flag {
  font-size: 0.68rem;
  color: var(--text-muted);
  flex-shrink: 0;
}

.erd-field--pk .erd-field__flag { color: #fbbf24; }
.erd-field--fk .erd-field__flag { color: #93c5fd; }

.erd-card__footer {
  display: flex;
  gap: 0.3rem;
  font-size: 0.7rem;
  color: var(--text-muted);
  border-top: 1px solid rgba(148, 163, 184, 0.12);
  padding-top: 0.4rem;
}
</style>
