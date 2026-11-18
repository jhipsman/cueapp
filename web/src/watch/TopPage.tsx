import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, type WatchCard } from '../api'
import { useDocumentTitle } from '../documentTitle'
import { Pills, type KindFilter } from './browse'
import { Card } from './parts'

type Top = Awaited<ReturnType<typeof api.watchTop>>

const DECADES = [2020, 2010, 2000, 1990, 1980, 1970, 1960, 1950]

// The Top 10 page: what's most watched on any service, in any country, of a
// genre or decade, with the big numbers. Everything in it plays on Cue.
export default function TopPage() {
  const [params, setParams] = useSearchParams()
  const kind = (params.get('kind') as KindFilter | null) ?? 'all'
  const service = params.get('service') ?? ''
  const country = params.get('country') ?? 'US'
  const genre = params.get('genre') ?? ''
  const decade = Number(params.get('decade') ?? 0)
  const [top, setTop] = useState<{ key: string; data: Top } | null>(null)
  const [error, setError] = useState('')
  const key = params.toString()
  useDocumentTitle(top?.data.title ?? 'Top 10')
  useEffect(() => {
    let live = true
    api
      .watchTop({ kind, service, country, genre, decade })
      .then((d) => live && setTop({ key, data: d }))
      .catch((e) => live && setError(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])
  const set = (k: string, v: string) => {
    const next = new URLSearchParams(params)
    if (!v || (k === 'kind' && v === 'all') || (k === 'country' && v === 'US')) next.delete(k)
    else next.set(k, v)
    setParams(next, { replace: true })
  }
  const data = top?.data
  const items: WatchCard[] | null = top && top.key === key ? top.data.items : null
  return (
    <div className="wx-page wx-toppage">
      <h1>{data?.title ?? 'Top 10'}</h1>
      <div className="wx-top-filters">
        <Pills value={kind} onChange={(k) => set('kind', k)} />
        <div className="wx-top-tabs" role="tablist" aria-label="Service">
          <button className={`wx-top-tab${service ? '' : ' on'}`} onClick={() => set('service', '')}>
            Everywhere
          </button>
          {data?.services.map((s) => (
            <button key={s.key} className={`wx-top-tab${service === s.key ? ' on' : ''}`} onClick={() => set('service', s.key)}>
              {s.name}
            </button>
          ))}
        </div>
        <div className="wx-top-selects">
          <select value={genre} onChange={(e) => set('genre', e.target.value)} aria-label="Genre">
            <option value="">Any genre</option>
            {data?.genres.map((g) => (
              <option key={g}>{g}</option>
            ))}
          </select>
          <select value={decade || ''} onChange={(e) => set('decade', e.target.value)} aria-label="Decade">
            <option value="">Right now</option>
            {DECADES.map((d) => (
              <option key={d} value={d}>
                The {d}s
              </option>
            ))}
          </select>
          <select value={country} onChange={(e) => set('country', e.target.value)} aria-label="Country">
            {Object.entries(data?.countries ?? { US: 'United States' })
              .sort((a, b) => a[1].localeCompare(b[1]))
              .map(([code, name]) => (
                <option key={code} value={code}>
                  {name}
                </option>
              ))}
          </select>
        </div>
      </div>
      {error && <p className="wx-error">{error}</p>}
      {items === null && !error && <div className="wx-top-grid">{Array.from({ length: 10 }, (_, i) => <div key={i} className="wx-top-cell wx-skeleton" />)}</div>}
      {items?.length === 0 && <p className="wx-dim">Nothing found for that. Try another service or genre.</p>}
      {items && items.length > 0 && (
        <div className="wx-top-grid">
          {items.map((c, i) => (
            <div className="wx-top-item" key={`${c.kind}-${c.tmdbId}`}>
              <span className="wx-top-n" aria-hidden="true">
                {i + 1}
              </span>
              <Card card={c} />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
