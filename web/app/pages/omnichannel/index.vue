<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from "vue";
import OmnichannelInboxLoading from "~/components/omnichannel/OmnichannelInboxLoading.vue";
import OmnichannelConfigDrawer from "~/components/omnichannel/config/OmnichannelConfigDrawer.vue";
import AppPanelButton from "~/components/ui/AppPanelButton.vue";
import { useAuthStore } from "~/stores/auth";

const AsyncOmnichannelInboxModule = defineAsyncComponent({
  loader: () => import("~/components/omnichannel/OmnichannelInboxModule.vue"),
  loadingComponent: OmnichannelInboxLoading,
  delay: 120,
  suspensible: false,
  timeout: 20_000,
});

definePageMeta({
  layout: "dashboard",
  workspaceId: "omnichannel",
  pageLabel: "Omnichannel",
});

const auth = useAuthStore();
const route = useRoute();
const configOpen = ref(false);
const configTab = computed(() => String(route.query.config || ""));
const canViewInbox = computed(() =>
  auth.effectivePermissionKeys.includes("omnichannel.conversations.view"),
);
const canConfigure = computed(
  () =>
    [
      "omnichannel.instances.manage",
      "omnichannel.settings.manage",
      "omnichannel.agents.manage",
    ].some((key) => auth.effectivePermissionKeys.includes(key)),
);

watch(
  () => route.query.config,
  (value) => {
    if (value && canConfigure.value) configOpen.value = true;
  },
  { immediate: true },
);
</script>

<template>
  <section class="omnichannel-inbox-page">
    <OmnichannelConfigDrawer v-model:open="configOpen" :initial-tab="configTab" />

    <ClientOnly>
      <AsyncOmnichannelInboxModule v-if="canViewInbox" @configure="configOpen = true" />
      <div v-else class="omnichannel-inbox-page__forbidden">
        <strong>Inbox não disponível</strong>
        <span>Seu acesso atual não inclui a permissão para visualizar conversas.</span>
        <AppPanelButton v-if="canConfigure" variant="secondary" @click="configOpen = true">
          Abrir configurações permitidas
        </AppPanelButton>
      </div>
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

.omnichannel-inbox-page__forbidden {
  display: grid;
  place-content: center;
  gap: 0.6rem;
  min-height: 18rem;
  padding: 1.5rem;
  color: rgb(var(--muted));
  text-align: center;
}

.omnichannel-inbox-page__forbidden strong {
  color: rgb(var(--text));
}

.omnichannel-inbox-page__forbidden :deep(.app-panel-button) {
  justify-self: center;
}
</style>
