import { useEffect, useState } from 'react'
import { api, type WatchProvider } from '../api'
import { onTV } from './tv'

// FreeElsewhere shows the free services (Tubi, Pluto TV, The Roku
// Channel...) a title is on, each opening a search for it there. Not on the
// TV: those open in a web browser.
export default function FreeElsewhere({
  kind,
  tmdbId,
  label = 'Free to watch on',
  subscriptions,
}: {
  kind: 'movie' | 'tv'
  tmdbId: number
  label?: string
  subscriptions?: boolean // also the services you pay for (Netflix, Disney+...)
}) {
  const [free, setFree] = useState<WatchProvider[]>([])
  const [paid, setPaid] = useState<WatchProvider[]>([])
  useEffect(() => {
    if (onTV()) return
    let live = true
    setFree([])
    setPaid([])
    api
      .watchProviders(kind, tmdbId)
      .then((p) => {
        if (!live) return
        setFree(p.free)
        setPaid(p.subscription)
      })
      .catch(() => undefined)
    return () => {
      live = false
    }
  }, [kind, tmdbId])
  const groups = [
    { label, list: free },
    { label: 'Streaming on', list: subscriptions ? paid.slice(0, 6) : [] },
  ].filter((g) => g.list.length > 0)
  if (groups.length === 0) return null
  return (
    <>
      {groups.map((g) => (
        <div className="wx-free" key={g.label}>
          <small>{g.label}</small>
          <div className="wx-free-list">
            {g.list.map((p) => (
              <a key={p.name} className="wx-provider" href={p.url} target="_blank" rel="noreferrer">
                {p.logo && <img src={p.logo} alt="" />}
                {p.name}
              </a>
            ))}
          </div>
        </div>
      ))}
    </>
  )
}
