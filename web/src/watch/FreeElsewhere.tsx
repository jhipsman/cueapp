import { useEffect, useState } from 'react'
import { api, type WatchProvider } from '../api'
import { onTV } from './tv'

// FreeElsewhere shows the free services (Tubi, Pluto TV, The Roku
// Channel...) a title is on, each opening a search for it there. Not on the
// TV: those open in a web browser.
export default function FreeElsewhere({ kind, tmdbId, label = 'Free to watch on' }: { kind: 'movie' | 'tv'; tmdbId: number; label?: string }) {
  const [free, setFree] = useState<WatchProvider[]>([])
  useEffect(() => {
    if (onTV()) return
    let live = true
    setFree([])
    api
      .watchProviders(kind, tmdbId)
      .then((p) => live && setFree(p.free))
      .catch(() => undefined)
    return () => {
      live = false
    }
  }, [kind, tmdbId])
  if (free.length === 0) return null
  return (
    <div className="wx-free">
      <small>{label}</small>
      <div className="wx-free-list">
        {free.map((p) => (
          <a key={p.name} className="wx-provider" href={p.url} target="_blank" rel="noreferrer">
            {p.logo && <img src={p.logo} alt="" />}
            {p.name}
          </a>
        ))}
      </div>
    </div>
  )
}
