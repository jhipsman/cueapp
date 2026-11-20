// The nightly polish walk of the web app: signs in to a throwaway Cue, turns
// on demo mode (sample titles in every state), and visits the main pages on a
// desktop and a phone, with video recording on. Each move is marked in
// marks.json with its time in the video, so a reviewer can cut out each
// transition. Console errors and page crashes go to errors.json.
//
//   node tools/polish/web-walk.mjs http://localhost:8264 out/web
//
// Needs Playwright (require('playwright')) and a Cue with an admin
// "polish" / $POLISH_PASSWORD.
import { mkdirSync, writeFileSync, renameSync } from 'node:fs'
import { createRequire } from 'node:module'
import path from 'node:path'

const require = createRequire(import.meta.url)
const { chromium } = require('playwright')

const base = process.argv[2] ?? 'http://localhost:8264'
const out = path.resolve(process.argv[3] ?? 'polish-out/web')
const password = process.env.POLISH_PASSWORD ?? 'polish-walk-2026'
mkdirSync(out, { recursive: true })

// Where to go, in order: [name, how]. A string is a path; a function acts on the page.
const STEPS = [
  ['dashboard', '/'],
  ['discover', '/discover'],
  ['open-a-title', async (page) => {
    const card = page.locator('a[href^="/title/"], a[href^="/show/"]').first()
    if (await card.count()) await card.click()
    else await page.goto(base + '/title/603')
  }],
  ['back', async (page) => page.goBack()],
  ['library', '/library'],
  ['calendar', '/calendar'],
  ['queue', '/queue'],
  ['search', async (page) => {
    await page.goto(base + '/search')
    const box = page.locator('input[type="search"], input[type="text"]').first()
    if (await box.count()) await box.pressSequentially('matrix', { delay: 60 })
  }],
  ['watch-home', '/watch'],
  ['settings', '/settings/streaming'],
  ['settings-notifications', '/settings/notifications'],
  ['settings-problems', '/settings/problems'],
  ['settings-logs', '/settings/logs'],
  ['open-a-menu', async (page) => {
    const more = page.getByRole('button', { name: /more|menu|account/i }).first()
    if (await more.count()) await more.click()
  }],
]

const VIEWPORTS = [
  ['desktop', { width: 1440, height: 900 }],
  ['phone', { width: 390, height: 844 }],
]

const errors = []
const marks = {}
const browser = await chromium.launch()
for (const [vpName, viewport] of VIEWPORTS) {
  const dir = path.join(out, vpName)
  mkdirSync(dir, { recursive: true })
  const context = await browser.newContext({ viewport, deviceScaleFactor: 1, recordVideo: { dir, size: viewport }, colorScheme: 'dark' })
  const res = await context.request.post(base + '/api/auth/login', { data: { username: 'polish', password } })
  if (!res.ok()) throw new Error(`sign-in failed: ${res.status()} ${await res.text()}`)
  await context.addInitScript(() => {
    try {
      localStorage.setItem('cue-demo', '1')
    } catch {}
  })
  const page = await context.newPage()
  page.on('console', (m) => {
    if (m.type() === 'error') errors.push({ viewport: vpName, url: page.url(), kind: 'console', text: m.text().slice(0, 2000) })
  })
  page.on('pageerror', (e) => errors.push({ viewport: vpName, url: page.url(), kind: 'crash', text: `${e.name}: ${e.message}\n${e.stack ?? ''}`.slice(0, 4000) }))

  const t0 = Date.now()
  marks[vpName] = []
  for (const [name, how] of STEPS) {
    const at = (Date.now() - t0) / 1000
    try {
      if (typeof how === 'string') await page.goto(base + how, { waitUntil: 'domcontentloaded' })
      else await how(page)
      await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {})
    } catch (e) {
      errors.push({ viewport: vpName, url: page.url(), kind: 'step', text: `${name}: ${e.message}` })
    }
    // Let the motion play out, then keep a still of where it settled.
    await page.waitForTimeout(1600)
    await page.screenshot({ path: path.join(dir, `${String(marks[vpName].length + 1).padStart(2, '0')}-${name}.png`) })
    marks[vpName].push({ step: name, start: at, end: (Date.now() - t0) / 1000, url: page.url().replace(base, '') })
  }
  const video = page.video()
  await context.close()
  if (video) renameSync(await video.path(), path.join(dir, 'walk.webm'))
}
await browser.close()
writeFileSync(path.join(out, 'marks.json'), JSON.stringify(marks, null, 2))
writeFileSync(path.join(out, 'errors.json'), JSON.stringify(errors, null, 2))
console.log(`web walk: ${Object.values(marks).flat().length} steps, ${errors.length} errors -> ${out}`)
