<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import DashboardHeaderNavMore from '~/components/dashboard/DashboardHeaderNavMore.vue'
import { useDashboardNav } from '~/composables/useDashboardNav'
import { useHeaderNavOverflow } from '~/composables/useHeaderNavOverflow'

const props = defineProps<{
  activeWorkspace?: string
  allowedWorkspaces?: readonly unknown[]
}>()

const route = useRoute()
const navRef = ref<HTMLElement | null>(null)

// Dropdowns dos grupos (Tools/Site/Manage): CLICK-controlados (id aberto). Fecham
// no clique-fora, no Esc e ao escolher um item — regra de dropdown do AGENT_RULES.
const openNavId = ref('')
// hoverSuppressed: id do dropdown cujo HOVER esta temporariamente suprimido. Apos
// clicar num item que navega, o ponteiro pode continuar sobre o dropdown e o
// :hover reabriria o popover; suprimimos ate o mouse sair (sem remover o hover).
const hoverSuppressed = ref('')

const { headerItems, resolveIcon, isItemActive, isGroupActive } = useDashboardNav(
  computed(() => props.activeWorkspace || ''),
  computed(() => props.allowedWorkspaces || []),
)

const { visibleHeaderItems, overflowHeaderItems, hasOverflow, setItemEl } = useHeaderNavOverflow(
  navRef,
  headerItems,
)

function toggleNav(id: string) {
  openNavId.value = openNavId.value === id ? '' : id
}

function closeNav() {
  openNavId.value = ''
}

function onNavItemClick(id: string) {
  closeNav()
  hoverSuppressed.value = id
}

function onDropdownLeave(id: string) {
  if (hoverSuppressed.value === id) hoverSuppressed.value = ''
}

// Clique-fora fecha o grupo aberto; o "Mais" cuida do proprio dismiss.
function handlePointerDown(event: PointerEvent) {
  const target = event.target as Node | null
  if (openNavId.value && navRef.value && target && !navRef.value.contains(target)) closeNav()
}

function handleEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') closeNav()
}

watch(
  () => route.fullPath,
  () => closeNav(),
)

onMounted(() => {
  document.addEventListener('pointerdown', handlePointerDown)
  document.addEventListener('keydown', handleEscape)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handlePointerDown)
  document.removeEventListener('keydown', handleEscape)
})
</script>

<template>
  <nav ref="navRef" class="dashboard-header__nav" aria-label="Menu principal">
    <template v-for="item in visibleHeaderItems" :key="item.id">
      <div
        v-if="item.children"
        class="dashboard-header__nav-dropdown"
        :class="{
          'is-open': openNavId === item.id,
          'is-suppressed': hoverSuppressed === item.id,
        }"
        @mouseleave="onDropdownLeave(item.id)"
      >
        <button
          class="dashboard-header__nav-link"
          :class="{ 'is-active': isGroupActive(item) }"
          type="button"
          :aria-expanded="openNavId === item.id ? 'true' : 'false'"
          aria-haspopup="true"
          @click="toggleNav(item.id)"
        >
          <component
            :is="resolveIcon(item.icon)"
            class="dashboard-header__nav-icon"
            :size="16"
            :stroke-width="2.15"
            aria-hidden="true"
          />
          <span>{{ item.label }}</span>
          <ChevronDown
            class="dashboard-header__nav-chevron"
            :size="14"
            :stroke-width="2.25"
            aria-hidden="true"
          />
        </button>

        <div class="dashboard-header__nav-popover">
          <NuxtLink
            v-for="child in item.children"
            :key="child.id"
            :to="child.path"
            class="dashboard-header__nav-popover-item"
            :class="{ 'is-active': isItemActive(child) }"
            @click="onNavItemClick(item.id)"
          >
            <component
              :is="resolveIcon(child.icon)"
              class="dashboard-header__nav-popover-icon"
              :size="16"
              :stroke-width="2.1"
              aria-hidden="true"
            />
            <span>{{ child.label }}</span>
          </NuxtLink>
        </div>
      </div>

      <NuxtLink
        v-else
        :to="item.path"
        class="dashboard-header__nav-link"
        :class="{ 'is-active': isItemActive(item) }"
      >
        <component
          :is="resolveIcon(item.icon)"
          class="dashboard-header__nav-icon"
          :size="16"
          :stroke-width="2.15"
          aria-hidden="true"
        />
        <span>{{ item.label }}</span>
      </NuxtLink>
    </template>

    <DashboardHeaderNavMore
      v-if="hasOverflow"
      :items="overflowHeaderItems"
      :resolve-icon="resolveIcon"
      :is-item-active="isItemActive"
    />

    <div class="dashboard-header__measure" aria-hidden="true">
      <span
        v-for="item in headerItems"
        :key="item.id"
        :ref="(el) => setItemEl(item.id, el)"
        class="dashboard-header__nav-link dashboard-header__measure-item"
      >
        <component
          :is="resolveIcon(item.icon)"
          class="dashboard-header__nav-icon"
          :size="16"
          :stroke-width="2.15"
        />
        <span>{{ item.label }}</span>
        <ChevronDown v-if="item.children" class="dashboard-header__nav-chevron" :size="14" />
      </span>
    </div>
  </nav>
</template>

<style scoped>
.dashboard-header__nav {
  position: relative;
  min-width: 0;
  flex: 1 1 auto;
  display: flex;
  align-items: center;
  gap: 0.24rem;
  overflow: visible;
  scrollbar-width: none;
}

.dashboard-header__nav::-webkit-scrollbar {
  display: none;
}

