<script setup>
import { onBeforeUnmount, onMounted } from "vue";
import { X } from "lucide-vue-next";

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  title: {
    type: String,
    default: "Detalhes"
  },
  subtitle: {
    type: String,
    default: ""
  },
  sections: {
    type: Array,
    default: () => []
  },
  width: {
    type: String,
    default: "min(42rem, calc(100vw - 2rem))"
  }
});

const emit = defineEmits(["update:modelValue"]);

function closeDialog() {
  emit("update:modelValue", false);
}

function formatValue(value) {
  if (Array.isArray(value)) {
    return value.filter(Boolean).join(", ") || "-";
  }

  if (value === null || value === undefined || String(value).trim() === "") {
    return "-";
  }

  return String(value);
}

function handleEscape(event) {
  if (event.key === "Escape" && props.modelValue) {
    closeDialog();
  }
}

onMounted(() => {
  document.addEventListener("keydown", handleEscape);
});

onBeforeUnmount(() => {
  document.removeEventListener("keydown", handleEscape);
});
</script>

<template>
  <Teleport to="body">
    <div v-if="modelValue" class="app-detail-dialog">
      <button class="app-detail-dialog__scrim" type="button" aria-label="Fechar detalhes" @click="closeDialog" />

      <section class="app-detail-dialog__card" :style="{ width }">
        <header class="app-detail-dialog__header">
          <div class="app-detail-dialog__copy">
            <p class="app-detail-dialog__eyebrow">Detalhes</p>
            <h3 class="app-detail-dialog__title">{{ title }}</h3>
            <p v-if="subtitle" class="app-detail-dialog__subtitle">{{ subtitle }}</p>
          </div>

          <button class="app-detail-dialog__close" type="button" aria-label="Fechar" @click="closeDialog">
            <X :size="18" :stroke-width="2.1" />
          </button>
        </header>

        <div class="app-detail-dialog__body">
          <section
            v-for="section in sections"
            :key="section?.id || section?.title || JSON.stringify(section)"
            class="app-detail-dialog__section"
          >
            <header v-if="section?.title" class="app-detail-dialog__section-header">
              <h4>{{ section.title }}</h4>
              <p v-if="section?.description">{{ section.description }}</p>
            </header>

            <div class="app-detail-dialog__fields">
              <article
                v-for="field in section?.fields || []"
                :key="`${section?.title || 'section'}-${field?.label || field?.key || 'field'}`"
                class="app-detail-dialog__field"
              >
                <span class="app-detail-dialog__field-label">{{ field?.label || field?.key }}</span>
                <strong class="app-detail-dialog__field-value">{{ formatValue(field?.value) }}</strong>
              </article>
            </div>
          </section>

          <slot />
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.app-detail-dialog {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: 1rem;
}

.app-detail-dialog__scrim {
  position: absolute;
  inset: 0;
  border: none;
  background: rgba(3, 6, 12, 0.76);
  backdrop-filter: blur(4px);
}

.app-detail-dialog__card {
  position: relative;
  z-index: 1;
  max-height: calc(100vh - 2rem);
  overflow: auto;
  border-radius: 1.2rem;
  border: 1px solid var(--line-soft);
  background: linear-gradient(180deg, rgba(13, 18, 29, 0.98), rgba(8, 12, 19, 0.98));
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.42);
}

.app-detail-dialog__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.2rem 1.2rem 0.9rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.app-detail-dialog__copy {
  display: grid;
  gap: 0.3rem;
}

.app-detail-dialog__eyebrow {
  margin: 0;
  font-size: 0.72rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: rgba(148, 163, 184, 0.82);
}

.app-detail-dialog__title {
  margin: 0;
  color: #ffffff;
  font-size: 1.15rem;
}

.app-detail-dialog__subtitle {
  margin: 0;
  color: var(--text-muted);
  line-height: 1.45;
}

.app-detail-dialog__close {
  width: 2.45rem;
  height: 2.45rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-main);
  cursor: pointer;
}

.app-detail-dialog__body {
  display: grid;
  gap: 1rem;
  padding: 1.2rem;
}

.app-detail-dialog__section {
  display: grid;
  gap: 0.8rem;
}

.app-detail-dialog__section-header {
  display: grid;
  gap: 0.2rem;
}

.app-detail-dialog__section-header h4 {
  margin: 0;
  font-size: 0.85rem;
  color: #ffffff;
}

.app-detail-dialog__section-header p {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.82rem;
}

.app-detail-dialog__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  gap: 0.75rem;
}

.app-detail-dialog__field {
  display: grid;
  gap: 0.28rem;
  padding: 0.85rem 0.9rem;
  border-radius: 0.9rem;
  border: 1px solid rgba(255, 255, 255, 0.05);
  background: rgba(18, 25, 38, 0.8);
}

.app-detail-dialog__field-label {
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgba(148, 163, 184, 0.82);
}

.app-detail-dialog__field-value {
  color: var(--text-main);
  line-height: 1.45;
  word-break: break-word;
}

@media (max-width: 720px) {
  .app-detail-dialog__card {
    width: calc(100vw - 1rem) !important;
  }

  .app-detail-dialog__fields {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>