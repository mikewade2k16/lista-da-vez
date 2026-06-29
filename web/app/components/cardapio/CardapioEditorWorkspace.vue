<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import CardapioSectionDados from '~/components/cardapio/sections/CardapioSectionDados.vue'
import CardapioSectionCategorias from '~/components/cardapio/sections/CardapioSectionCategorias.vue'
import CardapioSectionProdutos from '~/components/cardapio/sections/CardapioSectionProdutos.vue'
import CardapioSectionAvaliacoes from '~/components/cardapio/sections/CardapioSectionAvaliacoes.vue'
import CardapioSectionPedidos from '~/components/cardapio/sections/CardapioSectionPedidos.vue'
import CardapioSectionRelatorios from '~/components/cardapio/sections/CardapioSectionRelatorios.vue'
import CardapioSectionEntrega from '~/components/cardapio/sections/CardapioSectionEntrega.vue'
import CardapioSectionDominios from '~/components/cardapio/sections/CardapioSectionDominios.vue'
import CardapioSectionAparencia from '~/components/cardapio/sections/CardapioSectionAparencia.vue'
import CardapioSectionSite from '~/components/cardapio/sections/CardapioSectionSite.vue'
import CardapioEditorClientSelect from '~/components/cardapio/CardapioEditorClientSelect.vue'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../../layers/core/stores/account'
import { getApiErrorMessage } from '~/utils/api-client'

const props = defineProps<{ restaurantId: string; accountId?: string }>()

const store = useCardapioStore()
const ui = useUiStore()
const auth = useAuthStore()
const accountStore = useCoreAccountStore()

type SectionId =
  | 'dados'
  | 'categorias'
  | 'produtos'
  | 'avaliacoes'
  | 'pedidos'
  | 'relatorios'
  | 'entrega'
  | 'dominios'
  | 'aparencia'
  | 'site'

// Faixa de acesso de uma secao. Decisao 2026-06-22: SEM split operacao/config —
// quem tem o modulo cardapio ve tudo ('all'); so Dominios/Site ficam restritos a
// plataforma (Crow / platform_admin). Mapeamento no doc PLANO_CARDAPIO_GESTAO_UX.
type SectionGate = 'all' | 'platform'

interface SectionTab {
  id: SectionId
  label: string
  gate: SectionGate
}

const SECTIONS: SectionTab[] = [
  { id: 'dados', label: 'Dados', gate: 'all' },
  { id: 'categorias', label: 'Categorias', gate: 'all' },
  { id: 'produtos', label: 'Produtos', gate: 'all' },
  { id: 'avaliacoes', label: 'Avaliacoes', gate: 'all' },
  { id: 'pedidos', label: 'Pedidos', gate: 'all' },
  { id: 'relatorios', label: 'Relatorios', gate: 'all' },
  { id: 'entrega', label: 'Entrega', gate: 'all' },
  { id: 'dominios', label: 'Dominios', gate: 'platform' },
  { id: 'aparencia', label: 'Aparencia', gate: 'all' },
  { id: 'site', label: 'Site', gate: 'platform' },
]

// platform_admin = papel platform_admin OU modo dev (platformView) do switcher —
// espelha o CardapioEditorClientSelect e o resto do painel. So ele ve Dominios/Site.
const isPlatformAdmin = computed(
  () => String(auth.role || '').trim() === 'platform_admin' || accountStore.platformView,
)

// Fail-safe de hidratacao (espelha useDashboardNav): enquanto o contexto da conta
// nao resolveu (accountsLoaded false), NAO filtra — evita flash sumindo secoes
// platform durante o load. Resolvido = aplica o gate de plataforma.
const accessReady = computed(() => accountStore.accountsLoaded)

function isSectionAllowed(section: SectionTab): boolean {
  if (section.gate !== 'platform' || !accessReady.value) {
    return true
  }
  return isPlatformAdmin.value
}

const visibleSections = computed(() => SECTIONS.filter(isSectionAllowed))

const active = ref<SectionId>('dados')
const togglingActive = ref(false)

