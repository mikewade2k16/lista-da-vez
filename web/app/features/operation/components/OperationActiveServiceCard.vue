<script setup>
import { computed } from "vue";
import { buildNickname } from "~/domain/utils/person-display";
import { formatClock, formatDuration } from "~/domain/utils/time";
import { useAlertsStore } from "~/stores/alerts";
import { alertCardStyle } from "~/utils/alert-colors";

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
  },
  cancelWindowSeconds: {
    type: Number,
    default: 30
  }
});

const emit = defineEmits(["finish", "stop", "startParallel"]);

const alertsStore = useAlertsStore();

const primaryService = computed(() => props.services?.[0] || null);

const activeServiceAlert = computed(() => {
  for (const service of props.services || []) {
    const serviceId = String(service?.serviceId || "").trim();
    if (!serviceId) {
      continue;
    }

    const alert = alertsStore.alertForService(serviceId);
    if (alert) {
      return alert;
    }
  }

  return null;
});

const activeServiceAlertLabel = computed(() => {
  const alert = activeServiceAlert.value;
  if (alert?.type === "long_open_service") {
    const elapsed = dynamicAlertElapsed(alert);
    if (elapsed) {
      return `Atendimento em aberto ha ${elapsed}`;
    }
  }

  return String(alert?.headline || alert?.body || "Alerta ativo").trim();
});

const activeServiceAlertTitle = computed(() => {
  const alert = activeServiceAlert.value;
  const body = String(alert?.body || "").trim();
  return body || activeServiceAlertLabel.value;
});

const activeServiceAlertStyle = computed(() => {
  return alertCardStyle(activeServiceAlert.value?.colorTheme || "amber");
});
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

function firstPositiveTimestamp(values) {
  return values
    .map((value) => Number(value || 0) || 0)
    .filter((value) => value > 0)
    .sort((left, right) => left - right)[0] || 0;
}

function isFrozenTimer(service) {
  return frozenAt(service) > 0;
}

function frozenAt(service) {
  return firstPositiveTimestamp([service?.effectiveFinishedAt, service?.stoppedAt]);
}

function cancelWindowMs() {
  return Math.max(0, Number(props.cancelWindowSeconds || 0) || 0) * 1000;
}

function isWithinCancelWindow(service) {
  if (Number(service?.stoppedAt || 0) > 0) {
    return false;
  }

  const windowMs = cancelWindowMs();
  if (!props.clockReady || windowMs <= 0) {
    return false;
  }

  const elapsedMs = Math.max(0, props.now - Number(service?.serviceStartedAt || 0));
  return elapsedMs <= windowMs;
}

function cancelProgressStyle(service) {
  const windowMs = cancelWindowMs();
  if (!isWithinCancelWindow(service) || windowMs <= 0) {
    return { width: "0%" };
  }

  const elapsedMs = Math.max(0, props.now - Number(service?.serviceStartedAt || 0));
  const remainingRatio = Math.max(0, 1 - (elapsedMs / windowMs));
  return { width: `${Math.max(0, Math.min(100, remainingRatio * 100))}%` };
}

function primaryActionLabel(service) {
  return isWithinCancelWindow(service) ? "Cancelar atendimento" : "Encerrar atendimento";
}

function isStopped(service) {
  return Number(service?.stoppedAt || 0) > 0;
}

function timerLabel(service) {
  if (!props.clockReady) {
    return "--:--";
  }

  return formatDuration(
    Math.max(
      0,
      isFrozenTimer(service)
        ? frozenAt(service) - Number(service?.serviceStartedAt || 0)
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

function alertStartedAt(alert) {
  const metadataStartedAt = Number(alert?.metadata?.serviceStartedAt || 0) || 0;
  if (metadataStartedAt > 0) {
    return metadataStartedAt;
  }

  const openedAt = Date.parse(String(alert?.openedAt || alert?.lastTriggeredAt || ""));
  return Number.isFinite(openedAt) ? openedAt : 0;
}

function formatAlertElapsed(elapsedMs) {
  const minutes = Math.max(0, Math.floor(Number(elapsedMs || 0) / 60000));

  if (minutes < 60) {
    return `${minutes}m`;
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return remainingMinutes === 0 ? `${hours}h` : `${hours}h${remainingMinutes}m`;
}

function dynamicAlertElapsed(alert) {
  const startedAt = alertStartedAt(alert);
  if (!startedAt) {
    return "";
  }

  const referenceNow = props.clockReady && props.now > 0 ? props.now : Date.now();
  return formatAlertElapsed(referenceNow - startedAt);
}

function serviceMetaLabel(service) {
  const startedLabel = `Iniciado às ${startedAtLabel(service)}`;
  // const stopLabel = isStopped(service) ? "Parado" : "Em andamento";

  // if (isPrimaryService(service)) {
  //   return `${stopLabel} · ${startedLabel}`;
  // }

  // return `${compactStatusLabel(service)} · ${stopLabel} · ${startedLabel}`;
  return startedLabel;
}

function handleFinish(service) {
	emit("finish", service || null);
}

function handleStop(service) {
	emit("stop", service || null);
}

function handleStartParallel() {
  emit("startParallel", primaryService.value?.id || "");
}
</script>

<template>
  <article
    class="service-card"
    :class="{ 'service-card--alert-active': activeServiceAlert }"
    :style="activeServiceAlert ? activeServiceAlertStyle : null"
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

      <span v-if="activeServiceAlert" class="service-card__alert-badge" :title="activeServiceAlertTitle">
        <span class="material-icons-round" aria-hidden="true">timer</span>
        <span class="service-card__alert-badge-text">{{ activeServiceAlertLabel }}</span>
      </span>
      <span v-else-if="parallelBadge" class="service-card__badge">{{ parallelBadge }}</span>
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

        <div
          v-if="!readOnly && !integratedMode"
          class="service-card__row-actions"
          :class="{ 'service-card__row-actions--parallel': canStartParallel && index === services.length - 1 }"
        >
          <button
            class="column-action column-action--secondary service-card__icon-action service-card__icon-action--stop"
            type="button"
            :disabled="isStopped(service)"
            :data-testid="`operation-stop-${service.serviceId}`"
            :title="isStopped(service) ? 'Atendimento parado' : 'Parar atendimento'"
            @click="handleStop(service)"
          >
            <span class="material-icons-round">stop_circle</span>
          </button>
          <button
            class="column-action column-action--secondary service-card__action service-card__action-button"
            type="button"
            :data-testid="`operation-finish-${service.serviceId}`"
            @click="handleFinish(service)"
          >
            <span
              v-if="isWithinCancelWindow(service)"
              class="service-card__action-progress"
              :style="cancelProgressStyle(service)"
            />
            <span class="service-card__action-label">{{ primaryActionLabel(service) }}</span>
          </button>
          <button
            v-if="canStartParallel && index === services.length - 1"
            class="column-action column-action--secondary service-card__icon-action"
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
