<script setup lang="ts">
import type {
  OmniVideoPlayerAspect,
  OmniVideoPlayerControl,
  OmniVideoPlayerSource,
  OmniVideoPlayerTheme,
  OmniVideoPlayerTrack,
} from './video-player/types'
import { OMNI_VIDEO_PLAYER_DEFAULT_CONTROLS } from './video-player/types'
import { useOmniVideoPlayer } from './video-player/useOmniVideoPlayer'

const props = withDefaults(
  defineProps<{
    src: string
    poster?: string
    title?: string
    description?: string
    theme?: OmniVideoPlayerTheme
    aspect?: OmniVideoPlayerAspect
    fit?: 'contain' | 'cover'
    controls?: readonly OmniVideoPlayerControl[]
    sources?: OmniVideoPlayerSource[]
    tracks?: OmniVideoPlayerTrack[]
    downloadName?: string
    autoplay?: boolean
    muted?: boolean
    loop?: boolean
    autofocus?: boolean
    clickToToggle?: boolean
    pauseDetailsDelay?: number
  }>(),
  {
    poster: '',
    title: '',
    description: '',
    theme: 'omni',
    aspect: 'auto',
    fit: 'contain',
    controls: () => [...OMNI_VIDEO_PLAYER_DEFAULT_CONTROLS],
    sources: () => [],
    tracks: () => [],
    downloadName: '',
    autoplay: false,
    muted: false,
    loop: false,
    autofocus: false,
    clickToToggle: true,
    pauseDetailsDelay: 700,
  },
)

const emit = defineEmits<{
  play: []
  pause: []
  ended: []
  error: []
  previous: []
  next: []
  'playback-blocked': []
}>()

const {
  rootElement,
  videoElement,
  paused,
  loading,
  currentTime,
  duration,
  volume,
  volumePercent,
  muted,
  playbackRate,
  controlsVisible,
  detailsVisible,
  speedMenuOpen,
  captionsMenuOpen,
  qualityMenuOpen,
  activeCaption,
  activeSourceUrl,
  activeQualityLabel,
  fullscreen,
  pictureInPicture,
  pictureInPictureAvailable,
  playerStyle,
  timeLabel,
  hasCaptions,
  hasQualities,
  hasAnyControls,
  hasControl,
  play,
  pause,
  focusPlayer,
  togglePlayback,
  onPlay,
  onPause,
  onLoadedMetadata,
  onTimeUpdate,
  seekBy,
  onTimelineInput,
  onVolumeInput,
  toggleMute,
  setPlaybackRate,
  setQuality,
  applyCaption,
  toggleSpeedMenu,
  toggleCaptionsMenu,
  toggleQualityMenu,
  toggleFullscreen,
  togglePictureInPicture,
  onKeydown,
  onStageClick,
  revealControls,
  scheduleControlsHide,
} = useOmniVideoPlayer(props, {
  onPlay: () => emit('play'),
  onPause: () => emit('pause'),
  onPlaybackBlocked: () => emit('playback-blocked'),
})

defineExpose({ play, pause, focus: focusPlayer, togglePlayback, videoElement })
</script>

