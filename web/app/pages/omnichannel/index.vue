<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from "vue";
import OmnichannelInboxLoading from "~/components/omnichannel/OmnichannelInboxLoading.vue";
import AppPanelButton from "~/components/ui/AppPanelButton.vue";
import OmnichannelConfigDrawer from "~/components/omnichannel/config/OmnichannelConfigDrawer.vue";
import { useAuthStore } from "~/stores/auth";

const AsyncOmnichannelInboxModule = defineAsyncComponent({
  loader: () => import("~/components/omnichannel/OmnichannelInboxModule.vue"),
  loadingComponent: OmnichannelInboxLoading,
  delay: 120,
  suspensible: false,
  timeout: 20_000
});

definePageMeta({
  // O legado usava layout 'admin', que nao existe aqui.
  layout: "dashboard",
  // Duas camadas de gating, iguais aos demais modulos (tasks/calendar):
  //  - workspaceId 'omnichannel' → gate de PAPEL no auth.global.ts.
  //  - MODULE_PATH_GUARDS (/omnichannel → 'omnichannel') no module-enabled.global.ts
  //    → gate de MODULO por conta (core.account_modules), espelhando o back.
  // O placeholder antigo usava workspaceId: '' (nunca-gated) — NAO herdar isso.
  workspaceId: "omnichannel",
  pageLabel: "Omnichannel"
});

const auth = useAuthStore();

// Badge de legado (principio 4) — visivel SO para quem administra.
// ARMADILHA: platform_admin tem has() = false no front, entao o gating precisa
// ser `isPlatformAdmin || has(...)`; so `has(...)` esconderia o aviso justamente
// de quem administra. `omnichannel.settings.manage` so passa a existir quando o
// modulo Go for registrado (F2) — ate la quem ve o badge e o platform_admin.
const isPlatformAdmin = computed(() => auth.role === "platform_admin");
const canSeeBackendBadge = computed(
  () =>
    isPlatformAdmin.value ||
    auth.effectivePermissionKeys.includes("omnichannel.settings.manage")
);

// F10 — telas de config. O botao/aba so aparece para quem administra: platform_admin
// (has()=false no front) OU quem tem alguma das 3 permissoes de gestao. Sem esse
// `isPlatformAdmin || ...` a config sumiria justamente do admin.
const CONFIG_PERMS = [
  "omnichannel.instances.manage",
  "omnichannel.settings.manage",
  "omnichannel.agents.manage"
];
const route = useRoute();
const configOpen = ref(false);
const canConfigure = computed(
  () =>
    isPlatformAdmin.value ||
    CONFIG_PERMS.some((key) => auth.effectivePermissionKeys.includes(key))
);
const canManageAutomation = computed(
  () =>
    isPlatformAdmin.value ||
    auth.effectivePermissionKeys.includes("omnichannel.settings.manage"),
);

// Deep-link ?config=<aba> abre o drawer direto (compartilhavel).
watch(
  () => route.query.config,
  (value) => {
    if (value && canConfigure.value) configOpen.value = true;
  },
  { immediate: true }
);
</script>

<template>
  <section class="omnichannel-inbox-page">
    <div v-if="canConfigure" class="omnichannel-inbox-page__toolbar">
      <NuxtLink
        v-if="canManageAutomation"
        to="/omnichannel/automacao"
        class="omnichannel-inbox-page__automation-link"
      >
        <UIcon name="i-lucide-bot" aria-hidden="true" />
        Automação IA
      </NuxtLink>
      <AppPanelButton variant="secondary" @click="configOpen = true">
        Configurar atendimento
      </AppPanelButton>
    </div>

    <OmnichannelConfigDrawer v-model:open="configOpen" />

    <!--
      PARCIAL (F2) — principio 4: o aviso de legado tem que ser VERDADEIRO, nao
      decorativo. Aviso que mente na direcao oposta ("nao pronto" sobre dado real)
      treina quem administra a ignorar o badge — que e justamente o mecanismo que
      protege contra tomar mock por pronto.
      Estado real: o piloto P0 (F1-F10 + F13) esta em codigo e no ar — leitura,
      canal, tempo real, envio e acoes FUNCIONAM. O pareamento REAL do WhatsApp
      via Evolution conecta LOCAL e na VPS (config CONFIG_SESSION_PHONE_VERSION +
      imagem v2.3.7). Atualizar este aviso a cada fase.
      Registrado em docs/LEGADO.md e docs/omnichannel/ESTADO.md.
    -->
    <div v-if="canSeeBackendBadge" class="omnichannel-inbox-page__legacy-notice">
      <span class="omnichannel-inbox-page__legacy-tag">PILOTO</span>
      <span>Atendimento WhatsApp em piloto — pareamento real via Evolution.</span>
    </div>

    <ClientOnly>
      <AsyncOmnichannelInboxModule />
      <template #fallback>
        <OmnichannelInboxLoading />
      </template>
    </ClientOnly>
  </section>
</template>

<style scoped>
.omnichannel-inbox-page {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: calc(100vh - 6.5rem);
}

.omnichannel-inbox-page__toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.omnichannel-inbox-page__automation-link {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  min-height: 36px;
  padding: 0 0.85rem;
  border-radius: 14px;
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
  font-size: 0.8rem;
  font-weight: 700;
  text-decoration: none;
}

/* Tokens semanticos da casa (tokens.css) — nada de cor cravada, senao o aviso
   nao acompanha a troca de tema. */
.omnichannel-inbox-page__legacy-notice {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
  margin-bottom: 0.5rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid color-mix(in srgb, var(--accent-warning) 45%, transparent);
  border-radius: var(--radius-card);
  background: color-mix(in srgb, var(--accent-warning) 14%, transparent);
  color: var(--text-main);
  font-size: 0.8125rem;
  line-height: 1.35;
}

.omnichannel-inbox-page__legacy-tag {
  flex: none;
  padding: 0.125rem 0.375rem;
  border-radius: var(--radius-soft);
  background: var(--accent-warning);
  color: var(--text-inverse);
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.04em;
}
</style>
