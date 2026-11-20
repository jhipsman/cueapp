// Run with: npm test
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { claudeText, seenText, splitCrashes, type Crash } from './crashReport.ts'

const crash: Crash = {
  id: 7,
  app: 'tv',
  headline: 'java.lang.IllegalStateException: Release should only be called once',
  place: 'app.cue.tv.home.PartsKt.CardRow(Parts.kt:233)',
  version: '1.5',
  device: 'TCL 55C (Android 12)',
  devices: ['TCL 55C (Android 12)', 'Fire TV Stick 4K (Android 9)'],
  versions: ['1.4', '1.5'],
  trace: 'java.lang.IllegalStateException: Release should only be called once\n\tat app.cue.tv.home.PartsKt.CardRow(Parts.kt:233)\n',
  count: 14,
  firstAt: '2026-11-18T09:00:00Z',
  lastAt: '2026-11-20T21:30:00Z',
}

test('seenText', () => {
  assert.equal(seenText({ count: 1, devices: ['a'] }), 'Once')
  assert.equal(seenText({ count: 3, devices: ['a'] }), '3 times')
  assert.equal(seenText(crash), '14 times on 2 devices')
})

test('claudeText carries what Claude needs', () => {
  const t = claudeText(crash, '2.8.0')
  assert.match(t, /^Cue's TV app \(Fire TV \/ Android TV\) crashed\./)
  assert.match(t, /- Where: app\.cue\.tv\.home\.PartsKt\.CardRow\(Parts\.kt:233\)/)
  assert.match(t, /- Seen: 14 times on 2 devices, first 2026-11-18, last 2026-11-20/)
  assert.match(t, /- App versions: 1\.4, 1\.5/)
  assert.match(t, /- Cue server: 2\.8\.0/)
  assert.match(t, /```text\njava\.lang\.IllegalStateException: Release should only be called once\n\tat app\.cue/)
  assert.match(t, /web\/src|android\/app/)
  assert.doesNotMatch(claudeText({ ...crash, app: 'web' }), /Kotlin/)
})

test('splitCrashes', () => {
  const { open, fixed } = splitCrashes([crash, { ...crash, id: 8, fixedAt: '2026-11-19T00:00:00Z' }])
  assert.deepEqual(
    [open.map((c) => c.id), fixed.map((c) => c.id)],
    [[7], [8]],
  )
})
