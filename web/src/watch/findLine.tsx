import { useEffect, useMemo, useRef, useState } from 'react'
import { matchCues, parseCues, type Cue } from './findLineText'

// Find a line: type words someone says and jump to that moment. It reads
// the title's subtitles (fetching them if they aren't on).

function clock(sec: number): string {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}` : `${m}:${String(s).padStart(2, '0')}`
}

export function FindLine({ subtitleUrl, onJump, onClose }: { subtitleUrl: () => Promise<string>; onJump: (sec: number) => void; onClose: () => void }) {
  const [cues, setCues] = useState<Cue[] | null>(null)
  const [error, setError] = useState('')
  const [q, setQ] = useState('')
  const box = useRef<HTMLInputElement>(null)
  useEffect(() => {
    let live = true
    box.current?.focus()
    subtitleUrl()
      .then((u) => fetch(u, { credentials: 'same-origin' }))
      .then((r) => {
        if (!r.ok) throw new Error('Couldn’t read the subtitles.')
        return r.text()
      })
      .then((t) => live && setCues(parseCues(t)))
      .catch((e) => live && setError(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])
  const hits = useMemo(() => (cues ? matchCues(cues, q).slice(0, 30) : []), [cues, q])
  return (
    <div className="wx-findline glass" role="dialog" aria-label="Find a line">
      <div className="wx-findline-top">
        <input ref={box} value={q} onChange={(e) => setQ(e.target.value)} placeholder="Type words someone says…" aria-label="Find a line" />
        <button className="wx-news-x" onClick={onClose} aria-label="Close">
          ×
        </button>
      </div>
      {error && <p className="wx-error">{error}</p>}
      {!cues && !error && <p className="wx-dim">Reading the subtitles…</p>}
      {cues && q.trim() && hits.length === 0 && <p className="wx-dim">No line with those words.</p>}
      <div className="wx-findline-hits">
        {hits.map((h) => (
          <button
            key={h.start}
            onClick={() => {
              onJump(Math.max(0, h.start - 1))
              onClose()
            }}
          >
            <span>{clock(h.start)}</span>
            {h.text}
          </button>
        ))}
      </div>
    </div>
  )
}
