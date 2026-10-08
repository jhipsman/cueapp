// How subtitles are shown, kept on this device: on or off by default, the
// text size, and whether lines sit on a dark band.

export interface SubsPrefs {
  on: boolean
  size: 'small' | 'medium' | 'large'
  band: boolean
}

const KEY = 'cue-subtitles'
const DEFAULTS: SubsPrefs = { on: false, size: 'medium', band: true }

export function loadSubsPrefs(): SubsPrefs {
  try {
    return { ...DEFAULTS, ...JSON.parse(localStorage.getItem(KEY) ?? '{}') }
  } catch {
    return DEFAULTS
  }
}

export function saveSubsPrefs(p: SubsPrefs) {
  try {
    localStorage.setItem(KEY, JSON.stringify(p))
  } catch {
    // Not kept: fine.
  }
}
