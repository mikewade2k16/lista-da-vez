<script setup lang="ts">
import { UAvatar, UIcon } from "#components";
import { computed, onBeforeUnmount, ref, watch } from "vue";
import type { MessageDirection } from "~/types";
import { resolveAvatarSource } from "~/composables/omnichannel/useAvatarProxy";

const props = withDefaults(
  defineProps<{
    src: string | null | undefined;
    durationSeconds?: number | null;
    direction?: MessageDirection;
    compact?: boolean;
    avatarUrl?: string | null;
    avatarText?: string | null;
  }>(),
  {
    durationSeconds: null,
    direction: "INBOUND",
    compact: false,
    avatarUrl: null,
    avatarText: null
  }
);

const audioRef = ref<HTMLAudioElement | null>(null);
const waveformRef = ref<HTMLElement | null>(null);
const isPlaying = ref(false);
const currentTimeSeconds = ref(0);
const durationSeconds = ref(
  typeof props.durationSeconds === "number" && Number.isFinite(props.durationSeconds)
    ? Math.max(0, props.durationSeconds)
    : 0
);

const waveformBars = computed(() => buildWaveformBars(props.src || ""));
const progressRatio = computed(() => {
  if (!durationSeconds.value || durationSeconds.value <= 0) {
    return 0;
  }

  return Math.max(0, Math.min(1, currentTimeSeconds.value / durationSeconds.value));
});

const timeLabel = computed(() => {
  if (!isPlaying.value && durationSeconds.value <= 0) {
    return "--:--";
  }

  const value = isPlaying.value ? currentTimeSeconds.value : durationSeconds.value;
  return formatDuration(value);
});

const shouldShowAvatar = computed(() => {
  if (props.compact) {
    return false;
  }

  return Boolean((props.avatarUrl && props.avatarUrl.trim().length > 0) || (props.avatarText && props.avatarText.trim().length > 0));
});

watch(
  () => props.src,
  () => {
    if (audioRef.value) {
      audioRef.value.pause();
      audioRef.value.currentTime = 0;
      audioRef.value.load();
    }

    isPlaying.value = false;
    currentTimeSeconds.value = 0;
    durationSeconds.value =
      typeof props.durationSeconds === "number" && Number.isFinite(props.durationSeconds)
        ? Math.max(0, props.durationSeconds)
        : 0;
  }
);

watch(
  () => props.durationSeconds,
  (nextDuration) => {
    if (isPlaying.value) {
      return;
    }

    if (typeof nextDuration !== "number" || !Number.isFinite(nextDuration)) {
      return;
    }

    durationSeconds.value = Math.max(0, nextDuration);
  }
);

onBeforeUnmount(() => {
  if (audioRef.value) {
    audioRef.value.pause();
  }
});

function createSeed(value: string) {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }

  return hash >>> 0;
}

function buildWaveformBars(value: string) {
  const bars: number[] = [];
  let seed = createSeed(value || "audio-wave");

  for (let index = 0; index < 36; index += 1) {
    seed = (seed * 1664525 + 1013904223) >>> 0;
    const normalized = ((seed % 1000) / 1000) * 0.8 + 0.2;
    bars.push(normalized);
  }

  return bars;
}

function formatDuration(value: number) {
  const safeValue = Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0;
  const minutes = String(Math.floor(safeValue / 60)).padStart(2, "0");
  const seconds = String(safeValue % 60).padStart(2, "0");
  return `${minutes}:${seconds}`;
}

function togglePlayback() {
  const element = audioRef.value;
  if (!element || !props.src) {
    return;
  }

  if (isPlaying.value) {
    element.pause();
    isPlaying.value = false;
    return;
  }

  void element.play().then(() => {
    isPlaying.value = true;
  }).catch(() => {
    isPlaying.value = false;
  });
}

function onLoadedMetadata() {
  if (!audioRef.value) {
    return;
  }

  if (Number.isFinite(audioRef.value.duration) && audioRef.value.duration > 0) {
    durationSeconds.value = Math.max(0, audioRef.value.duration);
  }
}