// Defesa em profundidade: o painel so renderiza a secao ativa se ela esta na
// lista visivel (defesa alem do nav). Durante a hidratacao tudo e visivel, entao
// nao some o conteudo; resolvido, uma secao gateada nunca pinta o painel.
const isActiveVisible = computed(() =>
  visibleSections.value.some((section) => section.id === active.value),
)

// A secao ativa cai sempre na primeira visivel para o usuario. Quando a lista
// visivel muda (hidratacao das permissoes ou troca de escopo) e a atual deixa
// de ser permitida, reposiciona — nunca fixa em 'dados' se o usuario nao a ve.
watch(
  visibleSections,
  (sections) => {
    if (!sections.length) {
      return
    }
    if (!sections.some((section) => section.id === active.value)) {
      active.value = sections[0].id
    }
  },
  { immediate: true },
)

const isActive = computed(() => store.restaurant?.isActive ?? false)
const publicUrl = computed(() => (store.primaryDomain ? `https://${store.primaryDomain}` : ''))

async function onToggleActive() {
  if (togglingActive.value || !store.restaurantId) {
    return
  }
  togglingActive.value = true
  try {
    await store.patchRestaurant(store.restaurantId, { isActive: !isActive.value })
    ui.success(isActive.value ? 'Site publicado.' : 'Site despublicado.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel alterar o status.'))
  } finally {
    togglingActive.value = false
  }
}

function loadActive() {
  if (props.restaurantId) {
    void store.loadRestaurant(props.restaurantId, props.accountId)
  }
}

watch(() => props.restaurantId, loadActive)
onMounted(loadActive)
</script>

<template>
  <section class="cardapio-editor">
    <header class="cardapio-editor__top">
      <div class="cardapio-editor__crumbs">
        <NuxtLink to="/cardapio" class="cardapio-editor__back">Presence</NuxtLink>
        <span class="cardapio-editor__sep">/</span>
        <span class="cardapio-editor__current">
          {{ store.restaurant?.name || 'Carregando...' }}
        </span>
      </div>

      <div class="cardapio-editor__status">
        <CardapioEditorClientSelect
          :restaurant-id="props.restaurantId"
          :account-id="props.accountId"
        />
        <a
          v-if="publicUrl"
          :href="publicUrl"
          target="_blank"
          rel="noopener"
          class="cardapio-editor__link"
        >
          {{ store.primaryDomain }}
        </a>
        <span class="cardapio-editor__pill" :class="isActive ? 'is-on' : 'is-off'">
          {{ isActive ? 'Ativo' : 'Inativo' }}
        </span>
        <button
          type="button"
          class="cardapio-editor__toggle"
          :disabled="togglingActive || !store.restaurant"
          @click="onToggleActive"
        >
          <span v-if="togglingActive" class="cardapio-editor__spinner" aria-hidden="true"></span>
          {{ isActive ? 'Despublicar' : 'Publicar' }}
        </button>
      </div>
    </header>

    <p v-if="store.detailError" class="cardapio-editor__error">{{ store.detailError }}</p>

    <div v-if="store.detailPending && !store.restaurant" class="cardapio-editor__loading">
      Carregando estabelecimento...
    </div>

    <div v-else class="cardapio-editor__body">
      <nav class="cardapio-editor__nav" aria-label="Secoes do cardapio">
        <button
          v-for="section in visibleSections"
          :key="section.id"
          type="button"
          class="cardapio-editor__nav-item"
          :class="{ 'cardapio-editor__nav-item--active': active === section.id }"
          @click="active = section.id"
        >
          {{ section.label }}
        </button>
      </nav>

      <div v-if="isActiveVisible" class="cardapio-editor__panel">
        <CardapioSectionDados v-if="active === 'dados'" />
        <CardapioSectionCategorias v-else-if="active === 'categorias'" />
        <CardapioSectionProdutos v-else-if="active === 'produtos'" />
        <CardapioSectionAvaliacoes v-else-if="active === 'avaliacoes'" />
        <CardapioSectionPedidos v-else-if="active === 'pedidos'" />
        <CardapioSectionRelatorios v-else-if="active === 'relatorios'" />
        <CardapioSectionEntrega v-else-if="active === 'entrega'" />
        <CardapioSectionDominios v-else-if="active === 'dominios'" />
        <CardapioSectionAparencia v-else-if="active === 'aparencia'" />
        <CardapioSectionSite v-else-if="active === 'site'" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.cardapio-editor {
  /* Container de rolagem da pagina (flex:1; min-height:0; overflow-y:auto). O
     header e a sidebar grudam (sticky) DENTRO deste elemento; por isso ele nao
     pode ter overflow horizontal que quebre o sticky dos filhos. */
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
  padding: 1.5rem;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  /* Altura do header sticky: a sidebar gruda logo abaixo dele. */
  --cardapio-editor-header-offset: 3.75rem;
}

