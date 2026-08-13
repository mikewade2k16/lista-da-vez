export type OmniVideoPlayerTheme = 'omni' | 'cinema' | 'social' | 'minimal'

export type OmniVideoPlayerAspect = 'auto' | 'landscape' | 'portrait' | 'square' | 'cinema'

export type OmniVideoPlayerControl =
  | 'previous'
  | 'next'
  | 'play'
  | 'seek-back'
  | 'seek-forward'
  | 'progress'
  | 'time'
  | 'volume'
  | 'speed'
  | 'quality'
  | 'captions'
  | 'pip'
  | 'download'
  | 'fullscreen'

export interface OmniVideoPlayerSource {
  src: string
  label: string
  type?: string
}

export interface OmniVideoPlayerTrack {
  src: string
  srcLang: string
  label: string
  kind?: 'captions' | 'subtitles'
  default?: boolean
}

export const OMNI_VIDEO_PLAYER_ALL_CONTROLS: readonly OmniVideoPlayerControl[] = [
  'previous',
  'next',
  'play',
  'seek-back',
  'seek-forward',
  'progress',
  'time',
  'volume',
  'speed',
  'quality',
  'captions',
  'pip',
  'download',
  'fullscreen',
]

export const OMNI_VIDEO_PLAYER_DEFAULT_CONTROLS: readonly OmniVideoPlayerControl[] = [
  'play',
  'seek-back',
  'seek-forward',
  'progress',
  'time',
  'volume',
  'speed',
  'captions',
  'pip',
  'fullscreen',
]
