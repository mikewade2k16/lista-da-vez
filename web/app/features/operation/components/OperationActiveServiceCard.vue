<script setup>
import { computed } from "vue";
import { buildNickname } from "~/domain/utils/person-display";
import { formatClock, formatDuration } from "~/domain/utils/time";

const props = defineProps({
  services: {
    type: Array,
    required: true
  },
  now: {
    type: Number,
    default: 0
  },
  serverClockOffsetMs: {
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
  },
  maxConcurrentPerConsultant: {
    type: Number,
    default: 1
  }
});

const emit = defineEmits(["finish", "startParallel"]);

const primaryService = computed(() => props.services?.[0] || null);
const totalOpenServices = computed(() => props.services?.length || 0);
const consultantDisplayName = computed(() => buildNickname(primaryService.value?.name || ""));
const primaryStatusLabel = computed(() => compactStatusLabel(primaryService.value));
const serviceColumns = computed(() => Math.min(3, Math.max(1, totalOpenServices.value)));
const canStartParallel = computed(() => {
  return !props.readOnly && !props.integratedMode && totalOpenServices.value < props.maxConcurrentPerConsultant;
});

function startedAtLabel(service) {
  return formatClock(Math.max(0, Number(service?.serviceStartedAt || 0) - Number(props.serverClockOffsetMs || 0)));
}

function isPrimaryService(service) {
  return String(service?.serviceId || "").trim() === String(primaryService.value?.serviceId || "").trim();
}

function isFrozenTimer(service) {
  return Number(service?.effectiveFinishedAt || 0) > 0;
}

function timerLabel(service) {
  if (!props.clockReady) {
    return "--:--";
  }

  return formatDuration(
    Math.max(
      0,
      isFrozenTimer(service)
        ? Number(service?.effectiveFinishedAt || 0) - Number(service?.serviceStartedAt || 0)
        : props.now - Number(service?.serviceStartedAt || 0)
    ),
    { roundUpPartialSecond: !isFrozenTimer(service) }
  );
}

function compactStatusLabel(service) {
  const skippedCount = service?.skippedPeople?.length || 0;
  let typeLabel = "Na vez";

  if (service?.startMode === "queue-jump") {
    typeLabel = "Fora da vez";
  } else if (service?.startMode === "parallel") {
    typeLabel = "Na sequencia";
  }

  return skippedCount > 0
    ? `${typeLabel}, passou ${skippedCount} ${skippedCount === 1 ? "pessoa" : "pessoas"}`
    : typeLabel;
}

const parallelBadge = computed(() => {
  if (totalOpenServices.value <= 1) {
    return null;
  }
  return `${totalOpenServices.value}/${props.maxConcurrentPerConsultant} em aberto`;
});

function serviceMetaLabel(service) {
  const startedLabel = `Iniciado às ${startedAtLabel(service)}`;

  if (isPrimaryService(service)) {
    return startedLabel;
  }

  return `${compactStatusLabel(service)} · ${startedLabel}`;
}

function handleFinish(serviceId) {
  emit("finish", serviceId || "");
}

function handleStartParallel() {
  emit("startParallel", primaryService.value?.id || "");
}
</script>

<template>
  <article
    class="service-card"
    :data-testid="`operation-service-group-${primaryService?.id || 'consultant'}`"
  >
    <div class="service-card__summary">
      <span class="queue-card__avatar queue-card__avatar--large" :style="{ '--avatar-accent': primaryService?.color }">
        {{ primaryService?.initials }}
      </span>

      <div class="service-card__content">
        <span class="queue-card__headline">
          <strong class="queue-card__name">{{ consultantDisplayName }}</strong>
          <span v-if="primaryStatusLabel" class="service-card__status-badge">{{ primaryStatusLabel }}</span>
          <span v-if="integratedMode && primaryService?.storeName" class="queue-card__store-badge">{{ primaryService.storeName }}</span>
        </span>
      </div>

      <span v-if="parallelBadge" class="service-card__badge">{{ parallelBadge }}</span>
    </div>

    <div class="service-card__services" :style="{ '--service-columns': serviceColumns }">
      <div
        v-for="(service, index) in services"
        :key="service.serviceId"
        class="service-card__service-row"
      >
        <div class="service-card__service-main">
          <div class="service-card__meta-row">
            <span class="queue-card__note">{{ serviceMetaLabel(service) }}</span>
            <strong class="service-card__timer">{{ timerLabel(service) }}</strong>
          </div>
        </div>

        <div v-if="!readOnly && !integratedMode" class="service-card__row-actions">
          <button
            class="column-action column-action--secondary service-card__action"
            :class="{ 'service-card__action--full': !(canStartParallel && index === services.length - 1) }"
            type="button"
            :data-testid="`operation-finish-${service.serviceId}`"
            @click="handleFinish(service.serviceId)"
          >
            Encerrar atendimento
          </button>
          <button
            v-if="canStartParallel && index === services.length - 1"
            class="service-card__icon-action"
            type="button"
            :data-testid="`operation-start-parallel-${service.serviceId}`"
            title="Abrir outro atendimento"
            @click="handleStartParallel"
          >
            <span class="material-icons-round">add</span>
          </button>
        </div>
      </div>
    </div>
  </article>
</template>
