import type { OmniVideoPlayerAspect } from './types'

export type OmniVideoPlayerShortcutAction =
  | 'toggle-playback'
  | 'seek-back'
  | 'seek-forward'
  | 'volume-up'
  | 'volume-down'
  | 'mute'
  | 'fullscreen'
  | 'captions'

const ASPECT_RATIOS: Record<Exclude<OmniVideoPlayerAspect, 'auto'>, string> = {
  landscape: '16 / 9',
  portrait: '9 / 16',
  square: '1 / 1',
  cinema: '21 / 9',
}

export function formatVideoTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00'
  const totalSeconds = Math.floor(seconds)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const remainingSeconds = totalSeconds % 60
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, '0')}:${String(remainingSeconds).padStart(2, '0')}`
  }
  return `${minutes}:${String(remainingSeconds).padStart(2, '0')}`
}

export function videoProgressPercent(value: number, duration: number): number {
  if (!Number.isFinite(value) || !Number.isFinite(duration) || duration <= 0) return 0
  return Math.min(Math.max((value / duration) * 100, 0), 100)
}

export function videoAspectRatio(aspect: OmniVideoPlayerAspect): string | undefined {
  return aspect === 'auto' ? undefined : ASPECT_RATIOS[aspect]
}

export function resolveVideoPlayerShortcut(
  key: string,
  hasCaptions: boolean,
): OmniVideoPlayerShortcutAction | null {
  switch (key.toLowerCase()) {
    case ' ':
    case 'backspace':
    case 'k':
      return 'toggle-playback'
    case 'arrowleft':
      return 'seek-back'
    case 'arrowright':
      return 'seek-forward'
    case 'arrowup':
      return 'volume-up'
    case 'arrowdown':
      return 'volume-down'
    case 'm':
      return 'mute'
    case 'f':
      return 'fullscreen'
    case 'c':
      return hasCaptions ? 'captions' : null
    default:
      return null
  }
}
