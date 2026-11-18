// Find a line's text work: reading subtitle files and matching words.

export type Cue = { start: number; text: string }

// parseCues reads WebVTT or SRT into lines with their start (seconds).
export function parseCues(src: string): Cue[] {
  const out: Cue[] = []
  const blocks = src.replace(/\r/g, '').split(/\n\n+/)
  const time = (s: string) => {
    const m = /(?:(\d+):)?(\d{1,2}):(\d{2})[.,](\d{1,3})/.exec(s)
    if (!m) return NaN
    return Number(m[1] ?? 0) * 3600 + Number(m[2]) * 60 + Number(m[3]) + Number(m[4].padEnd(3, '0')) / 1000
  }
  for (const b of blocks) {
    const lines = b.split('\n')
    const i = lines.findIndex((l) => l.includes('-->'))
    if (i < 0) continue
    const start = time(lines[i].split('-->')[0])
    const text = lines
      .slice(i + 1)
      .join(' ')
      // Formatting tags (<i>, <font>) and ASS codes ({\an8}) off; then any
      // < or > left (a broken or nested tag) too, so no tag can survive.
      .replace(/<[^>]*>/g, '')
      .replace(/[<>]/g, '')
      .replace(/\{[^}]*\}/g, '')
      .replace(/\s+/g, ' ')
      .trim()
    if (!isNaN(start) && text) out.push({ start, text })
  }
  return out
}

// matchCues finds lines with all the words typed, in order of the film.
export function matchCues(cues: Cue[], q: string): Cue[] {
  const words = q
    .toLowerCase()
    .replace(/[^\p{L}\p{N}' ]/gu, ' ')
    .split(/\s+/)
    .filter(Boolean)
  if (words.length === 0) return []
  const norm = (s: string) => s.toLowerCase().replace(/[^\p{L}\p{N}' ]/gu, ' ')
  return cues.filter((c) => {
    const t = norm(c.text)
    return words.every((w) => t.includes(w))
  })
}

