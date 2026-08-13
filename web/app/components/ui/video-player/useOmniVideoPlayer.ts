import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type {
  OmniVideoPlayerAspect,
  OmniVideoPlayerControl,
  OmniVideoPlayerSource,
  OmniVideoPlayerTrack,
} from './types'
import {
  formatVideoTime,
  resolveVideoPlayerShortcut,
  videoAspectRatio,
  videoProgressPercent,
} from './utils'

interface VideoPlayerRuntimeProps {
  src: string
  title: string
  description: string
  aspect: OmniVideoPlayerAspect
  fit: 'contain' | 'cover'
  controls: readonly OmniVideoPlayerControl[]
  sources?: OmniVideoPlayerSource[]
  tracks: OmniVideoPlayerTrack[]
  muted: boolean
  autofocus: boolean
  clickToToggle: boolean
  pauseDetailsDelay: number
}

interface VideoPlayerCallbacks {
  onPlay: () => void
  onPause: () => void
  onPlaybackBlocked: () => void
}

export function useOmniVideoPlayer(
  props: VideoPlayerRuntimeProps,
  callbacks: VideoPlayerCallbacks,
) {
  const rootElement = ref<HTMLElement | null>(null)
  const videoElement = ref<HTMLVideoElement | null>(null)
  const paused = ref(true)
  const loading = ref(false)
  const currentTime = ref(0)
  const duration = ref(0)
  const bufferedTime = ref(0)
  const volume = ref(props.muted ? 0 : 1)
  const muted = ref(props.muted)
  const previousVolume = ref(1)
  const playbackRate = ref(1)
  const controlsVisible = ref(true)
  const detailsVisible = ref(false)
  const speedMenuOpen = ref(false)
  const captionsMenuOpen = ref(false)
  const qualityMenuOpen = ref(false)
  const activeCaption = ref('off')
  const sourceOptions = computed(() => props.sources ?? [])
  const activeSourceUrl = ref(props.src)
  const activeQualityLabel = ref(
    sourceOptions.value.find((source) => source.src === props.src)?.label ?? 'Original',
  )
  const fullscreen = ref(false)
  const pictureInPicture = ref(false)
  const pictureInPictureAvailable = ref(false)
  let controlsTimer: ReturnType<typeof setTimeout> | undefined
  let detailsTimer: ReturnType<typeof setTimeout> | undefined
  let pendingSourceTime: number | undefined
  let resumeAfterSourceChange = false

  const controlSet = computed(() => new Set(props.controls))
  const playedPercent = computed(() => videoProgressPercent(currentTime.value, duration.value))
  const bufferedPercent = computed(() => videoProgressPercent(bufferedTime.value, duration.value))
  const volumePercent = computed(() => (muted.value ? 0 : volume.value * 100))
  const playerStyle = computed(() => ({
    aspectRatio: videoAspectRatio(props.aspect),
    '--omni-video-fit': props.fit,
    '--omni-video-played': `${playedPercent.value}%`,
    '--omni-video-buffered': `${bufferedPercent.value}%`,
  }))
  const timeLabel = computed(
    () => `${formatVideoTime(currentTime.value)} / ${formatVideoTime(duration.value)}`,
  )
  const hasDetails = computed(() => Boolean(props.title || props.description))
  const hasCaptions = computed(() => props.tracks.length > 0)
  const hasQualities = computed(() => sourceOptions.value.length > 1)
  const hasAnyControls = computed(() => props.controls.length > 0)

  function hasControl(control: OmniVideoPlayerControl): boolean {
    return controlSet.value.has(control)
  }

  function clearTimer(timer: ReturnType<typeof setTimeout> | undefined): void {
    if (timer) clearTimeout(timer)
  }

  function closeMenus(): void {
    speedMenuOpen.value = false
    captionsMenuOpen.value = false
    qualityMenuOpen.value = false
  }

  function toggleSpeedMenu(): void {
    captionsMenuOpen.value = false
    qualityMenuOpen.value = false
    speedMenuOpen.value = !speedMenuOpen.value
  }

  function toggleCaptionsMenu(): void {
    speedMenuOpen.value = false
    qualityMenuOpen.value = false
    captionsMenuOpen.value = !captionsMenuOpen.value
  }

  function toggleQualityMenu(): void {
    speedMenuOpen.value = false
    captionsMenuOpen.value = false
    qualityMenuOpen.value = !qualityMenuOpen.value
  }

  function scheduleControlsHide(): void {
    clearTimer(controlsTimer)
    if (paused.value) return
    controlsTimer = setTimeout(() => {
      controlsVisible.value = false
      closeMenus()
    }, 2400)
  }

  function revealControls(): void {
    controlsVisible.value = true
    scheduleControlsHide()
  }

  function scheduleDetails(): void {
    clearTimer(detailsTimer)
    detailsVisible.value = false
    if (!hasDetails.value) return
    detailsTimer = setTimeout(
      () => {
        if (paused.value) detailsVisible.value = true
      },
      Math.max(props.pauseDetailsDelay, 0),
    )
  }

  async function play(): Promise<void> {
    if (!videoElement.value) return
    try {
      await videoElement.value.play()
    } catch {
      callbacks.onPlaybackBlocked()
    }
  }

  function pause(): void {
    videoElement.value?.pause()
  }

  async function togglePlayback(): Promise<void> {
    if (paused.value) await play()
    else pause()
  }

  function onPlay(): void {
    paused.value = false
    loading.value = false
    detailsVisible.value = false
    clearTimer(detailsTimer)
    scheduleControlsHide()
    callbacks.onPlay()
  }

  function onPause(): void {
    paused.value = true
    controlsVisible.value = true
    clearTimer(controlsTimer)
    scheduleDetails()
    callbacks.onPause()
  }

  function applyCaption(language: string): void {
    const video = videoElement.value
    if (!video) return
    Array.from(video.textTracks).forEach((track) => {
      track.mode = language !== 'off' && track.language === language ? 'showing' : 'disabled'
    })
    activeCaption.value = language
    captionsMenuOpen.value = false
  }

  function applyDefaultCaption(): void {
    const defaultTrack = props.tracks.find((track) => track.default)
    applyCaption(defaultTrack?.srcLang ?? 'off')
  }

  function onLoadedMetadata(): void {
    const video = videoElement.value
    if (!video) return
    duration.value = Number.isFinite(video.duration) ? video.duration : 0
    currentTime.value = video.currentTime
    applyDefaultCaption()
    if (pendingSourceTime !== undefined) {
      video.currentTime = Math.min(pendingSourceTime, duration.value)
      currentTime.value = video.currentTime
      pendingSourceTime = undefined
      if (resumeAfterSourceChange) void play()
      resumeAfterSourceChange = false
    }
    if (video.paused) scheduleDetails()
  }

  function onTimeUpdate(): void {
    const video = videoElement.value
    if (!video) return
    currentTime.value = video.currentTime
    if (video.buffered.length) bufferedTime.value = video.buffered.end(video.buffered.length - 1)
  }

  function seekTo(value: number): void {
    const video = videoElement.value
    if (!video || !duration.value) return
    video.currentTime = Math.min(Math.max(value, 0), duration.value)
    currentTime.value = video.currentTime
    revealControls()
  }

  function seekBy(seconds: number): void {
    seekTo(currentTime.value + seconds)
  }

  function onTimelineInput(event: Event): void {
    seekTo(Number((event.target as HTMLInputElement).value))
  }

  function setVolume(value: number): void {
    const video = videoElement.value
    if (!video) return
    const nextVolume = Math.min(Math.max(value, 0), 1)
    video.volume = nextVolume
    video.muted = nextVolume === 0
    volume.value = nextVolume
    muted.value = video.muted
    if (nextVolume > 0) previousVolume.value = nextVolume
  }

  function onVolumeInput(event: Event): void {
    setVolume(Number((event.target as HTMLInputElement).value))
  }

  function adjustVolume(delta: number): void {
    const video = videoElement.value
    if (!video) return
    setVolume((video.muted ? 0 : video.volume) + delta)
    revealControls()
  }

  function toggleMute(): void {
    const video = videoElement.value
    if (!video) return
    if (video.muted || video.volume === 0) {
      video.muted = false
      video.volume = previousVolume.value || 1
    } else {
      previousVolume.value = video.volume
      video.muted = true
    }
    muted.value = video.muted
    volume.value = video.volume
  }

  function setPlaybackRate(rate: number): void {
    if (!videoElement.value) return
    videoElement.value.playbackRate = rate
    playbackRate.value = rate
    speedMenuOpen.value = false
  }

  async function setQuality(source: OmniVideoPlayerSource): Promise<void> {
    const video = videoElement.value
    if (!video || source.src === activeSourceUrl.value) {
      qualityMenuOpen.value = false
      return
    }
    pendingSourceTime = video.currentTime
    resumeAfterSourceChange = !video.paused
    activeSourceUrl.value = source.src
    activeQualityLabel.value = source.label
    qualityMenuOpen.value = false
    await nextTick()
    videoElement.value?.load()
  }

  function toggleCaptions(): void {
    applyCaption(activeCaption.value === 'off' ? (props.tracks[0]?.srcLang ?? 'off') : 'off')
  }

  async function toggleFullscreen(): Promise<void> {
    if (!rootElement.value || typeof document === 'undefined') return
    if (document.fullscreenElement) await document.exitFullscreen()
    else await rootElement.value.requestFullscreen()
  }

  async function togglePictureInPicture(): Promise<void> {
    const video = videoElement.value
    if (!video || typeof document === 'undefined' || !pictureInPictureAvailable.value) return
    if (document.pictureInPictureElement) await document.exitPictureInPicture()
    else await video.requestPictureInPicture()
  }

  function onDocumentClick(event: MouseEvent): void {
    const target = event.target
    if (!(target instanceof Element) || target.closest('.omni-video-player__menu-wrap')) return
    closeMenus()
  }

  function onKeydown(event: KeyboardEvent): void {
    const target = event.target
    if (
      target instanceof Element &&
      target !== rootElement.value &&
      target.closest('button, a, input, select, textarea')
    )
      return
    if (
      event.key === 'Escape' &&
      (speedMenuOpen.value || captionsMenuOpen.value || qualityMenuOpen.value)
    ) {
      event.stopPropagation()
      closeMenus()
      return
    }
    const shortcut = resolveVideoPlayerShortcut(event.key, hasCaptions.value)
    if (!shortcut) return
    event.preventDefault()
    event.stopPropagation()
    switch (shortcut) {
      case 'toggle-playback':
        void togglePlayback()
        break
      case 'seek-back':
        seekBy(-5)
        break
      case 'seek-forward':
        seekBy(5)
        break
      case 'volume-up':
        adjustVolume(0.05)
        break
      case 'volume-down':
        adjustVolume(-0.05)
        break
      case 'mute':
        toggleMute()
        break
      case 'fullscreen':
        void toggleFullscreen()
        break
      case 'captions':
        toggleCaptions()
        break
    }
  }

  function onStageClick(): void {
    focusPlayer()
    if (props.clickToToggle) void togglePlayback()
  }

  function focusPlayer(): void {
    rootElement.value?.focus({ preventScroll: true })
  }

  function syncFullscreen(): void {
    fullscreen.value =
      typeof document !== 'undefined' && document.fullscreenElement === rootElement.value
  }

  watch(
    () => props.src,
    async () => {
      paused.value = true
      activeSourceUrl.value = props.src
      activeQualityLabel.value =
        sourceOptions.value.find((source) => source.src === props.src)?.label ?? 'Original'
      currentTime.value = 0
      duration.value = 0
      bufferedTime.value = 0
      detailsVisible.value = false
      await nextTick()
      videoElement.value?.load()
    },
  )

  watch(
    () => props.muted,
    (value) => {
      muted.value = value
      if (videoElement.value) videoElement.value.muted = value
    },
  )

  onMounted(() => {
    pictureInPictureAvailable.value =
      typeof document !== 'undefined' &&
      'pictureInPictureEnabled' in document &&
      document.pictureInPictureEnabled
    document.addEventListener('fullscreenchange', syncFullscreen)
    document.addEventListener('click', onDocumentClick)
    if (props.autofocus) void nextTick(focusPlayer)
  })

  onBeforeUnmount(() => {
    clearTimer(controlsTimer)
    clearTimer(detailsTimer)
    if (typeof document !== 'undefined') {
      document.removeEventListener('fullscreenchange', syncFullscreen)
      document.removeEventListener('click', onDocumentClick)
    }
  })

  return {
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
  }
}
