// Walks Cue's LG app (lgtv/dist-app, opened as a file, as on the TV) against
// a stand-in Cue, pressing the remote's keys: sign-in by phone code,
// profiles, Home, the menu, Search, a show's page, and playing (the first
// version fails, then its copy, and the second plays). Saves a screenshot at
// each step and fails on any problem.
//   npm run build && node test/walk.cjs dist-app test-shots
const fs = require('fs')
const path = require('path')
const { chromium } = require('playwright')

const APP = process.argv[2] || 'dist-app'
const OUT = process.argv[3] || 'test-shots'
fs.mkdirSync(OUT, { recursive: true })

// Pictures: one plain tile for every poster and backdrop. Video: 12 seconds
// of a test pattern (test/walk.webm: WebM, what this Chromium plays).
const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAADCAIAAAA2iEnWAAAAFElEQVR4nGO0cYtiYGBgYmBgQFAAEIoA4nyaPE0AAAAASUVORK5CYII=', 'base64')
const VIDEO = path.join(__dirname, 'walk.webm')

const titles = ['Night Harbor', 'Paper Moons', 'Signal Fire', 'Glass Rivers', 'Northbound', 'Copper Sky', 'Quiet Hours', 'Last Light', 'Open Water', 'Far Country']
const card = (i, kind = i % 3 ? 'movie' : 'tv', extra = {}) => ({
  kind, tmdbId: 100 + i, title: titles[i % titles.length], year: 2015 + (i % 9), rating: 6.5 + (i % 3) * 0.6,
  overview: 'A story told over one long summer, when everything changed for a family on the coast and nobody was ready for it.',
  posterUrl: `https://image.tmdb.org/t/p/w500/p${i % 8}.png`, backdropUrl: `https://image.tmdb.org/t/p/w1280/b${i % 8}.png`, ...extra,
})
const home = {
  hero: card(0, 'tv'),
  rows: [
    { key: 'continue', title: 'Continue watching', items: [card(1, 'movie', { progress: 0.4 }), card(0, 'tv', { progress: 0.7, season: 2, episode: 3 }), card(5, 'movie', { progress: 0.2 })] },
    { key: 'trending-movies', title: 'Trending films', items: [2, 4, 5, 7, 8, 1, 6].map((i) => card(i, 'movie')) },
    { key: 'trending-tv', title: 'Trending series', items: [0, 3, 6, 9].map((i) => card(i, 'tv')) },
    { key: 'my-list', title: 'My List', items: [card(4, 'movie'), card(3, 'tv')] },
  ],
  top10: [{ key: 'netflix', name: 'Netflix', items: [1, 2, 3, 4, 5, 6, 7, 8, 9, 0].map((i) => card(i)), movies: [1, 2, 4, 5, 7, 8].map((i) => card(i, 'movie')), shows: [0, 3, 6, 9].map((i) => card(i, 'tv')) }],
}
const title = (kind, id) => {
  const i = id - 100
  const t = { ...card(i, kind), genres: ['Drama', 'Mystery'], certification: 'TV-14', cast: ['Ana Reyes'], inList: false, more: [2, 4, 5, 7].map((j) => card(j, 'movie')), tagline: 'Some tides never turn.' }
  if (kind === 'tv') Object.assign(t, { seasons: [{ number: 1, name: 'Season 1', episodes: 8 }, { number: 2, name: 'Season 2', episodes: 8 }], resumeSeason: 2, resumeEpisode: 3 })
  else Object.assign(t, { runtime: 118, resumePosition: 0 })
  return t
}
const season = (n) => Array.from({ length: 8 }, (_, k) => ({ season: n, episode: k + 1, title: ['Low Tide', 'The Pier', 'Lanterns', 'Salt', 'Undertow', 'The Lighthouse', 'Driftwood', 'Home'][k], overview: 'Things come to a head on the pier.', runtime: 52, aired: !(n === 2 && k > 5), progress: n === 2 && k === 2 ? 0.4 : 0, stillUrl: `https://image.tmdb.org/t/p/w400/b${k % 8}.png` }))

const seen = { headers: [], polls: 0, profileHeader: false, progressSaves: 0 }

