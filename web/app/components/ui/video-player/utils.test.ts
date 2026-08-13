import { describe, expect, it } from 'vitest'
import {
  formatVideoTime,
  resolveVideoPlayerShortcut,
  videoAspectRatio,
  videoProgressPercent,
} from './utils'

describe('video player utils', () => {
  it('formats short and long durations', () => {
    expect(formatVideoTime(65.8)).toBe('1:05')
    expect(formatVideoTime(3661)).toBe('1:01:01')
    expect(formatVideoTime(Number.NaN)).toBe('0:00')
  })

  it('clamps timeline progress', () => {
    expect(videoProgressPercent(30, 120)).toBe(25)
    expect(videoProgressPercent(140, 120)).toBe(100)
    expect(videoProgressPercent(-1, 120)).toBe(0)
    expect(videoProgressPercent(2, 0)).toBe(0)
  })

  it('maps supported aspect presets', () => {
    expect(videoAspectRatio('portrait')).toBe('9 / 16')
    expect(videoAspectRatio('cinema')).toBe('21 / 9')
    expect(videoAspectRatio('auto')).toBeUndefined()
  })

  it('maps YouTube-style keyboard shortcuts', () => {
    expect(resolveVideoPlayerShortcut(' ', false)).toBe('toggle-playback')
    expect(resolveVideoPlayerShortcut('Backspace', false)).toBe('toggle-playback')
    expect(resolveVideoPlayerShortcut('ArrowLeft', false)).toBe('seek-back')
    expect(resolveVideoPlayerShortcut('ArrowRight', false)).toBe('seek-forward')
    expect(resolveVideoPlayerShortcut('ArrowUp', false)).toBe('volume-up')
    expect(resolveVideoPlayerShortcut('ArrowDown', false)).toBe('volume-down')
    expect(resolveVideoPlayerShortcut('m', false)).toBe('mute')
    expect(resolveVideoPlayerShortcut('f', false)).toBe('fullscreen')
    expect(resolveVideoPlayerShortcut('c', false)).toBeNull()
    expect(resolveVideoPlayerShortcut('c', true)).toBe('captions')
  })
})