<template>
  <div
    ref="rootElement"
    class="omni-video-player"
    :class="[
      `is-theme-${theme}`,
      { 'is-paused': paused, 'is-controls-visible': controlsVisible, 'is-loading': loading },
    ]"
    :style="playerStyle"
    tabindex="0"
    role="region"
    :aria-label="title || 'Player de vídeo'"
    @keydown="onKeydown"
    @pointermove="revealControls"
    @pointerleave="scheduleControlsHide"
  >
    <div class="omni-video-player__stage" @click="onStageClick" @dblclick="toggleFullscreen">
      <video
        ref="videoElement"
        class="omni-video-player__video"
        :src="activeSourceUrl"
        :poster="poster || undefined"
        :autoplay="autoplay"
        :muted="muted"
        :loop="loop"
        preload="metadata"
        playsinline
        @loadedmetadata="onLoadedMetadata"
        @durationchange="onLoadedMetadata"
        @timeupdate="onTimeUpdate"
        @progress="onTimeUpdate"
        @play="onPlay"
        @pause="onPause"
        @waiting="loading = true"
        @canplay="loading = false"
        @ended="emit('ended')"
        @error="emit('error')"
        @enterpictureinpicture="pictureInPicture = true"
        @leavepictureinpicture="pictureInPicture = false"
      >
        <track
          v-for="track in tracks"
          :key="`${track.srcLang}:${track.src}`"
          :src="track.src"
          :srclang="track.srcLang"
          :label="track.label"
          :kind="track.kind || 'subtitles'"
          :default="track.default"
        />
      </video>

      <Transition name="omni-video-details">
        <div v-if="detailsVisible" class="omni-video-player__details" aria-live="polite">
          <strong v-if="title">{{ title }}</strong>
          <p v-if="description">{{ description }}</p>
        </div>
      </Transition>

      <button
        v-if="hasControl('play') && (paused || loading)"
        type="button"
        class="omni-video-player__center-action"
        :aria-label="paused ? 'Reproduzir' : 'Carregando vídeo'"
        :disabled="loading"
        @click.stop="togglePlayback"
      >
        <UIcon :name="loading ? 'i-lucide-loader-circle' : 'i-lucide-play'" />
      </button>
    </div>

    <div v-if="hasAnyControls" class="omni-video-player__chrome" @click.stop="focusPlayer">
      <label v-if="hasControl('progress')" class="omni-video-player__timeline">
        <span class="sr-only">Posição do vídeo</span>
        <input
          type="range"
          min="0"
          :max="duration || 0"
          step="0.1"
          :value="currentTime"
          @input="onTimelineInput"
        />
      </label>

      <div class="omni-video-player__toolbar">
        <div class="omni-video-player__group">
          <button
            v-if="hasControl('previous')"
            type="button"
            class="omni-video-player__button"
            aria-label="Vídeo anterior"
            @click="emit('previous')"
          >
            <UIcon name="i-lucide-skip-back" />
          </button>
          <button
            v-if="hasControl('play')"
            type="button"
            class="omni-video-player__button"
            :aria-label="paused ? 'Reproduzir' : 'Pausar'"
            @click="togglePlayback"
          >
            <UIcon :name="paused ? 'i-lucide-play' : 'i-lucide-pause'" />
          </button>
          <button
            v-if="hasControl('next')"
            type="button"
            class="omni-video-player__button"
            aria-label="Próximo vídeo"
            @click="emit('next')"
          >
            <UIcon name="i-lucide-skip-forward" />
          </button>
          <button
            v-if="hasControl('seek-back')"
            type="button"
            class="omni-video-player__button"
            aria-label="Voltar 5 segundos"
            @click="seekBy(-5)"
          >
            <UIcon name="i-lucide-rotate-ccw" />
            <small>5</small>
          </button>
          <button
            v-if="hasControl('seek-forward')"
            type="button"
            class="omni-video-player__button"
            aria-label="Avançar 5 segundos"
            @click="seekBy(5)"
          >
            <UIcon name="i-lucide-rotate-cw" />
            <small>5</small>
          </button>
          <div v-if="hasControl('volume')" class="omni-video-player__volume">
            <button
              type="button"
              class="omni-video-player__button"
              :aria-label="muted ? 'Ativar som' : 'Silenciar'"
              @click="toggleMute"
            >
              <UIcon
                :name="
                  muted || volume === 0
                    ? 'i-lucide-volume-x'
                    : volume < 0.5
                      ? 'i-lucide-volume-1'
                      : 'i-lucide-volume-2'
                "
              />
            </button>
            <label>
              <span class="sr-only">Volume</span>
              <input
                type="range"
                min="0"
                max="1"
                step="0.05"
                :value="muted ? 0 : volume"
                :style="{ '--omni-video-volume': `${volumePercent}%` }"
                @input="onVolumeInput"
              />
            </label>
          </div>
          <span v-if="hasControl('time')" class="omni-video-player__time">{{ timeLabel }}</span>
        </div>

        <div class="omni-video-player__group">
          <div v-if="hasControl('speed')" class="omni-video-player__menu-wrap">
            <button
              type="button"
              class="omni-video-player__button is-text"
              aria-label="Velocidade de reprodução"
              :aria-expanded="speedMenuOpen"
              @click="toggleSpeedMenu"
            >
              {{ playbackRate }}x
            </button>
            <div v-if="speedMenuOpen" class="omni-video-player__menu">
              <button
                v-for="rate in [0.5, 0.75, 1, 1.25, 1.5, 2]"
                :key="rate"
                type="button"
                :class="{ 'is-active': playbackRate === rate }"
                @click="setPlaybackRate(rate)"
              >
                {{ rate }}x
              </button>
            </div>
          </div>
          <div v-if="hasControl('quality') && hasQualities" class="omni-video-player__menu-wrap">
            <button
              type="button"
              class="omni-video-player__button is-text"
              aria-label="Qualidade do vídeo"
              :aria-expanded="qualityMenuOpen"
              @click="toggleQualityMenu"
            >
              {{ activeQualityLabel }}
            </button>
            <div v-if="qualityMenuOpen" class="omni-video-player__menu">
              <button
                v-for="source in sources"
                :key="source.src"
                type="button"
                :class="{ 'is-active': activeSourceUrl === source.src }"
                @click="setQuality(source)"
              >
                {{ source.label }}
              </button>
            </div>
          </div>
          <div v-if="hasControl('captions') && hasCaptions" class="omni-video-player__menu-wrap">
            <button
              type="button"
              class="omni-video-player__button"
              aria-label="Legendas"
              :aria-pressed="activeCaption !== 'off'"
              :aria-expanded="captionsMenuOpen"
              @click="toggleCaptionsMenu"
            >
              <UIcon name="i-lucide-captions" />
            </button>
            <div v-if="captionsMenuOpen" class="omni-video-player__menu">
              <button
                :class="{ 'is-active': activeCaption === 'off' }"
                @click="applyCaption('off')"
              >
                Desativadas
              </button>
              <button
                v-for="track in tracks"
                :key="track.srcLang"
                type="button"
                :class="{ 'is-active': activeCaption === track.srcLang }"
                @click="applyCaption(track.srcLang)"
              >
                {{ track.label }}
              </button>
            </div>
          </div>
          <button
            v-if="hasControl('pip') && pictureInPictureAvailable"
            type="button"
            class="omni-video-player__button"
            aria-label="Picture-in-picture"
            :aria-pressed="pictureInPicture"
            @click="togglePictureInPicture"
          >
            <UIcon name="i-lucide-picture-in-picture-2" />
          </button>
          <a
            v-if="hasControl('download')"
            class="omni-video-player__button"
            :href="activeSourceUrl"
            :download="downloadName || title || 'video'"
            aria-label="Baixar vídeo"
            @click.stop
          >
            <UIcon name="i-lucide-download" />
          </a>
          <button
            v-if="hasControl('fullscreen')"
            type="button"
            class="omni-video-player__button"
            :aria-label="fullscreen ? 'Sair da tela cheia' : 'Tela cheia'"
            @click="toggleFullscreen"
          >
            <UIcon :name="fullscreen ? 'i-lucide-minimize' : 'i-lucide-maximize'" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped src="./video-player/player.css"></style>
