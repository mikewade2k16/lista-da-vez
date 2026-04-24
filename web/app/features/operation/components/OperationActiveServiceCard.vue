<script setup>
import { computed } from "vue";
import { formatClock, formatDuration } from "~/domain/utils/time";

const props = defineProps({
  service: {
    type: Object,
    required: true
  },
  now: {
    type: Number,
    default: 0
  },
  clockReady: {
    type: Boolean,
    default: false
  },
  readOnly: {
    type: Boolean,
    default: false
  },
  integratedMode: {
    type: Boolean,
    default: false
  }
});

const emit = defineEmits(["finish"]);

const startedAtLabel = computed(() => formatClock(props.service?.serviceStartedAt || 0));
const timerLabel = computed(() =>
  props.clockReady ? formatDuration(Math.max(0, props.now - (props.service?.serviceStartedAt || 0))) : "--:--"
);
const serviceStatusLabel = computed(() => {
  const skippedCount = props.service?.skippedPeople?.length || 0;
  const typeLabel = props.service?.startMode === "queue-jump" ? "Fora da vez" : "Na vez";

  return skippedCount > 0
    ? `${typeLabel}, passou ${skippedCount} ${skippedCount === 1 ? "pessoa" : "pessoas"}`
    : typeLabel;
});

function handleFinish() {
  emit("finish", props.service?.id || "");
}
</script>

<template>
  <article
    class="service-card"
    :data-testid="`operation-service-${service.id}`"
  >
    <div class="service-card__meta-row">
      <span class="queue-card__note">Iniciado as {{ startedAtLabel }}</span>
      <strong class="service-card__timer">{{ timerLabel }}</strong>
    </div>

    <div class="service-card__body">
      <span class="queue-card__avatar queue-card__avatar--large" :style="{ '--avatar-accent': service.color }">
        {{ service.initials }}
      </span>

      <div class="service-card__content">
        <span class="queue-card__headline">
          <strong class="queue-card__name">{{ service.name }}</strong>
          <span v-if="integratedMode && service.storeName" class="queue-card__store-badge">{{ service.storeName }}</span>
        </span>
        <span class="queue-card__role">{{ service.role }}</span>
        <span class="queue-card__note">{{ serviceStatusLabel }}</span>
      </div>
    </div>

    <button
      v-if="!readOnly && !integratedMode"
      class="column-action column-action--secondary service-card__action"
      type="button"
      :data-testid="`operation-finish-${service.id}`"
      @click="handleFinish"
    >
      Encerrar atendimento
    </button>
  </article>
</template>
