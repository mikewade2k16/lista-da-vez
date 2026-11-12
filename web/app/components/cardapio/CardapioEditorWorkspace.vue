<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  CARDAPIO_SECTIONS,
  resolveCardapioSection,
  cardapioSectionLocation,
  type SectionTab,
} from '~/domain/cardapio/editor-sections'

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
import CardapioLargePhotosNotice from '~/components/cardapio/product/CardapioLargePhotosNotice.vue'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../../layers/core/stores/account'
import { getApiErrorMessage } from '~/utils/api-client'

const props = defineProps<{ restaurantId: string; accountId?: string }>()

const route = useRoute()
const store = useCardapioStore()
const ui = useUiStore()
const auth = useAuthStore()
const accountStore = useCoreAccountStore()

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

const visibleSections = computed(() => CARDAPIO_SECTIONS.filter(isSectionAllowed))

// Todas as abas liberadas ficam visiveis (sem menu "Mais"): primeiro as do dia a
// dia, depois as demais na ordem de CARDAPIO_SECTIONS.
const PRIMARY_SECTION_ORDER = ['produtos', 'categorias', 'pedidos']
const primarySections = computed(() => [
  ...PRIMARY_SECTION_ORDER.map((id) =>
    visibleSections.value.find((section) => section.id === id),
  ).filter((section): section is SectionTab => Boolean(section)),
  ...visibleSections.value.filter((section) => !PRIMARY_SECTION_ORDER.includes(section.id)),
])

// Quem so tem um estabelecimento entra direto nele (a lista redireciona), entao
// o atalho de volta para a lista some para esse usuario.
const showListLink = computed(() => isPlatformAdmin.value || store.restaurants.length !== 1)

const active = computed(() =>
  resolveCardapioSection(
    route.query.aba,
    visibleSections.value.map((section) => section.id),
  ),
)
const togglingActive = ref(false)

