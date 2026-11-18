import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, type WatchPersonPage } from '../api'
import { useDocumentTitle } from '../documentTitle'
import { Pills, type KindFilter } from './browse'
import { Card, shortDate } from './parts'

// A person's page (from a cast photo): their photo and story, and what else
// they're in, everything playable, most popular first.
export default function PersonPage() {
  const id = Number(useParams().id)
  const [p, setP] = useState<WatchPersonPage | null>(null)
  const [error, setError] = useState('')
  const [kind, setKind] = useState<KindFilter>('all')
  const [more, setMore] = useState(false)
  useDocumentTitle(p?.name ?? 'Cast')
  useEffect(() => {
    let live = true
    api
      .watchPerson(id)
      .then((x) => live && setP(x))
      .catch((e) => live && setError(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
  }, [id])
  if (error) {
    return (
      <div className="wx-page">
        <p className="wx-error">{error}</p>
      </div>
    )
  }
  if (!p) return <div className="wx-page"><div className="wx-person-head wx-skeleton" style={{ height: 220 }} /></div>
  const credits = kind === 'all' ? p.credits : p.credits.filter((c) => c.kind === kind)
  const age = p.born ? ageOf(p.born, p.died) : 0
  const bio = p.biography ?? ''
  return (
    <div className="wx-page wx-person">
      <header className="wx-person-head">
        <div className="wx-person-pic">{p.photo ? <img src={p.photo.replace('/w185/', '/w300/')} alt="" /> : <span>{p.name.slice(0, 1)}</span>}</div>
        <div>
          <h1>{p.name}</h1>
          <p className="wx-dim">
            {[p.knownFor === 'Acting' ? 'Actor' : p.knownFor, p.born && `Born ${shortDate(p.born)}${p.died ? '' : age ? ` (${age})` : ''}`, p.died && `Died ${shortDate(p.died)}`, p.birthplace]
              .filter(Boolean)
              .join(' · ')}
          </p>
          {bio && (
            <p className={`wx-person-bio${more ? ' open' : ''}`}>
              {bio}
              {bio.length > 420 && (
                <button className="wx-linkish" onClick={() => setMore((m) => !m)}>
                  {more ? ' Less' : ' More'}
                </button>
              )}
            </p>
          )}
        </div>
      </header>
      <div className="wx-search-top">
        <h2 style={{ margin: 0 }}>Known for</h2>
        <Pills value={kind} onChange={setKind} />
      </div>
      {credits.length === 0 ? (
        <p className="wx-dim">Nothing here to watch.</p>
      ) : (
        <div className="wx-grid">
          {credits.map((c) => (
            <div key={`${c.kind}-${c.tmdbId}`} className="wx-person-credit">
              <Card card={{ ...c, episodeTitle: undefined }} />
              {c.episodeTitle && <small>{c.episodeTitle}</small>}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function ageOf(born: string, died?: string): number {
  const b = new Date(born)
  const e = died ? new Date(died) : new Date()
  let a = e.getFullYear() - b.getFullYear()
  if (e.getMonth() < b.getMonth() || (e.getMonth() === b.getMonth() && e.getDate() < b.getDate())) a--
  return a > 0 && a < 120 ? a : 0
}
