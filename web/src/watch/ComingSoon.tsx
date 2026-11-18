import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api, type WatchCard, type WeekEpisode } from '../api'
import { Pills, ThisWeek, type KindFilter } from './browse'
import Icon from '../components/Icon'
import { shortDate, titleHref } from './parts'

// Coming soon: films not out to stream yet, kept apart from everything you
// can watch now. Each has its day (when TMDB knows it), its picture, what
// it's about, and Remind me: Cue tells you once it's out (at the top of Home,
// and on your phone with alerts on).

const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']

function DateBlock({ day }: { day?: string }) {
  const m = day ? /^\d{4}-(\d{2})-(\d{2})/.exec(day) : null
  return (
    <div className="wx-soon-date" aria-hidden="true">
      {m ? (
        <>
          <span>{MONTHS[Number(m[1]) - 1]}</span>
          <b>{Number(m[2])}</b>
        </>
      ) : (
        <span>SOON</span>
      )}
    </div>
  )
}

export default function ComingSoon() {
  const [items, setItems] = useState<WatchCard[] | null>(null)
  const [error, setError] = useState('')
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const kind = (params.get('show') as KindFilter | null) ?? 'all'
  const [week, setWeek] = useState<WeekEpisode[]>([])
  useEffect(() => {
    api
      .comingSoon()
      .then(setItems)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
    api
      .thisWeek()
      .then((w) => setWeek(w ?? []))
      .catch(() => undefined)
  }, [])
  const films = (items ?? []).filter((c) => kind === 'all' || c.kind === kind)

  const toggle = async (c: WatchCard) => {
    const next = !c.reminding
    setItems((l) => l?.map((x) => (x.kind === c.kind && x.tmdbId === c.tmdbId ? { ...x, reminding: next } : x)) ?? l)
    try {
      await api.setRemind(c.kind, c.tmdbId, next)
    } catch {
      setItems((l) => l?.map((x) => (x.kind === c.kind && x.tmdbId === c.tmdbId ? { ...x, reminding: !next } : x)) ?? l)
    }
  }

  return (
    <div className="wx-page wx-soon">
      <div className="wx-search-top">
        <h1>Coming soon</h1>
        <Pills value={kind} onChange={(k) => setParams(k === 'all' ? {} : { show: k }, { replace: true })} />
      </div>
      <p className="wx-dim">
        {kind === 'tv' ? 'New episodes of the shows you watch, by day.' : 'On their way. Press Remind me and Cue tells you the day you can watch.'}
      </p>
      {error && <p className="wx-error">{error}</p>}
      {kind !== 'movie' && <ThisWeek items={week} />}
      {kind === 'tv' && week.length === 0 && <p className="wx-dim">No new episodes of your shows in the next two weeks.</p>}
      {items === null && !error && kind !== 'tv' && <div className="wx-soon-item wx-skeleton" style={{ height: 260 }} />}
      {items !== null && films.length === 0 && kind !== 'tv' && <p className="wx-dim">Nothing announced right now.</p>}
      {kind !== 'tv' && week.length > 0 && films.length > 0 && <h2 className="wx-soon-sub">Films on their way</h2>}
      {kind !== 'tv' && films.map((c) => (
        <article className="wx-soon-item" key={`${c.kind}-${c.tmdbId}`}>
          <DateBlock day={c.outDate} />
          <div className="wx-soon-body">
            <button className="wx-soon-pic" onClick={() => navigate(titleHref(c))} aria-label={c.title}>
              {c.backdropUrl && <img src={c.backdropUrl} alt="" loading="lazy" />}
            </button>
            <div className="wx-soon-head">
              <h2>{c.title}</h2>
              <div className="wx-soon-actions">
                <button className={`wx-btn small${c.reminding ? '' : ' play'}`} onClick={() => void toggle(c)} aria-pressed={!!c.reminding}>
                  <Icon name={c.reminding ? 'check' : 'clock'} size={16} /> {c.reminding ? 'Reminding you' : 'Remind me'}
                </button>
                <button className="wx-btn small" onClick={() => navigate(titleHref(c))}>
                  <Icon name="info" size={16} /> Info
                </button>
              </div>
            </div>
            <p className="wx-soon-when">{c.outDate ? `Streaming ${shortDate(c.outDate)}` : 'Streaming date not announced yet'}</p>
            {c.overview && <p className="wx-soon-about">{c.overview}</p>}
          </div>
        </article>
      ))}
    </div>
  )
}