.cardapio-editor__top {
  /* FIXO no topo enquanto o painel rola. O fundo opaco + margem negativa que
     cobre o padding do container impedem o conteudo de aparecer por tras/ao lado
     ao rolar; z-index acima da sidebar e do painel. */
  position: sticky;
  top: 0;
  z-index: 3;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  margin: -1.5rem -1.5rem 0;
  padding: 1rem 1.5rem;
  background: rgb(var(--surface));
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-editor__crumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.95rem;
}

.cardapio-editor__back {
  color: rgb(var(--primary));
  font-weight: 600;
  text-decoration: none;
}

.cardapio-editor__sep {
  color: var(--text-muted);
}

.cardapio-editor__current {
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-editor__status {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.cardapio-editor__link {
  font-size: 0.85rem;
  color: rgb(var(--primary));
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  text-decoration: none;
}

.cardapio-editor__pill {
  padding: 0.2rem 0.65rem;
  border-radius: 999px;
  font-size: 0.78rem;
  font-weight: 600;
}

.cardapio-editor__pill.is-on {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.cardapio-editor__pill.is-off {
  background: rgb(var(--muted) / 0.16);
  color: var(--text-muted);
}

.cardapio-editor__toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.5rem 0.95rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  font-weight: 600;
  font-size: 0.87rem;
  cursor: pointer;
}

.cardapio-editor__toggle:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-editor__error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.14);
  padding: 0.6rem 0.85rem;
  border-radius: var(--radius-sm);
  font-size: 0.9rem;
}

.cardapio-editor__loading {
  color: var(--text-muted);
  padding: 1rem 0;
}

.cardapio-editor__body {
  display: grid;
  grid-template-columns: 188px minmax(0, 1fr);
  gap: 1.5rem;
  align-items: start;
  flex: 1;
  min-height: 0;
}

.cardapio-editor__nav {
  /* FIXA enquanto o painel rola. align-self: flex-start impede o item de
     esticar na altura do grid (sem isso o sticky nao teria folga para grudar).
     top abaixo do header sticky para nao sobrepor. */
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  position: sticky;
  top: var(--cardapio-editor-header-offset);
  align-self: flex-start;
  z-index: 1;
}

.cardapio-editor__nav-item {
  width: 100%;
  padding: 0.6rem 0.85rem;
  border-radius: var(--radius-sm);
  border: none;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.92rem;
  font-weight: 500;
  text-align: left;
  transition:
    background 0.12s ease,
    color 0.12s ease;
}

.cardapio-editor__nav-item:hover {
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
}

.cardapio-editor__nav-item--active {
  background: rgb(var(--primary) / 0.15);
  color: var(--text-main);
  font-weight: 600;
}

.cardapio-editor__panel {
  min-width: 0;
}

.cardapio-editor__spinner {
  width: 0.85rem;
  height: 0.85rem;
  border-radius: 999px;
  border: 2px solid rgb(var(--primary) / 0.35);
  border-top-color: rgb(var(--primary));
  animation: cardapio-editor-spin 0.7s linear infinite;
}

@keyframes cardapio-editor-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 880px) {
  .cardapio-editor__body {
    grid-template-columns: 1fr;
  }

  .cardapio-editor__nav {
    position: static;
    flex-direction: row;
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .cardapio-editor__nav-item {
    width: auto;
    white-space: nowrap;
  }
}
</style>
