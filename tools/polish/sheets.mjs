// Cuts each transition out of the nightly polish recordings and lays its
// frames out on one contact sheet (a grid, left to right, top to bottom), so
// a reviewer can check the motion against docs/motion-rulebook.md frame by
// frame. Needs ffmpeg.
//
//   node tools/polish/sheets.mjs <polish-out-dir>
//
// Reads <dir>/web/marks.json + <dir>/web/<viewport>/walk.webm, and
// <dir>/tv/steps.txt + <dir>/tv/rec/starts.txt + <dir>/tv/rec/part-NN.mp4.
// Writes <dir>/sheets/*.jpg and <dir>/sheets/index.json.
import { execFileSync } from 'node:child_process'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'

const dir = path.resolve(process.argv[2] ?? 'polish-out')
const sheets = path.join(dir, 'sheets')
mkdirSync(sheets, { recursive: true })
const index = []

function sheet(video, start, seconds, fps, cols, width, name, about) {
  const frames = Math.max(1, Math.round(seconds * fps))
  const rows = Math.ceil(frames / cols)
  const file = path.join(sheets, name + '.jpg')
  try {
    execFileSync('ffmpeg', ['-v', 'error', '-y', '-ss', Math.max(0, start).toFixed(2), '-t', seconds.toFixed(2), '-i', video,
      '-vf', `fps=${fps},scale=${width}:-1,tile=${cols}x${rows}:padding=4:color=0x202020`, '-frames:v', '1', '-q:v', '4', file])
    index.push({ sheet: path.basename(file), ...about, frameMs: Math.round(1000 / fps), frames })
  } catch (e) {
    console.error(`${name}: ${e.message}`)
  }
}

const slug = (s) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')

// The web: each step from the move until it settled (at most 2.4 s), at 12 fps.
const marksFile = path.join(dir, 'web', 'marks.json')
if (existsSync(marksFile)) {
  const marks = JSON.parse(readFileSync(marksFile, 'utf8'))
  for (const [viewport, steps] of Object.entries(marks)) {
    const video = path.join(dir, 'web', viewport, 'walk.webm')
    if (!existsSync(video)) continue
    steps.forEach((s, i) => {
      const n = String(i + 1).padStart(2, '0')
      const width = viewport === 'phone' ? 200 : 360
      sheet(video, s.start, Math.min(2.4, s.end - s.start), 12, 6, width, `web-${viewport}-${n}-${slug(s.step)}`,
        { app: 'web', viewport, step: s.step, url: s.url, startSec: s.start })
    })
  }
}

// The TV: each step from the first key press after the step before (the
// move that led to it) until just after it showed, at most 4 s, at 10 fps.
const stepsFile = path.join(dir, 'tv', 'steps.txt')
const startsFile = path.join(dir, 'tv', 'rec', 'starts.txt')
if (existsSync(stepsFile) && existsSync(startsFile)) {
  const parts = readFileSync(startsFile, 'utf8').trim().split('\n').map((l) => {
    const [name, t] = l.trim().split(/\s+/)
    return { name, t: Number(t) }
  }).filter((p) => p.name && Number.isFinite(p.t))
  const events = readFileSync(stepsFile, 'utf8').split('\n').map((l) => {
    const m = l.match(/^\s*([0-9]+\.[0-9]+)\s.*CueWalkThrough:\s*(.*)$/)
    return m ? { t: Number(m[1]), what: m[2].trim() } : null
  }).filter(Boolean)
  let firstPress = null
  for (const e of events) {
    if (e.what.startsWith('press ')) {
      firstPress ??= e.t
      continue
    }
    const m = e.what.match(/^step (\d+) (.*)$/)
    if (!m) continue
    const end = e.t + 0.4
    const start = Math.max(firstPress ?? e.t - 2, end - 4)
    firstPress = null
    const part = [...parts].reverse().find((p) => p.t <= start) ?? parts[0]
    const video = path.join(dir, 'tv', 'rec', part.name + '.mp4')
    if (!part || !existsSync(video)) continue
    sheet(video, start - part.t, end - start, 10, 8, 320, `tv-${m[1]}-${slug(m[2])}`, { app: 'tv', step: m[2], startSec: +(start - part.t).toFixed(2), part: part.name })
  }
}

writeFileSync(path.join(sheets, 'index.json'), JSON.stringify(index, null, 2))
console.log(`${index.length} contact sheets in ${sheets}`)
