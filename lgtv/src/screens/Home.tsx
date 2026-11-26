import { useEffect, useState } from 'react'
import { api, backdrop, type Card, type Home, type Kind, type Row } from '../api'
import { PosterRow, Spinner } from '../ui'

// Home (and Movies and Shows, its rows for one kind): the highlighted
// title's picture behind the screen, its name and story at the top, the
// rows sliding up beneath them, the highlighted row just under the story.

let cached: Home | null = null

export function useHome() {
  const [home, setHome] = useState<Home | null>(cached)
  const [problem, setProblem] = useState('')
  useEffect(() => {
    api
      .home()
      .then((h) => {
        cached = h
        setHome(h)
      })
      .catch((e) => {
        if (!cached) setProblem(e instanceof Error ? e.message : String(e))
      })
  }, [])
  return { home, problem }
}

function rowsFor(h: Home, kind?: Kind): Row[] {
  const seen = new Set<string>()
  const rows: Row[] = []
  for (const r of h.rows) {
    if (seen.has(r.key)) continue
    seen.add(r.key)
    const items = kind ? r.items.filter((c) => c.kind === kind) : r.items
    if (items.length) rows.push({ ...r, items })
  }
  // A Top 10 (the first service's), after what you're watching.
  const top = h.top10?.[0]
  const topItems = top ? (kind === 'movie' ? top.movies : kind === 'tv' ? top.shows : top.items) ?? top.items : []
  if (top && topItems.length) {
    const at = kind ? 0 : Math.max(0, rows.filter((r) => r.key === 'continue' || r.key === 'new-episodes').length)
    rows.splice(at, 0, { key: `top10-${top.key}`, title: `Top 10 on ${top.name}`, items: topItems.slice(0, 10) })
  }
  return rows
}

export function Banner({ c }: { c: Card | null }) {
  const [shown, setShown] = useState(c)
  // The words follow the highlight once it rests, not every poster passed.
  useEffect(() => {
    const t = window.setTimeout(() => setShown(c), 260)
    return () => window.clearTimeout(t)
  }, [c])
  if (!shown) return null
  const facts = [shown.year, shown.kind === 'tv' ? 'Series' : 'Film', shown.rating ? `★ ${shown.rating.toFixed(1)}` : ''].filter(Boolean)
  return (
    <>
      <div className="ambient" key={`bg-${shown.kind}-${shown.tmdbId}`}>
        {shown.backdropUrl && <img src={backdrop(shown.backdropUrl)} alt="" />}
      </div>
      <div className="banner" key={`b-${shown.kind}-${shown.tmdbId}`}>
        {shown.logoUrl ? <img className="banner-logo" src={shown.logoUrl} alt={shown.title} /> : <h1>{shown.title}</h1>}
        <p className="facts">
          {facts.join('  ·  ')}
          {shown.season && shown.episode ? `  ·  S${shown.season}:E${shown.episode}${shown.episodeTitle ? ` ${shown.episodeTitle}` : ''}` : ''}
        </p>
        {shown.overview && <p className="story">{shown.overview}</p>}
      </div>
    </>
  )
}

export function HomeScreen({ kind, onPick }: { kind?: Kind; onPick: (c: Card) => void }) {
  const { home, problem } = useHome()
  const [spot, setSpot] = useState<Card | null>(null)
  if (!home) return <div className="screen center">{problem ? <p className="problem">{problem}</p> : <Spinner label="Loading…" />}</div>
  const rows = rowsFor(home, kind)
  const first = (!kind || home.hero?.kind === kind ? home.hero : undefined) ?? rows[0]?.items.find((c) => c.backdropUrl && !c.progress) ?? null
  return (
    <div className="screen home">
      <Banner c={spot ?? first} />
      <div className="rows" data-vs="" data-vs-at="0">
        {rows.length === 0 && <p className="dim">Nothing here yet.</p>}
        {rows.map((r) => (
          <PosterRow key={r.key} id={`${kind ?? 'home'}-${r.key}`} title={r.title} items={r.items} ranked={r.key.startsWith('top10-')} onPick={onPick} onFocus={setSpot} />
        ))}
        <div className="rows-end" />
      </div>
    </div>
  )
}