function onTimeUpdate() {
  if (!audioRef.value) {
    return;
  }

  currentTimeSeconds.value = Number.isFinite(audioRef.value.currentTime)
    ? Math.max(0, audioRef.value.currentTime)
    : 0;
}

function onEnded() {
  isPlaying.value = false;
  if (audioRef.value) {
    audioRef.value.currentTime = 0;
  }
  currentTimeSeconds.value = 0;
}

function onPlaybackError() {
  isPlaying.value = false;
}

function seekFromWaveform(clientX: number) {
  const element = waveformRef.value;
  const audioElement = audioRef.value;
  if (!element || !audioElement) {
    return;
  }

  const totalDuration = Number.isFinite(audioElement.duration) && audioElement.duration > 0
    ? audioElement.duration
    : durationSeconds.value;
  if (!Number.isFinite(totalDuration) || totalDuration <= 0) {
    return;
  }

  const bounds = element.getBoundingClientRect();
  if (!Number.isFinite(bounds.width) || bounds.width <= 0) {
    return;
  }

  const ratio = Math.max(0, Math.min(1, (clientX - bounds.left) / bounds.width));
  const targetTime = ratio * totalDuration;
  audioElement.currentTime = targetTime;
  currentTimeSeconds.value = targetTime;
}

function onWaveformPointerDown(event: MouseEvent) {
  seekFromWaveform(event.clientX);
}
</script>

<template>
  <div
    class="audio-message-player"
    :class="[
      `audio-message-player--${direction === 'OUTBOUND' ? 'outbound' : 'inbound'}`,
      { 'audio-message-player--compact': compact }
    ]"
  >
    <UAvatar
      v-if="shouldShowAvatar"
      :src="resolveAvatarSource(avatarUrl) || undefined"
      :text="avatarText || undefined"
      size="2xs"
      class="audio-message-player__avatar"
    />

    <button
      type="button"
      class="audio-message-player__action"
      :disabled="!src"
      @click="togglePlayback"
    >
      <UIcon :name="isPlaying ? 'i-lucide-pause' : 'i-lucide-play'" />
    </button>

    <div class="audio-message-player__waveform">
      <button
        ref="waveformRef"
        type="button"
        class="audio-message-player__waveform-hitbox"
        @mousedown="onWaveformPointerDown"
      />
      <span
        v-for="(bar, index) in waveformBars"
        :key="`bar-${index}`"
        class="audio-message-player__bar"
        :class="{ 'audio-message-player__bar--active': index / waveformBars.length <= progressRatio }"
        :style="{ height: `${Math.round((compact ? 10 : 14) + bar * (compact ? 8 : 12))}px` }"
      />
    </div>

    <span class="audio-message-player__time">{{ timeLabel }}</span>

    <audio
      ref="audioRef"
      :src="src || undefined"
      preload="metadata"
      class="audio-message-player__native"
      @loadedmetadata="onLoadedMetadata"
      @timeupdate="onTimeUpdate"
      @ended="onEnded"
      @error="onPlaybackError"
    />
  </div>
</template>

<style scoped>
.audio-message-player {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-height: 2.5rem;
}

.audio-message-player--compact {
  min-height: 2.1rem;
}

.audio-message-player__avatar {
  flex-shrink: 0;
}

.audio-message-player__action {
  flex-shrink: 0;
  width: 2rem;
  height: 2rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
  display: grid;
  place-items: center;
  cursor: pointer;
}

.audio-message-player--compact .audio-message-player__action {
  width: 1.8rem;
  height: 1.8rem;
}

.audio-message-player__waveform {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.12rem;
  height: 1.7rem;
  position: relative;
  cursor: pointer;
}

.audio-message-player__waveform-hitbox {
  appearance: none;
  position: absolute;
  inset: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
  padding: 0;
  margin: 0;
  z-index: 2;
}

.audio-message-player__bar {
  width: 0.12rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.45);
  transition: background-color 0.2s ease;
  pointer-events: none;
  position: relative;
  z-index: 1;
}

.audio-message-player__bar--active {
  background: rgb(var(--primary));
}

.audio-message-player__time {
  font-size: 0.73rem;
  color: rgb(var(--muted));
  min-width: 2.6rem;
  text-align: right;
}

.audio-message-player__native {
  display: none;
}
</style>
