<script setup lang="ts">
import { ref } from "vue";
import SettingsTabs from "~/components/settings/SettingsTabs.vue";
import RoadmapTimeline from "~/components/roadmap/RoadmapTimeline.vue";
import RoadmapDatabaseSchema from "~/components/roadmap/RoadmapDatabaseSchema.vue";
import { ROADMAP_SUBTITLE, ROADMAP_TITLE } from "~/components/roadmap/roadmap-data";

const tabs = [
  { id: "timeline", label: "Roadmap", icon: "timeline" },
  { id: "database", label: "Banco", icon: "database" }
];

const activeTab = ref<string>("timeline");
</script>

<template>
  <div class="roadmap-workspace">
    <header class="roadmap-workspace__header">
      <h2 class="roadmap-workspace__title">{{ ROADMAP_TITLE }}</h2>
      <p class="roadmap-workspace__text">{{ ROADMAP_SUBTITLE }}</p>
    </header>

    <SettingsTabs :tabs="tabs" :active-tab="activeTab" @update:active-tab="activeTab = $event" />

    <RoadmapTimeline v-if="activeTab === 'timeline'" />
    <RoadmapDatabaseSchema v-else-if="activeTab === 'database'" />
  </div>
</template>

<style scoped>
.roadmap-workspace {
  display: grid;
  align-content: start;
  gap: 1.4rem;
  padding: 1.2rem;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}

.roadmap-workspace__header {
  display: grid;
  gap: 0.35rem;
}

.roadmap-workspace__title {
  margin: 0;
  font-size: 1.45rem;
  color: var(--text-main);
}

.roadmap-workspace__text {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.9rem;
  line-height: 1.5;
  max-width: 880px;
}
</style>
