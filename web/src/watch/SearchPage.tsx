import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api, type LiveHit, type WatchCard } from '../api'
import { useDocumentTitle } from '../documentTitle'
import ChannelLogo from './ChannelLogo'
import { liveHref } from './LivePage'
import { Card } from './parts'
import { useProfiles } from './profiles'

const hhmm = (s: string) => new Date(s).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })

// when says when a show is on: "Ends 8:45 PM" while it's on, "7:30 PM"
// today, "Tomorrow 1:00 PM", or the weekday.
function when(h: LiveHit): string {
  if (!h.show) return ''
  if (h.onNow) return `Ends ${hhmm(h.show.stop)}`
  const start = new Date(h.show.start)
  const today = new Date()
  const days = Math.round((new Date(start).setHours(0, 0, 0, 0) - new Date(today).setHours(0, 0, 0, 0)) / 86_400_000)
  const day = days === 0 ? 'Today' : days === 1 ? 'Tomorrow' : start.toLocaleDateString([], { weekday: 'long' })
  return `${day} ${hhmm(h.show.start)}`
}

// Search: every movie and show TMDB knows, best match first, and with Live
// TV set up, games and shows on now and coming up, and channels by name.
// The search box is in the top bar.
export default function SearchPage() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const liveTV = useProfiles().data?.liveTV ?? false
  const q = params.get('q') ?? ''
  const [results, setResults] = useState<{ q: string; items: WatchCard[] } | null>(null)
  const [live, setLive] = useState<{ q: string; hits: LiveHit[] } | null>(null)
  const [error, setError] = useState('')
  useDocumentTitle(q ? `Search: ${q}` : 'Search')

  useEffect(() => {
    if (!q.trim()) return
    let current = true
    api
      .watchSearch(q)
      .then((items) => {
        if (current) {
          setResults({ q, items })
          setError('')
        }
      })
      .catch((e) => current && setError(e instanceof Error ? e.message : String(e)))
    if (liveTV) {
      api
        .liveSearch(q)
        .then((r) => current && setLive({ q, hits: r.hits }))
        .catch(() => current && setLive({ q, hits: [] }))
    }
    return () => {
      current = false
    }
  }, [q, liveTV])

  const items = results?.items ?? []
  const hits = live?.q === q ? live.hits : []
  const shows = hits.filter((h) => h.show)
  const onNow = shows.filter((h) => h.onNow)
  const later = shows.filter((h) => !h.onNow)
  const channels = hits.filter((h) => !h.show)
  const done = results?.q === q && (!liveTV || live?.q === q)

  const LiveCard = ({ h }: { h: LiveHit }) => (
    <button className={`wx-live-hit${h.onNow ? ' now' : ''}`} onClick={() => navigate(liveHref(h.channel.id, 'all'))}>
      <span className="wx-live-hit-logo">
        <ChannelLogo name={h.channel.name} src={h.channel.logo} />
      </span>
      <span className="wx-live-hit-body">
        {h.onNow && (
          <span className="wx-live-hit-badge">
            <i /> LIVE
          </span>
        )}
        <b>{h.show?.title ?? h.channel.name}</b>
        <small>
          {h.show ? `${h.channel.name} · ${when(h)}` : h.channel.now ? `Now: ${h.channel.now.title}` : `Channel ${h.channel.num || ''}`}
        </small>
      </span>
    </button>
  )

  return (
    <div className="wx-page">
      {error && <p className="wx-error">{error}</p>}
      {done && items.length === 0 && hits.length === 0 && <p className="wx-dim">Nothing found for “{q}”.</p>}

      {onNow.length > 0 && (
        <section className="wx-search-live">
          <h2>On now</h2>
          <div className="wx-live-hits">
            {onNow.map((h) => (
              <LiveCard key={`${h.channel.id}-${h.show?.start}`} h={h} />
            ))}
          </div>
        </section>
      )}

      {items.length > 0 && (
        <section>
          {hits.length > 0 && <h2>Movies and shows</h2>}
          <div className="wx-grid">
            {items.map((c) => (
              <Card key={`${c.kind}-${c.tmdbId}`} card={c} />
            ))}
          </div>
        </section>
      )}

      {later.length > 0 && (
        <section className="wx-search-live">
          <h2>Coming up on Live TV</h2>
          <div className="wx-live-hits">
            {later.map((h) => (
              <LiveCard key={`${h.channel.id}-${h.show?.start}`} h={h} />
            ))}
          </div>
        </section>
      )}

      {channels.length > 0 && (
        <section className="wx-search-live">
          <h2>Channels</h2>
          <div className="wx-live-hits">
            {channels.map((h) => (
              <LiveCard key={h.channel.id} h={h} />
            ))}
          </div>
        </section>
      )}
    </div>
  )
}
