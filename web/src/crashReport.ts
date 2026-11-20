// Settings > System > Problems: crashes of the TV app and the web app, and the
// text "Copy for Claude" puts on the clipboard. Nothing here talks to the server.

export interface Crash {
  id: number
  app: 'tv' | 'web' | string
  headline: string
  place: string
  version: string
  device: string
  devices: string[]
  versions: string[]
  trace: string
  count: number
  firstAt: string
  lastAt: string
  fixedAt?: string
}

export interface CrashList {
  items: Crash[]
  open: number
  alerts: boolean
  serverVersion: string
}

export function appName(app: string): string {
  return app === 'web' ? 'Web app' : 'TV app'
}

// "3 times on 2 devices" / "Once".
export function seenText(c: Pick<Crash, 'count' | 'devices'>): string {
  const times = c.count === 1 ? 'Once' : `${c.count} times`
  const n = c.devices.length
  return n > 1 ? `${times} on ${n} devices` : times
}

// Where the app's source for a crash lives, so Claude starts looking there.
function sourceHint(app: string): string {
  return app === 'web' ? 'web/src (React, TypeScript)' : 'android/app/src/main/java/app/cue/tv (Kotlin, Jetpack Compose for TV)'
}

// The text to paste into a Claude Code session: what crashed, where, how
// often, and the trace, with what to do. No addresses, names or keys: a
// trace and a device model only.
export function claudeText(c: Crash, serverVersion = ''): string {
  const lines = [
    `Cue's ${c.app === 'web' ? 'web app' : 'TV app (Fire TV / Android TV)'} crashed. Please find the cause and fix it in the jhipsman/cueapp repository.`,
    '',
    `- What: ${c.headline || 'no message'}`,
  ]
  if (c.place) lines.push(`- Where: ${c.place}`)
  lines.push(`- Source: ${sourceHint(c.app)}`)
  lines.push(`- Seen: ${seenText(c)}, first ${c.firstAt.slice(0, 10)}, last ${c.lastAt.slice(0, 10)}`)
  if (c.fixedAt) lines.push(`- Marked fixed ${c.fixedAt.slice(0, 10)}`)
  if (c.versions.length) lines.push(`- App versions: ${c.versions.join(', ')}`)
  if (c.devices.length) lines.push(`- Devices: ${c.devices.join('; ')}`)
  if (serverVersion) lines.push(`- Cue server: ${serverVersion}`)
  lines.push(
    '',
    'Stack trace (latest report):',
    '```text',
    c.trace.trim(),
    '```',
    '',
    'Fix the cause rather than catching the exception, keep the change small, add a line to CHANGELOG.md under "Unreleased", and open a pull request. If the trace points into a library, find the call in our code that leads there.',
  )
  return lines.join('\n')
}

// The open crashes, newest first, then the fixed ones.
export function splitCrashes(items: Crash[]): { open: Crash[]; fixed: Crash[] } {
  return { open: items.filter((c) => !c.fixedAt), fixed: items.filter((c) => !!c.fixedAt) }
}
