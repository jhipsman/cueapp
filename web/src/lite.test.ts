import { test } from 'node:test'
import assert from 'node:assert/strict'
import { liteAuto } from './lite.ts'

// A browser to try liteAuto in: its name, memory and cores.
function device(ua: string, memory?: number, cores?: number) {
  Object.defineProperty(globalThis, 'navigator', { value: { userAgent: ua, deviceMemory: memory, hardwareConcurrency: cores }, configurable: true })
  Object.defineProperty(globalThis, 'location', { value: { search: '' }, configurable: true })
  Object.defineProperty(globalThis, 'sessionStorage', { value: { getItem: () => null }, configurable: true })
}

test('Light mode is on by itself on TV browsers', () => {
  device('Mozilla/5.0 (Web0S; Linux/SmartTV) AppleWebKit/537.36 Chrome/94', 4, 4)
  assert.equal(liteAuto(), true)
  device('Mozilla/5.0 (SMART-TV; Linux; Tizen 6.0) AppleWebKit/537.36', 8, 4)
  assert.equal(liteAuto(), true)
  device('Mozilla/5.0 (Linux; Android 9; AFTMM Build/PS7633) AppleWebKit/537.36 Chrome/120', 2, 4)
  assert.equal(liteAuto(), true)
})

test('Light mode is on by itself with little memory or two cores', () => {
  device('Mozilla/5.0 (X11; CrOS x86_64) Chrome/120', 2, 4)
  assert.equal(liteAuto(), true)
  device('Mozilla/5.0 (Linux; Android 12) Chrome/120 Mobile', 4, 2)
  assert.equal(liteAuto(), true)
})

test('Light mode stays off on an ordinary computer or phone', () => {
  device('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/605.1.15', undefined, 8)
  assert.equal(liteAuto(), false)
  device('Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120', 8, 8)
  assert.equal(liteAuto(), false)
  device('Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148')
  assert.equal(liteAuto(), false)
})
