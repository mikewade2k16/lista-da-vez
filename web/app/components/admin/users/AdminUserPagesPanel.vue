<script setup lang="ts">
import type { AdminUserItem } from '~/types/admin-users'
import { WORKSPACE_ACCESS_DEFINITIONS } from '~/domain/utils/permissions'
import { useAccessControlStore } from '~/stores/access-control'

// Painel "Paginas". Liga/desliga por usuario a visibilidade de cada pagina do
// painel (workspace) com tri-estado: Herdar (usa o padrao do papel) / Mostrar
// (allow) / Ocultar (deny). IMPORTANTE: o gating de pagina do front vem do modulo
// legado `access` (auth.permissionKeys <- /v1/me/context <- access_role_permissions
// + user_access_overrides). Por isso este painel grava nos overrides de access
// (store access-control, PUT /v1/access/users/{id}/overrides) e NAO no core — so
// assim o menu/pagina realmente muda. Registrado em docs/LEGADO.md. Ao migrar o
// gating de pagina para o core (fase futura), este painel troca de fonte.
//
// O override de pagina e por-usuario (global, com escopo de tenant resolvido no
// backend): controla o que ESTE usuario ve, inclusive usuarios so-organizacao
// (sem cliente), atendendo ao controle total de acesso.

type TriState = 'inherit' | 'allow' | 'deny'

const props = defineProps<{ user: AdminUserItem }>()
const emit = defineEmits<{ updated: [] }>()

const access = useAccessControlStore()

const loading = ref(true)
const saving = ref(false)
const errorMessage = ref('')

// Catalogo de chaves de permissao reconhecidas pelo backend de access. So
// oferecemos toggle para paginas cujo view-permission esta no catalogo (senao o
// PUT volta 422 invalid). Carregado junto do acesso do usuario.
const catalogKeys = ref<Set<string>>(new Set())
// Estado tri por workspaceId (rascunho do usuario).
const states = reactive<Record<string, TriState>>({})
// Snapshot do estado salvo (para detectar pendencia e o resumo do botao).
const savedStates = reactive<Record<string, TriState>>({})
// Overrides do usuario que NAO sao de pagina (ex.: permissoes sensiveis). Sao
// preservados ao salvar, pois o backend substitui TODOS os overrides do usuario.
const otherOverrides = ref<{ permissionKey: string; effect: string; note: string }[]>([])
// Chaves view com o padrao do papel (perfil) marcado, para o hint "Perfil: ...".
const baseKeys = ref<Set<string>>(new Set())

// Paginas controlaveis: workspaces com view-permission. A relevancia no catalogo
// e checada depois (catalogKeys), via `isControllable`.
const pageRows = computed(() =>
  WORKSPACE_ACCESS_DEFINITIONS.filter((w) => String(w.viewPermission || '').trim()).map((w) => ({
    id: w.id,
    label: w.label,
    description: w.description,
    viewPermission: String(w.viewPermission).trim(),
  })),
)

function isControllable(viewPermission: string): boolean {
  return catalogKeys.value.has(viewPermission)
}

// Paginas exibiveis: as que o backend reconhece. As demais sao controladas por
// modulo/papel (sem override de pagina) — mostramos um aviso em vez de toggle morto.
const controllableRows = computed(() =>
  pageRows.value.filter((r) => isControllable(r.viewPermission)),
)
const uncontrollableCount = computed(() => pageRows.value.length - controllableRows.value.length)

function isDirty(): boolean {
  return Object.keys(states).some((id) => states[id] !== savedStates[id])
}

const overrideCount = computed(() => Object.values(states).filter((s) => s !== 'inherit').length)

function applyAccess(data: ReturnType<typeof access.getUserAccess>) {
  for (const key of Object.keys(states)) delete states[key]
  for (const key of Object.keys(savedStates)) delete savedStates[key]
  if (!data) return

  catalogKeys.value = new Set((data.permissions || []).map((p) => p.key))
  baseKeys.value = new Set(data.basePermissionKeys || [])

  // Mapa view-permission -> effect dos overrides atuais; o resto preserva.
  const viewKeys = new Set(pageRows.value.map((r) => r.viewPermission))
  const overrideByKey = new Map<string, string>()
  otherOverrides.value = []
  for (const ov of data.overrides || []) {
    if (viewKeys.has(ov.permissionKey)) {
      overrideByKey.set(ov.permissionKey, ov.effect)
    } else {
      otherOverrides.value.push({
        permissionKey: ov.permissionKey,
        effect: ov.effect,
        note: ov.note || '',
      })
    }
  }

  for (const row of pageRows.value) {
    const effect = overrideByKey.get(row.viewPermission)
    const state: TriState = effect === 'allow' ? 'allow' : effect === 'deny' ? 'deny' : 'inherit'
    states[row.id] = state
    savedStates[row.id] = state
  }
}

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await access.loadUserAccess(props.user.id)
    applyAccess(data)
  } catch {
    errorMessage.value = access.errorMessage || 'Nao foi possivel carregar o acesso do usuario.'
  } finally {
    loading.value = false
  }
}

watch(() => props.user.id, load, { immediate: true })

function setState(id: string, value: TriState) {
  states[id] = value
}

// Hint do padrao do papel para a pagina (perfil concede ou nao por padrao).
function baseLabel(viewPermission: string): string {
  return baseKeys.value.has(viewPermission) ? 'Perfil: visivel' : 'Perfil: oculto'
}

// Efetivo = override vence o padrao do papel.
function effectiveLabel(row: { id: string; viewPermission: string }): string {
  const state = states[row.id]
  if (state === 'allow') return 'Visivel'
  if (state === 'deny') return 'Oculto'
  return baseKeys.value.has(row.viewPermission) ? 'Visivel' : 'Oculto'
}