.dashboard-header__nav-link {
  appearance: none;
  position: relative;
  min-height: 2.45rem;
  display: inline-flex;
  align-items: center;
  gap: 0.42rem;
  flex: 0 0 auto;
  border: 0;
  border-radius: var(--radius-sm);
  padding: 0 0.65rem;
  background: transparent;
  box-shadow: none;
  color: var(--admin-header-muted);
  font-size: 0.82rem;
  font-weight: 800;
  text-decoration: none;
  white-space: nowrap;
  cursor: pointer;
  transition: color 0.16s ease;
}

.dashboard-header__nav-link:hover,
.dashboard-header__nav-link.is-active {
  background: transparent;
  box-shadow: none;
  color: var(--admin-header-text);
}

.dashboard-header__nav-link::after {
  content: '';
  position: absolute;
  inset-inline: 0.6rem;
  bottom: 0.18rem;
  height: 2px;
  border-radius: 999px;
  background: rgb(var(--primary));
  transform: scaleX(0);
  transform-origin: left center;
  transition: transform 0.18s ease;
}

.dashboard-header__nav-link:hover::after,
.dashboard-header__nav-link:focus-visible::after,
.dashboard-header__nav-link.is-active::after,
.dashboard-header__nav-dropdown:hover .dashboard-header__nav-link::after,
.dashboard-header__nav-dropdown.is-open .dashboard-header__nav-link::after {
  transform: scaleX(1);
}

.dashboard-header__nav-icon,
.dashboard-header__nav-chevron {
  flex-shrink: 0;
}

.dashboard-header__nav-chevron {
  width: 0.86rem;
  height: 0.86rem;
  color: var(--admin-header-muted);
  transition:
    transform 0.16s ease,
    color 0.16s ease;
}

.dashboard-header__nav-dropdown {
  position: relative;
  flex: 0 0 auto;
  padding-block: 0.4rem;
}

/* Abre no HOVER (comportamento original) E no clique (.is-open, que "fixa"
   aberto). As duas funcionalidades coexistem: o hover abre ao passar o mouse; o
   clique fixa e o clique-fora/Esc/opcao fecham (handlers em JS). Sem
   :focus-within (deixava preso aberto apos clicar e nao fechava no clique-fora). */
.dashboard-header__nav-dropdown:hover .dashboard-header__nav-link,
.dashboard-header__nav-dropdown.is-open .dashboard-header__nav-link {
  background: transparent;
  box-shadow: none;
  color: var(--admin-header-text);
}

.dashboard-header__nav-dropdown:hover .dashboard-header__nav-chevron,
.dashboard-header__nav-dropdown.is-open .dashboard-header__nav-chevron {
  transform: rotate(180deg);
  color: rgb(var(--primary));
}

.dashboard-header__nav-dropdown:hover .dashboard-header__nav-popover,
.dashboard-header__nav-dropdown.is-open .dashboard-header__nav-popover {
  opacity: 1;
  visibility: visible;
  transform: translateY(0);
  pointer-events: auto;
}

/* Apos clicar num item (navegacao), segura o hover fechado ate o mouse sair —
   senao o :hover reabriria o popover com o ponteiro ainda sobre o dropdown.
   Especificidade maior (.is-suppressed:hover) vence a regra de :hover acima. */
.dashboard-header__nav-dropdown.is-suppressed:hover .dashboard-header__nav-popover {
  opacity: 0;
  visibility: hidden;
  transform: translateY(-0.35rem);
  pointer-events: none;
}

.dashboard-header__nav-dropdown.is-suppressed:hover .dashboard-header__nav-chevron {
  transform: none;
  color: var(--admin-header-muted);
}

.dashboard-header__nav-popover {
  position: absolute;
  top: calc(100% - 0.3rem);
  left: 0;
  z-index: 9600;
  min-width: 13rem;
  display: grid;
  gap: 0.2rem;
  border: 1px solid var(--admin-header-border);
  border-radius: var(--radius-sm);
  background: var(--admin-header-panel-bg);
  box-shadow: var(--shadow-md);
  padding: 0.35rem;
  backdrop-filter: blur(var(--admin-header-panel-blur));
  opacity: 0;
  visibility: hidden;
  transform: translateY(-0.35rem);
  pointer-events: none;
  transition:
    opacity 0.14s ease,
    transform 0.14s ease,
    visibility 0.14s ease;
}

.dashboard-header__nav-popover-item {
  min-height: 2.2rem;
  display: flex;
  align-items: center;
  gap: 0.55rem;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  padding: 0 0.65rem;
  color: var(--admin-header-text);
  font-size: 0.82rem;
  font-weight: 750;
  text-decoration: none;
}

.dashboard-header__nav-popover-item:hover,
.dashboard-header__nav-popover-item.is-active {
  border-color: rgb(var(--ring) / 0.22);
  background: var(--admin-header-hover-bg);
}

.dashboard-header__nav-popover-icon {
  flex-shrink: 0;
  color: var(--admin-header-muted);
}

.dashboard-header__nav-popover-item:hover .dashboard-header__nav-popover-icon,
.dashboard-header__nav-popover-item.is-active .dashboard-header__nav-popover-icon {
  color: rgb(var(--primary));
}

/* Faixa de medicao oculta: renderiza todos os itens top-level fora de tela para
   medir a largura de cada um sem afetar o layout visivel. */
.dashboard-header__measure {
  position: absolute;
  top: 0;
  left: 0;
  display: flex;
  align-items: center;
  gap: 0.24rem;
  visibility: hidden;
  pointer-events: none;
  white-space: nowrap;
}

.dashboard-header__measure-item {
  cursor: default;
}

@media (max-width: 900px) {
  .dashboard-header__nav {
    min-height: 2.6rem;
    overflow-x: auto;
    overflow-y: visible;
  }
}
</style>