// Defesa em profundidade: o painel so renderiza a secao ativa se ela esta na
// lista visivel (defesa alem do nav). Durante a hidratacao tudo e visivel, entao
// nao some o conteudo; resolvido, uma secao gateada nunca pinta o painel.
const isActiveVisible = computed(() =>
  visibleSections.value.some((section) => section.id === active.value),
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

watch(() => [props.restaurantId, props.accountId], loadActive)
onMounted(loadActive)
</script>

<template>
  <section class="cardapio-editor">
    <header class="cardapio-editor__top">
      <div class="cardapio-editor__crumbs">
        <template v-if="showListLink">
          <NuxtLink
            to="/cardapio"
            class="cardapio-editor__back"
            title="Voltar para Presence"
            aria-label="Voltar para Presence"
          >
            <UIcon name="i-lucide-arrow-left" class="cardapio-editor__back-icon" />
            <span class="cardapio-editor__back-label">Presence</span>
          </NuxtLink>
          <span class="cardapio-editor__sep">/</span>
        </template>
        <span class="cardapio-editor__current">
          {{ store.restaurant?.name || 'Carregando...' }}
        </span>
        <span
          v-if="store.restaurant && !isActive"
          class="cardapio-editor__offline"
          title="Site fora do ar"
        >
          Fora do ar
        </span>
      </div>

      <nav
        v-if="store.restaurant || !store.detailPending"
        class="cardapio-editor__tabs"
        aria-label="Secoes do cardapio"
      >
        <NuxtLink
          v-for="section in primarySections"
          :key="section.id"
          :to="cardapioSectionLocation(route, section.id)"
          :aria-current="active === section.id ? 'page' : undefined"
          class="cardapio-editor__tab"
          :class="{ 'is-active': active === section.id }"
          :title="section.label"
          :aria-label="section.label"
        >
          <UIcon :name="section.icon" class="cardapio-editor__tab-icon" />
          <span class="cardapio-editor__tab-label">{{ section.label }}</span>
        </NuxtLink>
      </nav>

      <div class="cardapio-editor__actions">
        <CardapioLargePhotosNotice v-if="isPlatformAdmin && store.restaurantId" />
        <UButton
          v-if="publicUrl"
          :to="publicUrl"
          target="_blank"
          icon="i-lucide-external-link"
          color="neutral"
          variant="ghost"
          size="sm"
          :title="`Abrir ${store.primaryDomain}`"
          :aria-label="`Abrir o site ${store.primaryDomain}`"
        />
        <UPopover :content="{ align: 'end' }">
          <UButton
            icon="i-lucide-ellipsis"
            color="neutral"
            variant="ghost"
            size="sm"
            title="Publicacao e configuracoes do site"
            aria-label="Publicacao e configuracoes do site"
          />
          <template #content>
            <div class="cardapio-editor__menu">
              <div class="cardapio-editor__menu-row">
                <span class="cardapio-editor__pill" :class="isActive ? 'is-on' : 'is-off'">
                  {{ isActive ? 'Publicado' : 'Fora do ar' }}
                </span>
                <button
                  type="button"
                  class="cardapio-editor__toggle"
                  :disabled="togglingActive || !store.restaurant"
                  @click="onToggleActive"
                >
                  <span
                    v-if="togglingActive"
                    class="cardapio-editor__spinner"
                    aria-hidden="true"
                  ></span>
                  {{ isActive ? 'Despublicar' : 'Publicar' }}
                </button>
              </div>
              <a
                v-if="publicUrl"
                :href="publicUrl"
                target="_blank"
                rel="noopener"
                class="cardapio-editor__link"
              >
                {{ store.primaryDomain }}
              </a>
              <CardapioEditorClientSelect
                :restaurant-id="props.restaurantId"
                :account-id="props.accountId"
              />
            </div>
          </template>
        </UPopover>
      </div>
    </header>

    <p v-if="store.detailError" class="cardapio-editor__error">{{ store.detailError }}</p>

    <div v-if="store.detailPending && !store.restaurant" class="cardapio-editor__loading">
      Carregando estabelecimento...
    </div>

    <div
      v-else-if="isActiveVisible"
      class="cardapio-editor__panel"
      :class="{ 'cardapio-editor__panel--fill': active === 'produtos' }"
    >
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
  </section>
</template>

<style scoped>
.cardapio-editor {
  /* Container de rolagem da pagina. O header (com as abas) gruda no topo. */
  display: flex;
  flex-direction: column;
  /* Sem gap: o respiro abaixo do topo fica no painel (a aba Produtos o zera). */
  gap: 0;
  padding: 0 1.5rem 0.75rem;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

/* Filhos da coluna com rolagem nunca encolhem: com abas em overflow-x o minimo
   automatico vira 0 e o header/painel se sobrepunham (visto no mobile). */
.cardapio-editor > * {
  flex-shrink: 0;
}

.cardapio-editor__top {
  /* Barra unica e compacta: nome | abas | acoes. Sticky no topo da area de
     rolagem (sem margem negativa no topo, que empurrava o sticky para baixo). */
  position: sticky;
  top: 0;
  z-index: 3;
  display: flex;
  align-items: center;
  gap: 1.25rem;
  margin: 0 -1.5rem;
  padding: 0.45rem 1.5rem;
  min-height: 3.25rem;
  background: rgb(var(--surface));
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-editor__crumbs {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
  font-size: 0.92rem;
  white-space: nowrap;
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
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-editor__offline {
  padding: 0.1rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--danger) / 0.14);
  color: rgb(var(--danger));
  font-size: 0.72rem;
  font-weight: 600;
}

.cardapio-editor__tabs {
  display: flex;
  align-items: center;
  gap: 0.15rem;
  flex: 1;
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: none;
}

.cardapio-editor__tab {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  flex-shrink: 0;
  padding: 0.4rem 0.75rem;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-muted);
  font-size: 0.88rem;
  font-weight: 500;
  text-decoration: none;
  white-space: nowrap;
  cursor: pointer;
  transition:
    background 0.12s ease,
    color 0.12s ease;
}

.cardapio-editor__tab:hover {
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
}

.cardapio-editor__tab.is-active {
  background: rgb(var(--primary) / 0.15);
  color: var(--text-main);
  font-weight: 600;
}

.cardapio-editor__actions {
  display: flex;
  align-items: center;
  gap: 0.15rem;
  margin-left: auto;
}

.cardapio-editor__menu {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-width: 15rem;
  padding: 0.85rem;
}

.cardapio-editor__menu-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.cardapio-editor__link {
  font-size: 0.82rem;
  color: rgb(var(--primary));
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  text-decoration: none;
  word-break: break-all;
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
  background: rgb(var(--danger) / 0.14);
  color: rgb(var(--danger));
}

.cardapio-editor__toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.4rem 0.8rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  font-weight: 600;
  font-size: 0.84rem;
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

.cardapio-editor__panel {
  min-width: 0;
  padding-top: 1rem;
}

/* Produtos: filtros e tabela colados no topo, sem faixas vazias entre eles. */
.cardapio-editor__panel--fill {
  padding-top: 0;
}

/* Produtos (desktop): o painel ocupa toda a altura restante e a tabela rola por
   dentro, sem faixa vazia embaixo. Declarado depois de `.cardapio-editor > *`
   para o flex-shrink valer. No mobile os cards rolam com a pagina. */
@media (min-width: 721px) {
  .cardapio-editor__panel--fill {
    display: flex;
    flex-direction: column;
    flex: 1 1 0;
    min-height: 0;
    /* tabela ate o fim da pagina: anula o respiro inferior do editor */
    margin-bottom: -0.75rem;
  }
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
</style>
<style scoped src="./cardapio-editor-responsive.css"></style>