async function save() {
  if (saving.value || !isDirty()) return
  saving.value = true
  errorMessage.value = ''

  // Preserva overrides nao-pagina + adiciona os estados de pagina (allow/deny).
  const payload = [...otherOverrides.value]
  for (const row of pageRows.value) {
    const state = states[row.id]
    if (state === 'allow' || state === 'deny') {
      payload.push({ permissionKey: row.viewPermission, effect: state, note: '' })
    }
  }

  const result = await access.saveUserOverrides(props.user.id, payload)
  saving.value = false
  if (result.ok) {
    applyAccess(result.access)
    emit('updated')
  } else {
    errorMessage.value = result.message || 'Nao foi possivel salvar as paginas.'
  }
}
</script>

<template>
  <section class="admin-user-pages">
    <UAlert
      v-if="errorMessage"
      class="admin-user-pages__error"
      color="error"
      variant="soft"
      icon="i-lucide-alert-triangle"
      :description="errorMessage"
    />

    <p v-if="loading" class="admin-user-pages__muted">Carregando paginas...</p>

    <template v-else>
      <p class="admin-user-pages__legend">
        <strong>Herdar</strong>
        usa o padrao do papel do usuario.
        <strong>Mostrar</strong>
        /
        <strong>Ocultar</strong>
        sobrescrevem so este usuario. O efeito vale em todo o painel para este usuario.
      </p>

      <p v-if="!controllableRows.length" class="admin-user-pages__muted">
        Nenhuma pagina com controle de visibilidade disponivel neste ambiente.
      </p>

      <ul v-else class="admin-user-pages__list">
        <li v-for="row in controllableRows" :key="row.id" class="admin-user-pages__row">
          <div class="admin-user-pages__copy">
            <span class="admin-user-pages__label">{{ row.label }}</span>
            <span class="admin-user-pages__desc">{{ row.description }}</span>
            <span class="admin-user-pages__meta">
              <span>{{ baseLabel(row.viewPermission) }}</span>
              <span class="admin-user-pages__meta-sep">·</span>
              <span>Efetivo: {{ effectiveLabel(row) }}</span>
            </span>
          </div>
          <div class="admin-user-pages__tri" role="group" :aria-label="row.label">
            <button
              type="button"
              class="admin-user-pages__tri-btn"
              :class="{ 'is-active': states[row.id] === 'inherit' }"
              @click="setState(row.id, 'inherit')"
            >
              Herdar
            </button>
            <button
              type="button"
              class="admin-user-pages__tri-btn admin-user-pages__tri-btn--allow"
              :class="{ 'is-active': states[row.id] === 'allow' }"
              @click="setState(row.id, 'allow')"
            >
              Mostrar
            </button>
            <button
              type="button"
              class="admin-user-pages__tri-btn admin-user-pages__tri-btn--deny"
              :class="{ 'is-active': states[row.id] === 'deny' }"
              @click="setState(row.id, 'deny')"
            >
              Ocultar
            </button>
          </div>
        </li>
      </ul>

      <p v-if="uncontrollableCount > 0" class="admin-user-pages__muted">
        {{ uncontrollableCount }} pagina(s) sao controladas por modulo/papel (aba Modulos) e nao tem
        override de visibilidade por usuario.
      </p>

      <div class="admin-user-pages__foot">
        <span class="admin-user-pages__count">
          {{ overrideCount }} override(s) de pagina; o restante herda do papel.
        </span>
        <UButton
          label="Salvar paginas"
          color="primary"
          :loading="saving"
          :disabled="saving || !isDirty()"
          @click="save"
        />
      </div>
    </template>
  </section>
</template>

<style scoped>
.admin-user-pages {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.admin-user-pages__error {
  margin: 0;
}

.admin-user-pages__muted {
  margin: 0;
  font-size: 0.8rem;
  color: rgb(var(--muted));
}

.admin-user-pages__legend {
  margin: 0;
  font-size: 0.76rem;
  color: rgb(var(--muted));
}

.admin-user-pages__legend strong {
  color: rgb(var(--text));
}

.admin-user-pages__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.admin-user-pages__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.55rem 0.65rem;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface-2));
}

.admin-user-pages__copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.12rem;
}

.admin-user-pages__label {
  font-size: 0.85rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.admin-user-pages__desc {
  font-size: 0.74rem;
  color: rgb(var(--muted));
  line-height: 1.35;
}

.admin-user-pages__meta {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.7rem;
  color: rgb(var(--muted));
}

.admin-user-pages__meta-sep {
  opacity: 0.5;
}

.admin-user-pages__tri {
  display: inline-flex;
  flex-shrink: 0;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  overflow: hidden;
}

.admin-user-pages__tri-btn {
  padding: 0.3rem 0.6rem;
  font-size: 0.74rem;
  border: none;
  background: rgb(var(--surface));
  color: rgb(var(--muted));
  cursor: pointer;
}

.admin-user-pages__tri-btn + .admin-user-pages__tri-btn {
  border-left: 1px solid rgb(var(--border));
}

.admin-user-pages__tri-btn.is-active {
  background: rgb(var(--primary));
  color: rgb(var(--surface));
}

.admin-user-pages__tri-btn--allow.is-active {
  background: rgb(var(--success));
}

.admin-user-pages__tri-btn--deny.is-active {
  background: rgb(var(--danger));
}

.admin-user-pages__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.admin-user-pages__count {
  font-size: 0.76rem;
  color: rgb(var(--muted));
}
</style>
