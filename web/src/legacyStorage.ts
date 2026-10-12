// Settings this browser saved before the rename to Cue (theme, sidebar,
// library view, reader and player preferences, ...) were stored under
// "mediarium" names. Each one is moved to its "cue" name once, before
// anything reads it. Storage can be blocked, so this never throws.
const OLD = 'mediarium'

try {
  for (const key of Object.keys(localStorage)) {
    if (!key.startsWith(OLD)) continue
    const next = 'cue' + key.slice(OLD.length)
    const value = localStorage.getItem(key)
    if (value !== null && localStorage.getItem(next) === null) localStorage.setItem(next, value)
    localStorage.removeItem(key)
  }
} catch {
  // ignore — the settings fall back to their defaults
}

export {}