;(async () => {
  const b = await chromium.launch()
  const p = await b.newPage({ viewport: { width: 1920, height: 1080 }, deviceScaleFactor: 1 })
  const errors = []
  p.on('pageerror', (e) => errors.push('pageerror ' + e.message))
  p.on('console', (m) => m.type() === 'error' && !/Failed to load resource/.test(m.text()) && errors.push('console ' + m.text()))

  await p.route('https://image.tmdb.org/**', (r) => {
    r.fulfill({ status: 200, contentType: 'image/png', body: PNG })
  })
  await p.route('http://cue.test/**', (r) => {
    const req = r.request()
    const u = new URL(req.url())
    const cors = { 'Access-Control-Allow-Origin': 'null', 'Access-Control-Allow-Headers': 'Content-Type, X-Cue-App, X-Cue-Session, X-Cue-Profile', 'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS', 'Access-Control-Expose-Headers': 'X-Cue-Session' }
    if (req.method() === 'OPTIONS') return r.fulfill({ status: 204, headers: cors })
    const h = req.headers()
    if (u.pathname.startsWith('/api/')) seen.headers.push(`${req.method()} ${u.pathname} app=${h['x-cue-app'] || '-'} session=${h['x-cue-session'] ? 'yes' : 'no'}`)
    if (h['x-cue-profile']) seen.profileHeader = true
    const json = (status, body, extra = {}) => r.fulfill({ status, headers: { ...cors, 'Content-Type': 'application/json', ...extra }, body: JSON.stringify(body) })
    const signedIn = h['x-cue-session'] === 'sess-1'
    const P = u.pathname
    if (P === '/v/test.webm') return r.fulfill({ status: 200, headers: { ...cors, 'Content-Type': 'video/webm' }, body: fs.readFileSync(VIDEO) })
    if (P === '/api/auth/me') return signedIn ? json(200, { id: 1, username: 'sam' }) : json(401, { error: 'unauthorized' })
    if (P === '/api/auth/tv-link' && req.method() === 'POST') return json(200, { code: 'K7Q2MX', token: 'tok', expiresIn: 600 })
    if (P.startsWith('/api/auth/tv-link/qr/')) return r.fulfill({ status: 200, headers: { ...cors, 'Content-Type': 'image/png' }, body: PNG })
    if (P === '/api/auth/tv-link/tok') {
      seen.polls++
      return seen.polls < 2 ? json(200, { pending: true }) : json(200, { ok: true }, { 'X-Cue-Session': 'sess-1' })
    }
    if (!signedIn) return json(401, { error: 'unauthorized' })
    if (P === '/api/profiles') return json(200, { profiles: [{ id: 1, name: 'Sam', avatar: 'teal', hasPin: false, main: true }, { id: 2, name: 'Kids', avatar: 'orange/fox', hasPin: false, main: false }, { id: 3, name: 'Dad', avatar: 'blue', hasPin: true, main: false }] })
    if (P === '/api/profiles/1/select') return json(200, { token: 'prof-1', profile: { id: 1, name: 'Sam', avatar: 'teal' } })
    if (P === '/api/watch/home') return json(200, home)
    if (P === '/api/watch/search') return json(200, [0, 3, 5, 8].map((i) => card(i)))
    let m
    if ((m = P.match(/^\/api\/watch\/tv\/(\d+)\/season\/(\d+)$/))) return json(200, season(+m[2]))
    if ((m = P.match(/^\/api\/watch\/tv\/(\d+)\/next$/))) return json(200, { season: 2, episode: 4, title: 'Salt', aired: true })
    if ((m = P.match(/^\/api\/watch\/(movie|tv)\/(\d+)$/))) return json(200, title(m[1], +m[2]))
    if (P.startsWith('/api/watch/list/')) return json(200, null)
    if (P.startsWith('/api/watch/progress/')) return json(200, { position: 0, duration: 0, finished: false })
    if (P === '/api/watch/progress') {
      seen.progressSaves++
      return json(200, null)
    }
    if (P.startsWith('/api/play/tmdb/')) {
      const opt = +(u.searchParams.get('option') || 1)
      seen.options = (seen.options || []).concat(opt + (u.searchParams.get('client') === 'webos' ? '(webos)' : ''))
      // The first version won't play here (a missing file); its copy won't either; the second does.
      if (opt === 1) return json(200, { title: 'x', url: 'http://cue.test/v/missing.mkv', streamUrl: 'http://cue.test/v/missing.mp4', fileName: 'x.mkv', release: 'x', quality: '2160p', option: 1, options: 2 })
      return json(200, { title: 'x', url: 'http://cue.test/v/test.webm', fileName: 'x.webm', release: 'x', quality: '1080p', option: 2, options: 2 })
    }
    return json(404, { error: 'stand-in: ' + P })
  })

  let n = 0
  const shot = async (name, wait = 700) => {
    await p.waitForTimeout(wait)
    n++
    await p.screenshot({ path: `${OUT}/${String(n).padStart(2, '0')}-${name}.png` })
  }
  const key = async (k, times = 1) => {
    for (let i = 0; i < times; i++) {
      await p.keyboard.press(k)
      await p.waitForTimeout(160)
    }
  }
  const see = async (text, ms = 6000) => {
    try {
      await p.getByText(text, { exact: false }).first().waitFor({ timeout: ms })
    } catch {
      await shot('never-saw-' + text.replace(/\W+/g, '-'), 0)
      throw new Error('never saw ' + text)
    }
  }
  const focused = () => p.evaluate(() => {
    const a = document.activeElement
    return a ? (a.getAttribute('aria-label') || a.textContent || a.tagName).trim().slice(0, 60) : ''
  })

  await p.goto('file://' + path.resolve(APP, 'index.html'))
  await see("Where's your Cue?")
  await shot('server')
  await p.keyboard.type('cue.test')
  await key('Enter')
  await see('Scan the code')
  await see('K7Q2MX')
  await shot('sign-in')
  await see("Who's watching?", 8000)
  await shot('profiles')
  console.log('profiles focus:', await focused())
  await key('Enter')
  await see('Continue watching')
  await shot('home', 1500)
  await key('ArrowDown')
  await key('ArrowRight', 2)
  await shot('home-moved', 900)
  console.log('home focus:', await focused())
  await key('ArrowDown')
  await shot('home-top10', 900)
  // The menu.
  await key('ArrowLeft', 6)
  await shot('menu-open', 500)
  console.log('menu focus:', await focused())
  await key('ArrowDown')
  await shot('movies', 1200)
  await key('ArrowDown')
  await shot('shows', 1200)
  // Search.
  await key('ArrowUp', 3)
  await key('ArrowRight')
  console.log('search focus:', await focused())
  await p.keyboard.type('night')
  await see('Results for')
  await shot('search-results', 900)
  // A show's page from Search.
  await key('ArrowDown')
  console.log('search result focus:', await focused())
  if (!(await focused()).includes('Night Harbor')) throw new Error('Down from search should land on the first result')
  await key('Enter')
  await see('Resume S2:E3')
  await shot('title-show', 1300)
  await key('ArrowDown')
  await shot('title-seasons', 700)
  await key('ArrowDown')
  await shot('title-episodes', 900)
  console.log('episode focus:', await focused())
  // Back to the buttons, then Play.
  await key('ArrowUp', 3)
  console.log('title focus:', await focused())
  await key('Enter')
  await p.waitForFunction(() => {
    const v = document.querySelector('video')
    return v && v.currentTime > 1
  }, null, { timeout: 15000 })
  await shot('player-playing', 300)
  await key('Enter') // pause
  await shot('player-paused', 500)
  await key('Enter')
  await p.waitForFunction(() => { const v = document.querySelector('video'); return v && v.currentTime > 6 }, null, { timeout: 15000 })
  await key('Escape') // Back
  await see('Resume S2:E3')
  await shot('back-from-player', 600)
  console.log('after player focus:', await focused())
  await key('Escape')
  await shot('back-to-search', 700)
  await key('Escape')
  await key('Escape') // menu, then Home
  await key('Escape')
  await key('Escape')
  await shot('leave-toast', 300)

  console.log('requests:', seen.headers.slice(0, 6).join(' | '), '…', seen.headers.length, 'in all')
  console.log('without the app header:', seen.headers.filter((x) => x.includes('app=-')).length)
  console.log('profile header sent:', seen.profileHeader, ' progress saves:', seen.progressSaves)
  console.log('versions asked for:', (seen.options || []).join(', '))
  console.log('errors:', errors.length ? errors.join('\n') : 'none')
  await b.close()
  const wrong = []
  if (errors.length) wrong.push('the page had errors')
  if (seen.headers.some((x) => x.includes('app=-') && !x.includes('/tv-link/qr/'))) wrong.push('a request went without the app header')
  if (!seen.profileHeader) wrong.push('the profile header was never sent')
  if (seen.progressSaves < 1) wrong.push('leaving the player saved no progress')
  if ((seen.options || []).join() !== '1(webos),2(webos)') wrong.push('versions asked for: ' + (seen.options || []).join())
  if (wrong.length) throw new Error(wrong.join('; '))
})().catch((e) => {
  console.error('FAILED:', e.message)
  process.exit(1)
})
